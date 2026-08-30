package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesToChatStreamDoesNotDuplicateTerminalToolCall(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}`,
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc_1","delta":"{\"q\":\"x\"}"}`,
		`data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "duplicate-tool-test")
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}

	usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 2, usage.TotalTokens)

	var arguments strings.Builder
	indexes := map[int]bool{}
	for _, frame := range strings.Split(recorder.Body.String(), "\n\n") {
		line := strings.TrimPrefix(frame, "data: ")
		if line == frame || line == "[DONE]" {
			continue
		}
		var chunk dto.ChatCompletionsStreamResponse
		if common.UnmarshalJsonStr(line, &chunk) != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			for _, call := range choice.Delta.ToolCalls {
				require.NotNil(t, call.Index)
				indexes[*call.Index] = true
				arguments.WriteString(call.Function.Arguments)
			}
		}
	}
	require.Equal(t, map[int]bool{0: true}, indexes)
	require.Equal(t, `{"q":"x"}`, arguments.String())
}

func TestOaiResponsesToChatStreamRejectsMissingTerminalAfterPartialOutput(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"model":"gpt-test","created_at":1}}`,
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		``,
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "missing-terminal-test")
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}

	usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Contains(t, relayErr.Error(), "responses stream missing terminal event")
	require.Contains(t, recorder.Body.String(), "partial")
	require.NotContains(t, recorder.Body.String(), `"finish_reason":"stop"`)
	require.NotContains(t, recorder.Body.String(), "data: [DONE]")
}

func TestOaiResponsesToChatStreamAcceptsIncompleteTerminal(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		`data: {"type":"response.incomplete","response":{"status":"incomplete","model":"gpt-test","output":[],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "incomplete-terminal-test")
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}

	usage, relayErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "partial")
	require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
	require.NotContains(t, recorder.Body.String(), `"finish_reason":"stop"`)
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}
