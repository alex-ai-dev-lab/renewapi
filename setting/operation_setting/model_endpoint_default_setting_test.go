package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func replaceModelEndpointDefaultsForTest(t *testing.T, defaults ModelEndpointDefaults) {
	t.Helper()

	modelEndpointDefaultsLock.Lock()
	previous := modelEndpointDefaults
	modelEndpointDefaults = defaults
	modelEndpointDefaultsLock.Unlock()

	t.Cleanup(func() {
		modelEndpointDefaultsLock.Lock()
		modelEndpointDefaults = previous
		modelEndpointDefaultsLock.Unlock()
	})
}

func TestDefaultModelEndpointDefaultsDisabled(t *testing.T) {
	require.False(t, defaultModelEndpointDefaults().Enabled)
}

func TestImageDefaultProfilesSupportEdits(t *testing.T) {
	defaults := defaultModelEndpointDefaults()
	imagePatterns := map[string]bool{
		"gpt-image":        false,
		"dall-e":           false,
		"imagen":           false,
		"flux":             false,
		"stable-diffusion": false,
		"sd3":              false,
		"recraft":          false,
		"ideogram":         false,
	}

	for _, entry := range defaults.Entries {
		if entry.DefaultEndpoint != endpoint(constant.EndpointTypeImageGeneration) {
			continue
		}
		if _, ok := imagePatterns[entry.Pattern]; !ok {
			continue
		}
		imagePatterns[entry.Pattern] = true
		require.Contains(t, normalizeEndpointList(entry.SupportedEndpoints), endpoint(constant.EndpointTypeImageEdits), entry.Pattern)
	}

	for pattern, found := range imagePatterns {
		require.True(t, found, "missing image profile %s", pattern)
	}
}

func TestResolveModelEndpointDecision_DefaultDisabledDoesNotMatch(t *testing.T) {
	replaceModelEndpointDefaultsForTest(t, defaultModelEndpointDefaults())

	_, ok := ResolveModelEndpointDecision("gpt-image-1", endpoint(constant.EndpointTypeImageEdits))

	require.False(t, ok)
}

func TestResolveModelEndpointDecision_ImageEditsSupported(t *testing.T) {
	defaults := defaultModelEndpointDefaults()
	defaults.Enabled = true
	replaceModelEndpointDefaultsForTest(t, defaults)

	decision, ok := ResolveModelEndpointDecision("gpt-image-1", endpoint(constant.EndpointTypeImageEdits))

	require.True(t, ok)
	require.True(t, decision.Supported)
	require.False(t, decision.AutoCorrected)
	require.Equal(t, endpoint(constant.EndpointTypeImageEdits), decision.EffectiveEndpoint)
}

func TestResolveModelEndpointDecision_LongestPrefixWins(t *testing.T) {
	defaults := ModelEndpointDefaults{
		Enabled: true,
		Entries: []ModelEndpointDefaultEntry{
			profile("prefix", "gpt", 1, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAI),
			profile("prefix", "gpt-image", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
		},
	}
	replaceModelEndpointDefaultsForTest(t, defaults)

	decision, ok := ResolveModelEndpointDecision("gpt-image-1", endpoint(constant.EndpointTypeImageEdits))

	require.True(t, ok)
	require.True(t, decision.Supported)
	require.Equal(t, endpoint(constant.EndpointTypeImageEdits), decision.EffectiveEndpoint)
	require.Equal(t, endpoint(constant.EndpointTypeImageGeneration), decision.DefaultEndpoint)
}

func TestResolveModelEndpointDecision_ExactMatchWinsOverPrefix(t *testing.T) {
	defaults := ModelEndpointDefaults{
		Enabled: true,
		Entries: []ModelEndpointDefaultEntry{
			profile("prefix", "gpt-image", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("exact", "gpt-image-1", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
		},
	}
	replaceModelEndpointDefaultsForTest(t, defaults)

	decision, ok := ResolveModelEndpointDecision("gpt-image-1", endpoint(constant.EndpointTypeEmbeddings))

	require.True(t, ok)
	require.True(t, decision.Supported)
	require.Equal(t, endpoint(constant.EndpointTypeEmbeddings), decision.EffectiveEndpoint)
	require.Equal(t, endpoint(constant.EndpointTypeEmbeddings), decision.DefaultEndpoint)
}

func TestResolveModelEndpointDecision_SafeTextCorrection(t *testing.T) {
	defaults := ModelEndpointDefaults{
		Enabled: true,
		Entries: []ModelEndpointDefaultEntry{
			profile("prefix", "gpt-5", 1, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAI),
		},
	}
	replaceModelEndpointDefaultsForTest(t, defaults)

	decision, ok := ResolveModelEndpointDecision("gpt-5.5", endpoint(constant.EndpointTypeAnthropic))

	require.True(t, ok)
	require.True(t, decision.Supported)
	require.True(t, decision.AutoCorrected)
	require.Equal(t, endpoint(constant.EndpointTypeOpenAI), decision.EffectiveEndpoint)
	require.Equal(t, "safe endpoint correction", decision.Reason)
}

func TestResolveModelEndpointDecision_UnsupportedNonTextEndpoint(t *testing.T) {
	defaults := ModelEndpointDefaults{
		Enabled: true,
		Entries: []ModelEndpointDefaultEntry{
			profile("prefix", "text-embedding", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
		},
	}
	replaceModelEndpointDefaultsForTest(t, defaults)

	decision, ok := ResolveModelEndpointDecision("text-embedding-3-small", endpoint(constant.EndpointTypeOpenAI))

	require.True(t, ok)
	require.False(t, decision.Supported)
	require.False(t, decision.AutoCorrected)
	require.Equal(t, endpoint(constant.EndpointTypeEmbeddings), decision.EffectiveEndpoint)
	require.Equal(t, "unsupported endpoint for model", decision.Reason)
}
