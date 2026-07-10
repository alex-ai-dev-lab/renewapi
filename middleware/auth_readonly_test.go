package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestTokenStatusAllowsReadOnlyRejectsOnlyDisabled(t *testing.T) {
	require.True(t, tokenStatusAllowsReadOnly(common.TokenStatusEnabled))
	require.True(t, tokenStatusAllowsReadOnly(common.TokenStatusExpired))
	require.True(t, tokenStatusAllowsReadOnly(common.TokenStatusExhausted))
	require.False(t, tokenStatusAllowsReadOnly(common.TokenStatusDisabled))
}
