package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRoutingModelsKeepsExplicitTargetsAndAddsSource(t *testing.T) {
	mapping := `{"glm-5.2":["@cf/zhipu-ai/glm-5.2","TCADP/glm-5.2","z-ai/glm-5.2","GLM-5.2"]}`
	channel := &Channel{
		Models:       "@cf/zhipu-ai/glm-5.2,TCADP/glm-5.2,z-ai/glm-5.2,GLM-5.2,other-model",
		ModelMapping: common.GetPointer(mapping),
	}
	require.Equal(t, []string{
		"@cf/zhipu-ai/glm-5.2",
		"GLM-5.2",
		"TCADP/glm-5.2",
		"glm-5.2",
		"other-model",
		"z-ai/glm-5.2",
	}, channel.GetRoutingModels())
}

func TestGetRoutingModelsDoesNotAddUnlistedFallbackTargets(t *testing.T) {
	mapping := `{"glm-5.2":["vendor-primary","vendor-fallback"]}`
	channel := &Channel{
		Models:       "other-model",
		ModelMapping: common.GetPointer(mapping),
	}
	require.Equal(t, []string{"glm-5.2", "other-model"}, channel.GetRoutingModels())
}

func TestGetRoutingModelsKeepsDirectModelThatIsAlsoAliasTarget(t *testing.T) {
	mapping := `{"gpt-5.5-mini":"gpt-5.5","gpt-5-mini":"gpt-5.5"}`
	channel := &Channel{
		Models:       "gpt-5.5,gpt-5.5-mini,gpt-5-mini",
		ModelMapping: common.GetPointer(mapping),
	}
	require.Equal(t, []string{"gpt-5-mini", "gpt-5.5", "gpt-5.5-mini"}, channel.GetRoutingModels())
}

func TestGetRoutingModelsKeepsLegacyModelsWithoutMapping(t *testing.T) {
	channel := &Channel{Models: "a,b"}
	require.Equal(t, []string{"a", "b"}, channel.GetRoutingModels())
}

func TestInitChannelCacheRoutesExplicitMappedTargetByPriority(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	highPriority := int64(1010)
	lowPriority := int64(9)
	highMapping := `{"gpt-5.5-mini":"gpt-5.5","gpt-5-mini":"gpt-5.5"}`
	channels := []*Channel{
		{
			Id:           71,
			Key:          "high-key",
			Status:       common.ChannelStatusEnabled,
			Name:         "high",
			Group:        "Test",
			Models:       "gpt-5.5,gpt-5.5-mini,gpt-5-mini",
			ModelMapping: common.GetPointer(highMapping),
			Priority:     &highPriority,
		},
		{
			Id:       80,
			Key:      "low-key",
			Status:   common.ChannelStatusEnabled,
			Name:     "low",
			Group:    "Test",
			Models:   "gpt-5.5",
			Priority: &lowPriority,
		},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}

	InitChannelCache()
	selected, err := GetRandomSatisfiedChannel("Test", "gpt-5.5", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 71, selected.Id)

	var ability Ability
	require.NoError(t, DB.Where(
		commonGroupCol+" = ? AND model = ? AND channel_id = ?",
		"Test", "gpt-5.5", 71,
	).First(&ability).Error)
	require.True(t, ability.Enabled)
}

func TestCacheUpdateChannelStatusUsesRoutingModels(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	priority := int64(10)
	mapping := `{"client-model":"upstream-model"}`
	channel := &Channel{
		Id:           71,
		Status:       common.ChannelStatusEnabled,
		Group:        "Test",
		Models:       "other-model",
		ModelMapping: common.GetPointer(mapping),
		Priority:     &priority,
	}
	channelsIDM = map[int]*Channel{71: channel}
	group2model2channels = map[string]map[string][]int{
		"Test": {
			"client-model": {71},
			"other-model":  {71},
		},
	}

	CacheUpdateChannelStatus(71, common.ChannelStatusManuallyDisabled)
	require.Empty(t, group2model2channels["Test"]["client-model"])
	require.Empty(t, group2model2channels["Test"]["other-model"])

	CacheUpdateChannelStatus(71, common.ChannelStatusEnabled)
	require.Equal(t, []int{71}, group2model2channels["Test"]["client-model"])
	require.Equal(t, []int{71}, group2model2channels["Test"]["other-model"])
	require.NotContains(t, group2model2channels["Test"], "upstream-model")
}
