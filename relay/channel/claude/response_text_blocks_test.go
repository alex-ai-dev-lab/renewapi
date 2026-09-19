package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponseClaude2OpenAIConcatenatesTextBlocks(t *testing.T) {
	first := "I'll search first. "
	middle := "Based on the search results, "
	last := "here is the answer."
	response := &dto.ClaudeResponse{
		Id:         "msg_test",
		Model:      "claude-test",
		StopReason: "end_turn",
		Content: []dto.ClaudeMediaMessage{
			{Type: "text", Text: &first},
			{Type: "web_search_tool_result"},
			{Type: "text", Text: &middle},
			{Type: "text", Text: &last},
		},
	}

	converted := ResponseClaude2OpenAI(response)
	require.Len(t, converted.Choices, 1)
	require.Equal(t, first+middle+last, converted.Choices[0].Message.StringContent())
	require.Equal(t, "stop", converted.Choices[0].FinishReason)
}
