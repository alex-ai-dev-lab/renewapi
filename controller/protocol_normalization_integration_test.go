package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type bridgeProtocol struct {
	endpoint, path string
	format         types.RelayFormat
}

var bridgeProtocols = []bridgeProtocol{
	{"openai", "/v1/chat/completions", types.RelayFormatOpenAI},
	{"openai-response", "/v1/responses", types.RelayFormatOpenAIResponses},
	{"anthropic", "/v1/messages", types.RelayFormatClaude},
}

func TestModelDefaultProtocolMatrixTwoTurnTools(t *testing.T) {
	runDefaultProtocolMatrix(t, false)
}

func TestModelDefaultProtocolMatrixReasoningAndImages(t *testing.T) {
	runDefaultProtocolMatrix(t, true)
}

func runDefaultProtocolMatrix(t *testing.T, rich bool) {
	for _, client := range bridgeProtocols {
		for _, upstream := range bridgeProtocols {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s_to_%s_stream_%t", client.endpoint, upstream.endpoint, stream), func(t *testing.T) {
					db := setupEmptyStreamRecoveryDB(t)
					seedRecoveryPrincipal(t, db)
					old := operation_setting.ModelEndpointDefaults2JsonString()
					t.Cleanup(func() { require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(old)) })
					profile := fmt.Sprintf(`{"enabled":true,"normalize_text_endpoints":true,"entries":[{"match_type":"exact","pattern":"%s","channel_type":1,"default_endpoint":"%s","supported_endpoints":["openai","openai-response","anthropic"]}]}`, failoverTestModel, upstream.endpoint)
					require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(profile))
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
							return
						}
						if r.URL.Path != upstream.path {
							t.Errorf("wire endpoint %s, expected %s", r.URL.Path, upstream.path)
							w.WriteHeader(404)
							return
						}
						turn := int(calls.Add(1))
						if turn == 2 {
							for _, s := range []string{"call_a", "call_b", "result-a", "result-b", "historical-visible-note"} {
								if !bytes.Contains(body, []byte(s)) {
									t.Errorf("second turn lost %q: %s", s, body)
								}
							}
						}
						var request map[string]any
						if err := common.Unmarshal(body, &request); err != nil {
							t.Error(err)
							return
						}
						if upstream.endpoint == "openai-response" {
							require.NotNil(t, request["input"])
						} else {
							require.NotNil(t, request["messages"])
						}
						if upstream.endpoint == "anthropic" {
							choice, _ := request["tool_choice"].(map[string]any)
							require.Equal(t, true, choice["disable_parallel_tool_use"])
						} else {
							require.Equal(t, false, request["parallel_tool_calls"])
						}
						if rich {
							require.Contains(t, string(body), bridgePixel)
							if upstream.endpoint == "anthropic" {
								require.NotNil(t, request["thinking"])
							} else {
								require.True(t, request["reasoning"] != nil || request["reasoning_effort"] != nil)
							}
						}
						writeBridgeFixture(t, w, upstream.endpoint, stream, turn)
					}))
					t.Cleanup(server.Close)
					channel := recoveryIntegrationChannel(9851, "protocol-matrix", server.URL, 100, failoverTestModel)
					require.NoError(t, db.Create(channel).Error)
					require.NoError(t, channel.AddAbilities(nil))
					model.InitChannelCache()
					for turn := 1; turn <= 2; turn++ {
						router, _, _ := recoveryIntegrationRouter(fmt.Sprintf("matrix-%s-%s-%t-%d", client.endpoint, upstream.endpoint, stream, turn))
						if client.path != "/v1/responses" {
							router.POST(client.path, middleware.Distribute(), func(c *gin.Context) { Relay(c, client.format) })
						}
						payload := bridgeClientRequest(client.endpoint, stream, turn)
						if rich {
							enrichBridgeClientRequest(payload, client.endpoint)
						}
						body, err := common.Marshal(payload)
						require.NoError(t, err)
						req := httptest.NewRequest(http.MethodPost, client.path, bytes.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						w := httptest.NewRecorder()
						router.ServeHTTP(w, req)
						require.Equal(t, 200, w.Code, w.Body.String())
						if turn == 1 {
							require.Contains(t, w.Body.String(), "call_a")
							require.Contains(t, w.Body.String(), "call_b")
							require.Contains(t, w.Body.String(), "lookup")
						} else {
							require.Contains(t, w.Body.String(), "All tools completed")
						}
						if stream {
							switch client.endpoint {
							case "anthropic":
								require.Contains(t, w.Body.String(), "message_stop")
							case "openai-response":
								require.Contains(t, w.Body.String(), "response.completed")
							default:
								require.Contains(t, w.Body.String(), "[DONE]")
							}
						} else {
							var response map[string]any
							require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
							switch client.endpoint {
							case "anthropic":
								require.Equal(t, "message", response["type"])
							case "openai-response":
								require.Equal(t, "response", response["object"])
							default:
								require.NotNil(t, response["choices"])
							}
						}
					}
					require.EqualValues(t, 2, calls.Load())
					var user model.User
					require.NoError(t, db.First(&user, 1).Error)
					require.Equal(t, 2, user.RequestCount)
					perfmetrics.WaitForPendingSamples()
				})
			}
		}
	}
}

