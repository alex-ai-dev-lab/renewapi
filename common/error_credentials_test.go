package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminErrorRedactionKeepsHostAndRemovesCredentials(t *testing.T) {
	got := RedactErrorCredentials(`https://private.example/v1?api_key=secret Authorization: Bearer top-secret sk-test-credential failed`, "top-secret")
	require.Contains(t, got, "private.example")
	require.NotContains(t, got, "secret")
	require.NotContains(t, got, "sk-test-credential")
}
