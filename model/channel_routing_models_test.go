package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRoutingModelsHidesFallbackTargetsAndAddsSource(t *testing.T) {
	mapping := `{"glm-5.2":["@cf/zhipu-ai/glm-5.2","TCADP/glm-5.2","z-ai/glm-5.2","GLM-5.2"]}`
	channel := &Channel{
		Models:       "@cf/zhipu-ai/glm-5.2,TCADP/glm-5.2,z-ai/glm-5.2,GLM-5.2,other-model",
		ModelMapping: common.GetPointer(mapping),
	}
	require.Equal(t, []string{"glm-5.2", "other-model"}, channel.GetRoutingModels())
}

func TestGetRoutingModelsKeepsLegacyModelsWithoutMapping(t *testing.T) {
	channel := &Channel{Models: "a,b"}
	require.Equal(t, []string{"a", "b"}, channel.GetRoutingModels())
}
