package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatResponseToResponsesPreservesTextToolsAndUsage(t *testing.T) {
	message := dto.Message{Role: "assistant"}
	message.SetStringContent("done")
	message.SetToolCalls([]dto.ToolCallResponse{{
		ID:   "call_1",
		Type: "function",
		Function: dto.FunctionResponse{
			Name:      "lookup",
			Arguments: `{"id":1}`,
		},
	}})
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl-test",
		Created: 123,
		Model:   "test-model",
		Choices: []dto.OpenAITextResponseChoice{{Message: message, FinishReason: "tool_calls"}},
		Usage:   dto.Usage{PromptTokens: 5, CompletionTokens: 7, TotalTokens: 12},
	}

	responses, err := ChatResponseToResponses(chat)
	require.NoError(t, err)
	require.Equal(t, "resp_test", responses.ID)
	require.Equal(t, "response", responses.Object)
	require.Len(t, responses.Output, 2)
	require.Equal(t, "message", responses.Output[0].Type)
	require.Equal(t, "done", responses.Output[0].Content[0].Text)
	require.Equal(t, "function_call", responses.Output[1].Type)
	require.Equal(t, "lookup", responses.Output[1].Name)
	require.Equal(t, `{"id":1}`, responses.Output[1].ArgumentsString())
	require.Equal(t, 5, responses.Usage.InputTokens)
	require.Equal(t, 7, responses.Usage.OutputTokens)
}

func TestChatResponseToResponsesMapsFinishReasons(t *testing.T) {
	tests := []struct {
		name             string
		finishReason     string
		expectedStatus   string
		expectedReason   string
		expectIncomplete bool
	}{
		{name: "stop", finishReason: constant.FinishReasonStop, expectedStatus: "completed"},
		{name: "tool calls", finishReason: constant.FinishReasonToolCalls, expectedStatus: "completed"},
		{name: "length", finishReason: constant.FinishReasonLength, expectedStatus: "incomplete", expectedReason: "max_output_tokens", expectIncomplete: true},
		{name: "content filter", finishReason: constant.FinishReasonContentFilter, expectedStatus: "incomplete", expectedReason: "content_filter", expectIncomplete: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := dto.Message{Role: "assistant"}
			message.SetStringContent("partial")
			chat := &dto.OpenAITextResponse{
				Id:      "chatcmpl-finish-reason",
				Created: 123,
				Model:   "test-model",
				Choices: []dto.OpenAITextResponseChoice{{Message: message, FinishReason: tt.finishReason}},
			}

			responses, err := ChatResponseToResponses(chat)
			require.NoError(t, err)
			require.Equal(t, `"`+tt.expectedStatus+`"`, string(responses.Status))
			require.NotEmpty(t, responses.Output)
			for _, item := range responses.Output {
				require.Equal(t, tt.expectedStatus, item.Status)
			}
			if tt.expectIncomplete {
				require.NotNil(t, responses.IncompleteDetails)
				require.Equal(t, tt.expectedReason, responses.IncompleteDetails.Reasoning)
			} else {
				require.Nil(t, responses.IncompleteDetails)
			}
		})
	}
}
