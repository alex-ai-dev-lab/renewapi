package claude

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func requireClaudeInvalidRequest(t *testing.T, err error, message string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, message, err.Error())
	apiErr, ok := err.(*types.NewAPIError)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
}

func TestConvertOpenAIRequestRejectsNonStringToolSchemaType(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name: "lookup",
				Parameters: map[string]any{
					"type":       123,
					"properties": map[string]any{},
				},
			},
		}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.Nil(t, converted)
	requireClaudeInvalidRequest(t, err, `tool "lookup" parameters.type must be a string`)
}

func TestConvertOpenAIRequestRejectsNonStringStopEntry(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Stop: []any{"valid", float64(123)},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.Nil(t, converted)
	requireClaudeInvalidRequest(t, err, "stop[1] must be a string")
}

func TestConvertOpenAIRequestRejectsOpus47ManualReasoningBudget(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-7",
		Reasoning: json.RawMessage(`{"enabled":true,"max_tokens":4096}`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.Nil(t, converted)
	requireClaudeInvalidRequest(t, err, "claude-opus-4-7 does not support reasoning.max_tokens; use reasoning_effort instead")
}

func TestConvertOpenAIRequestAllowsOpus46ManualReasoningBudget(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{"enabled":true,"max_tokens":4096}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "enabled", claudeRequest.Thinking.Type)
	require.NotNil(t, claudeRequest.Thinking.BudgetTokens)
	require.Equal(t, 4096, *claudeRequest.Thinking.BudgetTokens)
}
