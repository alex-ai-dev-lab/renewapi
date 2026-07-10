package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestBuiltInGPT5DefaultsToResponses(t *testing.T) {
	defaults := defaultModelEndpointDefaults()
	defaults.Enabled = true
	modelEndpointDefaultsLock.Lock()
	previous := modelEndpointDefaults
	modelEndpointDefaults = defaults
	modelEndpointDefaultsLock.Unlock()
	t.Cleanup(func() {
		modelEndpointDefaultsLock.Lock()
		modelEndpointDefaults = previous
		modelEndpointDefaultsLock.Unlock()
	})

	for _, modelName := range []string{"gpt-5.1", "gpt-5.5", "gpt5.5"} {
		profile, ok := ResolveModelDefaultProfile(modelName)
		require.True(t, ok)
		require.Equal(t, string(constant.EndpointTypeOpenAIResponse), profile.DefaultEndpoint)
	}
}
