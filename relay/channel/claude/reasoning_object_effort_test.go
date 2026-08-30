package claude

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestClaudeAdaptorUsesReasoningObjectEffortForOpus47(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-7",
		Reasoning: json.RawMessage(`{"effort":"high"}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))
	require.Equal(t, "", request.ReasoningEffort)
}

func TestClaudeAdaptorUsesReasoningObjectEffortForOpus46(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{"effort":"high"}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "enabled", claudeRequest.Thinking.Type)
	require.NotNil(t, claudeRequest.Thinking.BudgetTokens)
	require.Equal(t, 4096, *claudeRequest.Thinking.BudgetTokens)
}

func TestClaudeAdaptorExplicitReasoningEffortTakesPrecedence(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:           "claude-opus-4-6",
		ReasoningEffort: "low",
		Reasoning:       json.RawMessage(`{"effort":"high"}`),
		Messages:        []dto.Message{{Role: "user", Content: "hello"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "enabled", claudeRequest.Thinking.Type)
	require.NotNil(t, claudeRequest.Thinking.BudgetTokens)
	require.Equal(t, 1280, *claudeRequest.Thinking.BudgetTokens)
}
