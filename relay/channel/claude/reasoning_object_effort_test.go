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

func TestClaudeAdaptorUsesReasoningObjectEffortLevels(t *testing.T) {
	tests := []struct {
		name         string
		effort       string
		budgetTokens int
	}{
		{name: "low", effort: "low", budgetTokens: 1280},
		{name: "medium", effort: "medium", budgetTokens: 2048},
		{name: "high", effort: "high", budgetTokens: 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model:     "claude-opus-4-6",
				Reasoning: json.RawMessage(`{"effort":"` + tt.effort + `"}`),
				Messages:  []dto.Message{{Role: "user", Content: "hello"}},
			}

			converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
			require.NoError(t, err)
			claudeRequest, ok := converted.(*dto.ClaudeRequest)
			require.True(t, ok)
			require.NotNil(t, claudeRequest.Thinking)
			require.Equal(t, "enabled", claudeRequest.Thinking.Type)
			require.NotNil(t, claudeRequest.Thinking.BudgetTokens)
			require.Equal(t, tt.budgetTokens, *claudeRequest.Thinking.BudgetTokens)
			require.Equal(t, "", request.ReasoningEffort)
		})
	}
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
	require.Equal(t, "low", request.ReasoningEffort)
}

func TestClaudeAdaptorDoesNotEnableExplicitlyDisabledReasoningObject(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{"enabled":false,"effort":"high"}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Nil(t, claudeRequest.Thinking)
	require.Equal(t, "", request.ReasoningEffort)
}

func TestPrepareOpenAIReasoningEffortForClaudeBoundaries(t *testing.T) {
	require.Nil(t, prepareOpenAIReasoningEffortForClaude(nil))

	noReasoning := &dto.GeneralOpenAIRequest{Model: "claude-opus-4-6"}
	require.Same(t, noReasoning, prepareOpenAIReasoningEffortForClaude(noReasoning))

	emptyReasoning := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{}`),
	}
	require.Same(t, emptyReasoning, prepareOpenAIReasoningEffortForClaude(emptyReasoning))

	unsupportedEffort := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{"effort":"minimal"}`),
	}
	require.Same(t, unsupportedEffort, prepareOpenAIReasoningEffortForClaude(unsupportedEffort))
}

func TestPrepareOpenAIReasoningEffortForClaudeNormalizesObjectEffort(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:     "claude-opus-4-6",
		Reasoning: json.RawMessage(`{"effort":" HIGH "}`),
	}

	prepared := prepareOpenAIReasoningEffortForClaude(request)
	require.NotSame(t, request, prepared)
	require.Equal(t, "high", prepared.ReasoningEffort)
	require.Equal(t, "", request.ReasoningEffort)
}
