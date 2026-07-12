package controller

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestConvertModelsDevToRatioDataUsesOnlyOfficialProviders(t *testing.T) {
	payload := `{
		"openai": {
			"models": {
				"gpt-5.5": {
					"cost": {"input": 5, "output": 30, "cache_read": 0.5, "cache_write": 6.25}
				}
			}
		},
		"poe": {
			"models": {
				"openai/gpt-5.5": {
					"cost": {"input": 4.5455, "output": 27.2727, "cache_read": 0.4545}
				}
			}
		},
		"openrouter": {
			"models": {
				"openai/gpt-5.5": {
					"cost": {"input": 5, "output": 30, "cache_read": 0.5}
				}
			}
		},
		"anthropic": {
			"models": {
				"claude-opus-4-6": {
					"cost": {"input": 5, "output": 25, "cache_read": 0.5}
				}
			}
		},
		"alibaba-cn": {
			"models": {
				"glm-5.2": {
					"cost": {"input": 1.1, "output": 3.851, "cache_read": 0.275}
				},
				"qwen3-max": {
					"family": "qwen",
					"cost": {"input": 0.861, "output": 3.441, "cache_read": 0.0861}
				},
				"qwen-zero": {
					"cost": {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0}
				},
				"siliconflow/deepseek-v3.2": {
					"cost": {"input": 0.1, "output": 0.2, "cache_read": 0.01}
				},
				"MiniMax/MiniMax-M2.7": {
					"cost": {"input": 0.2, "output": 0.4, "cache_read": 0.02}
				},
				"future-qwen-codename": {
					"family": "qwen",
					"cost": {"input": 0.8, "output": 3.2}
				}
			}
		},
		"alibaba": {
			"models": {
				"qwen3-max": {
					"family": "qwen",
					"cost": {"input": 1.2, "output": 6, "cache_read": 0.12}
				}
			}
		},
		"zai": {
			"models": {
				"glm-5.2": {
					"cost": {"input": 1.4, "output": 4.4, "cache_read": 0.26, "cache_write": 0}
				}
			}
		},
		"vercel": {
			"models": {
				"anthropic/claude-opus-4-6": {
					"cost": {"input": 4, "output": 20, "cache_read": 0.4}
				}
			}
		}
	}`

	converted, err := convertModelsDevToRatioData(strings.NewReader(payload))
	require.NoError(t, err)

	modelRatios := converted["model_ratio"].(map[string]any)
	completionRatios := converted["completion_ratio"].(map[string]any)
	cacheRatios := converted["cache_ratio"].(map[string]any)
	createCacheRatios := converted["create_cache_ratio"].(map[string]any)
	officialNames := converted[modelsDevOfficialNamesField].(map[string]any)

	require.Equal(t, 2.5, modelRatios["gpt-5.5"])
	require.Equal(t, 6.0, completionRatios["gpt-5.5"])
	require.Equal(t, 0.1, cacheRatios["gpt-5.5"])
	require.Equal(t, 1.25, createCacheRatios["gpt-5.5"])
	require.Equal(t, true, officialNames["gpt-5.5"])
	require.NotContains(t, modelRatios, "openai/gpt-5.5")

	require.Equal(t, 2.5, modelRatios["claude-opus-4-6"])
	require.Equal(t, 5.0, completionRatios["claude-opus-4-6"])
	require.Equal(t, 0.1, cacheRatios["claude-opus-4-6"])
	require.NotContains(t, modelRatios, "anthropic/claude-opus-4-6")

	require.Equal(t, 0.6, modelRatios["qwen3-max"])
	require.Equal(t, 5.0, completionRatios["qwen3-max"])
	require.Equal(t, 0.1, cacheRatios["qwen3-max"])
	require.Equal(t, 0.0, modelRatios["qwen-zero"])
	require.Equal(t, 0.0, completionRatios["qwen-zero"])
	require.Equal(t, 0.0, cacheRatios["qwen-zero"])
	require.Equal(t, 0.0, createCacheRatios["qwen-zero"])
	require.NotContains(t, modelRatios, "siliconflow/deepseek-v3.2")
	require.NotContains(t, modelRatios, "MiniMax/MiniMax-M2.7")
	require.Equal(t, 0.4, modelRatios["future-qwen-codename"])
	require.Equal(t, 4.0, completionRatios["future-qwen-codename"])
	require.Equal(t, 0.7, modelRatios["glm-5.2"])
	require.InDelta(t, 4.4/1.4, completionRatios["glm-5.2"], 1e-12)
	require.InDelta(t, 0.26/1.4, cacheRatios["glm-5.2"], 1e-12)
	require.Equal(t, 0.0, createCacheRatios["glm-5.2"])
}