func bridgeClientRequest(protocol string, stream bool, turn int) map[string]any {
	prompt := "Analyze this synthetic report. " + strings.Repeat("Each task must retain its identifier and be finalized only once. ", 100)
	schema := map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}, "required": []string{"q"}}
	body := map[string]any{"model": failoverTestModel, "stream": stream}
	switch protocol {
	case "openai-response":
		body["max_output_tokens"] = 2200
		body["tools"] = []any{map[string]any{"type": "function", "name": "lookup", "parameters": schema}}
		body["parallel_tool_calls"] = false
		input := []any{map[string]any{"role": "user", "content": prompt}}
		if turn == 2 {
			input = append(input, map[string]any{"role": "assistant", "content": "historical-visible-note"})
			for _, id := range []string{"a", "b"} {
				input = append(input, map[string]any{"type": "function_call", "call_id": "call_" + id, "name": "lookup", "arguments": `{"q":"` + id + `"}`})
			}
			for _, id := range []string{"a", "b"} {
				input = append(input, map[string]any{"type": "function_call_output", "call_id": "call_" + id, "output": "result-" + id})
			}
		}
		body["input"] = input
	case "anthropic":
		body["max_tokens"] = 2200
		body["tools"] = []any{map[string]any{"name": "lookup", "input_schema": schema}}
		body["tool_choice"] = map[string]any{"type": "auto", "disable_parallel_tool_use": true}
		messages := []any{map[string]any{"role": "user", "content": prompt}}
		if turn == 2 {
			content := []any{map[string]any{"type": "text", "text": "historical-visible-note"}}
			results := []any{}
			for _, id := range []string{"a", "b"} {
				content = append(content, map[string]any{"type": "tool_use", "id": "call_" + id, "name": "lookup", "input": map[string]any{"q": id}})
				results = append(results, map[string]any{"type": "tool_result", "tool_use_id": "call_" + id, "content": "result-" + id})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": content}, map[string]any{"role": "user", "content": results})
		}
		body["messages"] = messages
	default:
		body["max_tokens"] = 2200
		body["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": "lookup", "parameters": schema}}}
		body["parallel_tool_calls"] = false
		messages := []any{map[string]any{"role": "user", "content": prompt}}
		if turn == 2 {
			calls := []any{}
			for _, id := range []string{"a", "b"} {
				calls = append(calls, map[string]any{"type": "function", "id": "call_" + id, "function": map[string]any{"name": "lookup", "arguments": `{"q":"` + id + `"}`}})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": "historical-visible-note", "tool_calls": calls})
			for _, id := range []string{"a", "b"} {
				messages = append(messages, map[string]any{"role": "tool", "tool_call_id": "call_" + id, "content": "result-" + id})
			}
		}
		body["messages"] = messages
	}
	return body
}

