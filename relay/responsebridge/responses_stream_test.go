package responsebridge

import (
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

func TestResponsesStreamEmitterProducesTerminalEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set(common.RequestIdKey, "bridge-test")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, OriginModelName: "test-model"}

	textChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-bridge",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}}},
	}
	textChunk.Choices[0].Delta.SetContentString("hello")
	require.NoError(t, EmitChatChunk(c, info, textChunk))

	index := 0
	toolChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-bridge",
		Created: 123,
		Model:   "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{{
				Index: &index,
				ID:    "call_1",
				Type:  "function",
				Function: dto.FunctionResponse{
					Name:      "lookup",
					Arguments: `{"id":`,
				},
			}}},
		}},
	}
	require.NoError(t, EmitChatChunk(c, info, toolChunk))
	toolChunk.Choices[0].Delta.ToolCalls[0].ID = ""
	toolChunk.Choices[0].Delta.ToolCalls[0].Function.Name = ""
	toolChunk.Choices[0].Delta.ToolCalls[0].Function.Arguments = "1}"
	require.NoError(t, EmitChatChunk(c, info, toolChunk))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}, "tool_calls"))

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, "event: response.function_call_arguments.delta")
	require.Contains(t, body, "event: response.completed")
	require.Contains(t, body, `"arguments":"{\"id\":1}"`)
	require.False(t, strings.Contains(body, "chat.completion.chunk"))
}
