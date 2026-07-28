package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheCreationTokensTotalUsesLargerDialectField(t *testing.T) {
	require.Equal(t, 20, (InputTokenDetails{CachedCreationTokens: 20, CacheWriteTokens: 10}).CacheCreationTokensTotal())
	require.Equal(t, 30, (InputTokenDetails{CachedCreationTokens: 20, CacheWriteTokens: 30}).CacheCreationTokensTotal())
}
