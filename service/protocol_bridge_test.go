package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestValidateResponsesTextBridgeRequestFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		req  dto.OpenAIResponsesRequest
	}{
		{name: "previous response", req: dto.OpenAIResponsesRequest{PreviousResponseID: "resp_1"}},
		{name: "conversation", req: dto.OpenAIResponsesRequest{Conversation: json.RawMessage(`{"id":"conv_1"}`)}},
		{name: "reasoning", req: dto.OpenAIResponsesRequest{Reasoning: &dto.Reasoning{Effort: "high"}}},
		{name: "image tool", req: dto.OpenAIResponsesRequest{Tools: json.RawMessage(`[{"type":"image_generation"}]`)}},
		{name: "mcp tool", req: dto.OpenAIResponsesRequest{Tools: json.RawMessage(`[{"type":"mcp","server_label":"x"}]`)}},
		{name: "image input", req: dto.OpenAIResponsesRequest{Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_image","image_url":"x"}]}]`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, ValidateResponsesTextBridgeRequest(&tt.req))
		})
	}
}

func TestValidateResponsesTextBridgeRequestAcceptsTextAndFunctions(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hi"}]},{"type":"function_call_output","call_id":"call_1","output":"ok"}]`),
		Tools: json.RawMessage(`[{"type":"function","name":"lookup","parameters":{"type":"object"}}]`),
	}
	require.NoError(t, ValidateResponsesTextBridgeRequest(req))
}
