package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesFinishReasonMaxOutputTokens(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		IncompleteDetails: &dto.IncompleteDetails{Reasoning: "max_output_tokens"},
	}
	resp.Status = []byte(`"incomplete"`)

	require.Equal(t, "length", ResponsesFinishReason(resp, false))
	require.Equal(t, "length", ResponsesFinishReason(resp, true))
}

func TestResponsesFinishReasonPreservesExistingFallbacks(t *testing.T) {
	completed := &dto.OpenAIResponsesResponse{
		IncompleteDetails: &dto.IncompleteDetails{Reasoning: "max_output_tokens"},
	}
	completed.Status = []byte(`"completed"`)
	require.Equal(t, "stop", ResponsesFinishReason(completed, false))

	unknownIncomplete := &dto.OpenAIResponsesResponse{
		IncompleteDetails: &dto.IncompleteDetails{Reasoning: "content_filter"},
	}
	unknownIncomplete.Status = []byte(`"incomplete"`)
	require.Equal(t, "stop", ResponsesFinishReason(unknownIncomplete, false))
	require.Equal(t, "tool_calls", ResponsesFinishReason(unknownIncomplete, true))
}

func TestResponsesResponseToChatCompletionsPreservesMaxOutputTokens(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Model:             "gpt-test",
		IncompleteDetails: &dto.IncompleteDetails{Reasoning: "max_output_tokens"},
		Output: []dto.ResponsesOutput{{
			Type:   "message",
			Role:   "assistant",
			Status: "incomplete",
			Content: []dto.ResponsesOutputContent{{
				Type: "output_text",
				Text: "partial",
			}},
		}},
	}
	resp.Status = []byte(`"incomplete"`)

	chatResp, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl-test")
	require.NoError(t, err)
	require.Len(t, chatResp.Choices, 1)
	require.Equal(t, "length", chatResp.Choices[0].FinishReason)
}
