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
