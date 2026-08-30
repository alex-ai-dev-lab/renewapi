package reasonmap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeStopReasonToOpenAIFinishReasonLengthCases(t *testing.T) {
	tests := []string{
		"max_tokens",
		"model_context_window_exceeded",
	}
	for _, reason := range tests {
		t.Run(reason, func(t *testing.T) {
			require.Equal(t, "length", ClaudeStopReasonToOpenAIFinishReason(reason))
		})
	}
}

func TestOpenAIFinishReasonToClaudeStopReasonLength(t *testing.T) {
	require.Equal(t, "max_tokens", OpenAIFinishReasonToClaudeStopReason("length"))
}