func TestReplaceOfficialRatioFieldOverwritesOfficialModelsOnly(t *testing.T) {
	current := map[string]float64{
		"grok-4.3-high":    99,
		"claude-opus-4-7":  2.5,
		"manual-local-one": 12,
		"stale-official":   42,
	}
	official := map[string]any{
		"grok-4.3-high":    1.5,
		"claude-opus-4-7":  7.5,
		"claude-opus-4-8":  2.5,
		"bad-price-format": "skip-me",
	}
	officialModels := map[string]bool{
		"grok-4.3-high":   true,
		"claude-opus-4-7": true,
		"claude-opus-4-8": true,
		"stale-official":  true,
	}

	syncModels := map[string]bool{
		"grok-4.3-high":   true,
		"claude-opus-4-7": true,
		"claude-opus-4-8": true,
		"stale-official":  true,
	}
	merged, changed := replaceOfficialRatioField(current, official, officialModels, syncModels)

	require.Equal(t, 4, changed)
	require.Equal(t, 1.5, merged["grok-4.3-high"])
	require.Equal(t, 7.5, merged["claude-opus-4-7"])
	require.Equal(t, 12.0, merged["manual-local-one"])
	require.Equal(t, 2.5, merged["claude-opus-4-8"])
	require.NotContains(t, merged, "stale-official")
	require.NotContains(t, merged, "bad-price-format")
}

func TestReplaceOfficialRatioFieldPreservesInactiveOfficialModels(t *testing.T) {
	current := map[string]float64{"active": 9, "inactive": 7}
	official := map[string]any{"active": 1.5, "inactive": 2.5}
	officialModels := map[string]bool{"active": true, "inactive": true}
	syncModels := map[string]bool{"active": true}

	merged, changed := replaceOfficialRatioField(current, official, officialModels, syncModels)

	require.Equal(t, 1, changed)
	require.Equal(t, 1.5, merged["active"])
	require.Equal(t, 7.0, merged["inactive"])
}

func TestBuildOfficialSyncModelSetIncludesExistingLegacyPrices(t *testing.T) {
	modelRatios := ratio_setting.GetModelRatioCopy()
	completionRatios := ratio_setting.GetCompletionRatioCopy()
	cacheRatios := ratio_setting.GetCacheRatioCopy()
	createCacheRatios := ratio_setting.GetCreateCacheRatioCopy()

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"legacy-model":1}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"legacy-output-model":2}`))
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"legacy-cache-model":0.1}`))
	require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(`{"legacy-create-cache-model":1.25}`))
	t.Cleanup(func() {
		modelJSON, _ := json.Marshal(modelRatios)
		completionJSON, _ := json.Marshal(completionRatios)
		cacheJSON, _ := json.Marshal(cacheRatios)
		createCacheJSON, _ := json.Marshal(createCacheRatios)
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(modelJSON)))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(string(completionJSON)))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(string(cacheJSON)))
		require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(string(createCacheJSON)))
	})

	models := buildOfficialSyncModelSet([]string{"new-enabled-model"})
	require.True(t, models["new-enabled-model"])
	require.True(t, models["legacy-model"])
	require.True(t, models["legacy-output-model"])
	require.True(t, models["legacy-cache-model"])
	require.True(t, models["legacy-create-cache-model"])
}

func TestNextOfficialSyncTimeUsesSevenAMLocalTime(t *testing.T) {
	location := time.FixedZone("Asia/Taipei", 8*60*60)

	beforeSeven := time.Date(2026, 6, 8, 6, 30, 0, 0, location)
	require.Equal(
		t,
		time.Date(2026, 6, 8, 7, 0, 0, 0, location),
		nextOfficialSyncTime(beforeSeven),
	)

	afterSeven := time.Date(2026, 6, 8, 7, 1, 0, 0, location)
	require.Equal(
		t,
		time.Date(2026, 6, 9, 7, 0, 0, 0, location),
		nextOfficialSyncTime(afterSeven),
	)
}
