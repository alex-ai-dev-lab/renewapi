package channel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnsupportedCapabilityErrorIsTyped(t *testing.T) {
	err := NewUnsupportedCapabilityError("provider-a", "claude_messages")
	require.ErrorIs(t, err, ErrUnsupportedCapability)
	var typed *UnsupportedCapabilityError
	require.ErrorAs(t, err, &typed)
	require.Equal(t, "provider-a", typed.Provider)
	require.Equal(t, "claude_messages", typed.Capability)
	require.True(t, errors.Is(err, ErrUnsupportedCapability))
}
