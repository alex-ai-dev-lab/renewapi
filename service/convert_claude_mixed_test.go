package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2ClaudePreservesMixedOutput(t *testing.T) {
	for _, finish := range []string{"tool_calls", "length"} {
		t.Run(finish, func(t *testing.T) {
			message := dto.Message{Role: "assistant", Content: "Checking now."}
			message.SetToolCalls([]dto.ToolCallResponse{{ID: "call_a", Type: "function", Function: dto.FunctionResponse{Name: "lookup", Arguments: `{"q":"x"}`}}})
			out := ResponseOpenAI2Claude(&dto.OpenAITextResponse{Choices: []dto.OpenAITextResponseChoice{{Message: message, FinishReason: finish}}}, nil)
			content := out.Content
			require.Len(t, content, 2)
			require.Equal(t, "text", content[0].Type)
			require.Equal(t, "Checking now.", *content[0].Text)
			require.Equal(t, "tool_use", content[1].Type)
			require.Equal(t, "call_a", content[1].Id)
			require.Equal(t, map[string]any{"q": "x"}, content[1].Input)
			if finish == "length" {
				require.Equal(t, "max_tokens", out.StopReason)
			}
		})
	}
}
