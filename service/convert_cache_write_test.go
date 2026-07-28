package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeUsageFromNativeOpenAICacheWrite(t *testing.T) {
	usage := buildClaudeUsageFromOpenAIUsage(&dto.Usage{
		PromptTokens: 100,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:     20,
			CacheWriteTokens: 30,
		},
	})
	require.Equal(t, 50, usage.InputTokens)
	require.Equal(t, 20, usage.CacheReadInputTokens)
	require.Equal(t, 30, usage.CacheCreationInputTokens)
}
