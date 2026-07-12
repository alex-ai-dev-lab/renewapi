package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseModelMappingSupportsOrderedFallbacks(t *testing.T) {
	mapping, err := ParseModelMapping(`{
		"glm-5.2": ["@cf/zhipu-ai/glm-5.2", "TCADP/glm-5.2", "z-ai/glm-5.2", "GLM-5.2"],
		"gpt": "gpt-upstream"
	}`)
	require.NoError(t, err)
	require.Equal(t, []string{"@cf/zhipu-ai/glm-5.2", "TCADP/glm-5.2", "z-ai/glm-5.2", "GLM-5.2"}, mapping["glm-5.2"])
	require.Equal(t, []string{"gpt-upstream"}, mapping["gpt"])
}

func TestResolveModelMappingCandidatesExpandsChains(t *testing.T) {
	mapping := map[string][]string{
		"client":  {"primary", "secondary"},
		"primary": {"primary-v2"},
	}
	resolved, err := ResolveModelMappingCandidates(mapping, "client")
	require.NoError(t, err)
	require.Equal(t, []string{"primary-v2", "secondary"}, resolved)
}

func TestResolveModelMappingCandidatesRejectsCycles(t *testing.T) {
	mapping := map[string][]string{"a": {"b"}, "b": {"a"}}
	_, err := ResolveModelMappingCandidates(mapping, "a")
	require.ErrorContains(t, err, "cycle")
}
