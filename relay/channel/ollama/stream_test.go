package ollama

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOllamaChatHandlerReturnsNonStreamToolCalls(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "compact response",
			raw:  `{"model":"llama3.1","created_at":"2026-05-27T12:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Paris","days":0}}}]},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":7}`,
		},
		{
			name: "pretty response fallback",
			raw: `{
  "model": "llama3.1",
  "message": {
    "role": "assistant",
    "tool_calls": [{"function": {"name": "get_weather", "arguments": {"city": "Paris", "days": 0}}}]
  },
  "done": true,
  "prompt_eval_count": 5,
  "eval_count": 7
}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.raw))}
			usage, relayErr := ollamaChatHandler(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "fallback"}}, resp)
			require.Nil(t, relayErr)
			require.Equal(t, 12, usage.TotalTokens)

			var output dto.OpenAITextResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &output))
			require.Equal(t, constant.FinishReasonToolCalls, output.Choices[0].FinishReason)
			var toolCalls []dto.ToolCallResponse
			require.NoError(t, common.Unmarshal(output.Choices[0].Message.ToolCalls, &toolCalls))
			require.Len(t, toolCalls, 1)
			require.Nil(t, toolCalls[0].Index)
			require.Equal(t, "get_weather", toolCalls[0].Function.Name)
			var arguments map[string]any
			require.NoError(t, common.Unmarshal([]byte(toolCalls[0].Function.Arguments), &arguments))
			require.Equal(t, "Paris", arguments["city"])
			require.Equal(t, float64(0), arguments["days"])
		})
	}
}

func TestOllamaToolCallsToOpenAIDefaultsNilArguments(t *testing.T) {
	toolCall := OllamaToolCall{}
	toolCall.Function.Name = "ping"
	converted, nextIndex := ollamaToolCallsToOpenAI([]OllamaToolCall{toolCall}, 2, true)
	require.Equal(t, 3, nextIndex)
	require.Len(t, converted, 1)
	require.Equal(t, "call_2", converted[0].ID)
	require.NotNil(t, converted[0].Index)
	require.Equal(t, 2, *converted[0].Index)
	require.Equal(t, "{}", converted[0].Function.Arguments)
}

func TestOllamaStreamHandlerUsesToolCallsFinishReason(t *testing.T) {
	body := strings.Join([]string{
		`{"model":"llama3.1","message":{"role":"assistant","tool_calls":[{"function":{"name":"ping","arguments":{}}}]},"done":false}`,
		`{"model":"llama3.1","done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":1}`,
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "llama3.1"}}
	usage, relayErr := ollamaStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 2, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"finish_reason":"tool_calls"`)
}
