package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesRequestToChatCompletionsRequestConvertsStringInput(t *testing.T) {
	input, _ := common.Marshal("hello")
	instructions, _ := common.Marshal("be concise")
	stream := true
	maxTokens := uint(128)
	req := &dto.OpenAIResponsesRequest{
		Model:           "claude-opus-4-8",
		Input:           input,
		Instructions:    instructions,
		Stream:          &stream,
		MaxOutputTokens: &maxTokens,
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)

	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", out.Model)
	require.Equal(t, &stream, out.Stream)
	require.Equal(t, &maxTokens, out.MaxTokens)
	require.Len(t, out.Messages, 2)
	require.Equal(t, "system", out.Messages[0].Role)
	require.Equal(t, "be concise", out.Messages[0].StringContent())
	require.Equal(t, "user", out.Messages[1].Role)
	require.Equal(t, "hello", out.Messages[1].StringContent())
}

func TestResponsesRequestToChatCompletionsRequestConvertsFunctionTool(t *testing.T) {
	input, _ := common.Marshal([]map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "weather"},
			},
		},
	})
	tools, _ := common.Marshal([]map[string]any{
		{
			"type":        "function",
			"name":        "get_weather",
			"description": "lookup",
			"parameters":  map[string]any{"type": "object"},
		},
	})
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt5.5",
		Input: input,
		Tools: tools,
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)

	require.NoError(t, err)
	require.Len(t, out.Messages, 1)
	require.Equal(t, "user", out.Messages[0].Role)
	require.Equal(t, "weather", out.Messages[0].StringContent())
	require.Len(t, out.Tools, 1)
	require.Equal(t, "function", out.Tools[0].Type)
	require.Equal(t, "get_weather", out.Tools[0].Function.Name)
}
