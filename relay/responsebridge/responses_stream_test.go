package responsebridge

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
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

func TestResponsesStreamEmitterRestoresNamespaceInLifecycleAndTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set(common.RequestIdKey, "namespace-bridge-test")
	openaicompat.SetResponsesBridgeToolMapping(c, openaicompat.ResponsesBridgeToolMapping{NamespaceTools: map[string]openaicompat.ResponsesNamespaceToolName{
		"team__send": {Namespace: "team", Name: "send"},
	}})
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, OriginModelName: "test-model"}
	index := 0
	chunk := &dto.ChatCompletionsStreamResponse{Id: "chatcmpl-namespace", Model: "test-model", Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{{
		Index: &index, ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "team__send", Arguments: `{}`},
	}}}}}}
	require.NoError(t, EmitChatChunk(c, info, chunk))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{TotalTokens: 1}, "tool_calls"))

	body := recorder.Body.String()
	require.Equal(t, 3, strings.Count(body, `"namespace":"team"`), "added, done, and terminal output must all restore namespace")
	require.NotContains(t, body, `"name":"team__send"`)
}

func TestResponsesStreamEmitterMapsLengthToIncompleteReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set(common.RequestIdKey, "incomplete-bridge-test")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, OriginModelName: "test-model"}

	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-incomplete",
		Created: 123,
		Model:   "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}}},
	}
	chunk.Choices[0].Delta.SetContentString("partial")
	require.NoError(t, EmitChatChunk(c, info, chunk))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}, "length"))

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.incomplete")
	require.Contains(t, body, `"status":"incomplete"`)
	require.Contains(t, body, `"incomplete_details":{"reason":"max_output_tokens"}`)
	require.NotContains(t, body, `"reasoning"`)
}