func writeBridgeFixture(t *testing.T, w http.ResponseWriter, protocol string, stream bool, turn int) {
	t.Helper()
	emit := func(v any) {
		b, err := common.Marshal(v)
		if err != nil {
			t.Error(err)
			return
		}
		if stream {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		} else {
			_, _ = w.Write(b)
		}
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	text := "All tools completed."
	switch protocol {
	case "openai-response":
		output := []any{}
		if turn == 1 {
			for _, id := range []string{"a", "b"} {
				output = append(output, map[string]any{"type": "function_call", "id": "fc_" + id, "call_id": "call_" + id, "name": "lookup", "arguments": `{"q":"` + id + `"}`})
			}
		} else {
			output = append(output, map[string]any{"type": "message", "id": "msg_1", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}})
		}
		response := map[string]any{"id": "resp_matrix", "object": "response", "model": failoverTestModel, "status": "completed", "output": output, "usage": map[string]any{"input_tokens": 1500, "output_tokens": 20, "total_tokens": 1520}}
		if !stream {
			emit(response)
			return
		}
		emit(map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_matrix", "model": failoverTestModel}})
		for i, item := range output {
			m := item.(map[string]any)
			emit(map[string]any{"type": "response.output_item.added", "output_index": i, "item": m})
			if turn == 1 {
				emit(map[string]any{"type": "response.function_call_arguments.delta", "item_id": m["id"], "output_index": i, "delta": m["arguments"]})
			} else {
				emit(map[string]any{"type": "response.output_text.delta", "item_id": m["id"], "output_index": i, "delta": text})
			}
			emit(map[string]any{"type": "response.output_item.done", "output_index": i, "item": m})
		}
		emit(map[string]any{"type": "response.completed", "response": response})
	case "anthropic":
		content := []any{}
		stop := "end_turn"
		if turn == 1 {
			stop = "tool_use"
			for _, id := range []string{"a", "b"} {
				content = append(content, map[string]any{"type": "tool_use", "id": "call_" + id, "name": "lookup", "input": map[string]any{"q": id}})
			}
		} else {
			content = append(content, map[string]any{"type": "text", "text": text})
		}
		response := map[string]any{"id": "msg_matrix", "type": "message", "role": "assistant", "model": failoverTestModel, "content": content, "stop_reason": stop, "usage": map[string]any{"input_tokens": 1500, "output_tokens": 20}}
		if !stream {
			emit(response)
			return
		}
		emit(map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_matrix", "type": "message", "role": "assistant", "model": failoverTestModel, "content": []any{}, "usage": map[string]any{"input_tokens": 1500, "output_tokens": 0}}})
		for i, item := range content {
			m := item.(map[string]any)
			var delta any
			if turn == 1 {
				arg, _ := common.Marshal(m["input"])
				m["input"] = map[string]any{}
				delta = map[string]any{"type": "input_json_delta", "partial_json": string(arg)}
			} else {
				m["text"] = ""
				delta = map[string]any{"type": "text_delta", "text": text}
			}
			emit(map[string]any{"type": "content_block_start", "index": i, "content_block": m})
			emit(map[string]any{"type": "content_block_delta", "index": i, "delta": delta})
			emit(map[string]any{"type": "content_block_stop", "index": i})
		}
		emit(map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop}, "usage": map[string]any{"output_tokens": 20}})
		emit(map[string]any{"type": "message_stop"})
	default:
		message := map[string]any{"role": "assistant", "content": text}
		stop := "stop"
		if turn == 1 {
			message["content"] = nil
			stop = "tool_calls"
			calls := []any{}
			for i, id := range []string{"a", "b"} {
				calls = append(calls, map[string]any{"index": i, "id": "call_" + id, "type": "function", "function": map[string]any{"name": "lookup", "arguments": `{"q":"` + id + `"}`}})
			}
			message["tool_calls"] = calls
		}
		usage := map[string]any{"prompt_tokens": 1500, "completion_tokens": 20, "total_tokens": 1520}
		if !stream {
			emit(map[string]any{"id": "chatcmpl_matrix", "object": "chat.completion", "model": failoverTestModel, "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": stop}}, "usage": usage})
			return
		}
		emit(map[string]any{"id": "chatcmpl_matrix", "object": "chat.completion.chunk", "model": failoverTestModel, "choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": nil}}})
		emit(map[string]any{"id": "chatcmpl_matrix", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": stop}}, "usage": usage})
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}
}

const bridgePixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a6n0AAAAASUVORK5CYII="

func TestNormalizedBridgeFailoverResetsStreamState(t *testing.T) {
	for caseIndex, pair := range [][2]int{{1, 2}, {2, 0}, {2, 1}} {
		client, upstream := bridgeProtocols[pair[0]], bridgeProtocols[pair[1]]
		t.Run(client.endpoint+"_to_"+upstream.endpoint, func(t *testing.T) {
			db := setupEmptyStreamRecoveryDB(t)
			seedRecoveryPrincipal(t, db)
			old := operation_setting.ModelEndpointDefaults2JsonString()
			t.Cleanup(func() { require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(old)) })
			require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(fmt.Sprintf(`{"enabled":true,"normalize_text_endpoints":true,"entries":[{"match_type":"exact","pattern":"%s","channel_type":1,"default_endpoint":"%s"}]}`, failoverTestModel, upstream.endpoint)))
			var badCalls, goodCalls atomic.Int32
			bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				badCalls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				switch upstream.endpoint {
				case "anthropic":
					_, _ = io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"failed_start\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\n")
				case "openai-response":
					_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"failed_start\"}}\n\n")
				default:
					_, _ = io.WriteString(w, "data: {\"id\":\"failed_start\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n")
				}
			}))
			t.Cleanup(bad.Close)
			good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				goodCalls.Add(1)
				writeBridgeFixture(t, w, upstream.endpoint, true, 1)
			}))
			t.Cleanup(good.Close)
			for i, server := range []*httptest.Server{bad, good} {
				channel := recoveryIntegrationChannel(9860+caseIndex*10+i, "bridge-failover", server.URL, int64(100-i), failoverTestModel)
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			model.InitChannelCache()
			router, _, _ := recoveryIntegrationRouter("bridge-failover-" + client.endpoint + "-" + upstream.endpoint)
			if client.path != "/v1/responses" {
				router.POST(client.path, middleware.Distribute(), func(c *gin.Context) { Relay(c, client.format) })
			}
			body, err := common.Marshal(bridgeClientRequest(client.endpoint, true, 1))
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, client.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "call_a")
			require.Contains(t, w.Body.String(), "call_b")
			require.NotContains(t, w.Body.String(), "failed_start")
			require.EqualValues(t, 1, badCalls.Load())
			require.EqualValues(t, 1, goodCalls.Load())
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			require.Equal(t, 1, user.RequestCount)
			perfmetrics.WaitForPendingSamples()
		})
	}
}

func enrichBridgeClientRequest(body map[string]any, protocol string) {
	url := "data:image/png;base64," + bridgePixel
	switch protocol {
	case "openai-response":
		body["reasoning"] = map[string]any{"effort": "low"}
		first := body["input"].([]any)[0].(map[string]any)
		first["content"] = []any{map[string]any{"type": "input_text", "text": first["content"]}, map[string]any{"type": "input_image", "image_url": url}}
	case "anthropic":
		body["thinking"] = map[string]any{"type": "adaptive"}
		first := body["messages"].([]any)[0].(map[string]any)
		first["content"] = []any{map[string]any{"type": "text", "text": first["content"]}, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": bridgePixel}}}
	default:
		body["reasoning_effort"] = "low"
		first := body["messages"].([]any)[0].(map[string]any)
		first["content"] = []any{map[string]any{"type": "text", "text": first["content"]}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}}}
	}
}
