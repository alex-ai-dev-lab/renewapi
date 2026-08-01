package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRewriteResponsesPassThroughModelPreservesUnknownFields(t *testing.T) {
	original := []byte(`{"model":"gpt-5.4-openai-compact","input":[],"provider_extension":{"enabled":true}}`)

	rewritten, err := rewriteResponsesPassThroughModel(original, "gpt-5.4")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(rewritten, &payload))
	require.Equal(t, "gpt-5.4", payload["model"])
	require.Equal(t, map[string]any{"enabled": true}, payload["provider_extension"])
}
