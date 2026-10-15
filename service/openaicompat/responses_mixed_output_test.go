package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesToChatPreservesTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{
		{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "Checking now."}}},
		{Type: "function_call", ID: "fc_1", CallId: "call_1", Name: "lookup", Arguments: []byte(`"{}"`)},
	}}
	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chat-test")
	require.NoError(t, err)
	require.Equal(t, "Checking now.", chat.Choices[0].Message.Content)
	calls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, calls, 1)
	require.Equal(t, "call_1", calls[0].ID)
	require.Equal(t, "tool_calls", chat.Choices[0].FinishReason)
}
