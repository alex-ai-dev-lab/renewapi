package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func configureModelEndpointRouteTest(t *testing.T, endpoints map[int]map[string]*ModelEndpoint) {
	t.Helper()

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true

	modelEndpointCacheLock.Lock()
	oldCache := modelEndpointCache
	modelEndpointCache = endpoints
	modelEndpointCacheLock.Unlock()

	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [
			{"match_type": "prefix", "pattern": "gpt", "channel_type": 1},
			{"match_type": "prefix", "pattern": "claude", "channel_type": 14},
			{"match_type": "prefix", "pattern": "gemini", "channel_type": 24}
		]
	}`))

	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		modelEndpointCacheLock.Lock()
		modelEndpointCache = oldCache
		modelEndpointCacheLock.Unlock()
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})
}

func TestResolveModelRoute_GlobalDefaultsDoNotOverrideCodexByDefault(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{})
	channel := &Channel{Id: 151, Type: constant.ChannelTypeCodex, BaseURL: common.GetPointer("")}

	channelType, baseURL, overridden := ResolveModelRoute(channel, "gpt-5.5")

	require.Equal(t, constant.ChannelTypeCodex, channelType)
	require.Equal(t, "https://chatgpt.com", baseURL)
	require.False(t, overridden)
}

func TestResolveModelRoute_GlobalDefaultsDoNotOverrideCodexEvenWhenEnabled(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{})
	channel := &Channel{
		Id:      151,
		Type:    constant.ChannelTypeCodex,
		BaseURL: common.GetPointer(""),
		Setting: common.GetPointer(`{"allow_model_protocol_override":true,"model_protocol_override_targets":["openai"]}`),
	}

	channelType, _, overridden := ResolveModelRoute(channel, "gpt-5.5")

	require.Equal(t, constant.ChannelTypeCodex, channelType)
	require.False(t, overridden)
}

func TestResolveModelRoute_GlobalDefaultsRequireChannelOptIn(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{})
	disabled := &Channel{Id: 1, Type: constant.ChannelTypeOpenAI, BaseURL: common.GetPointer("")}
	enabled := &Channel{
		Id:      2,
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer(""),
		Setting: common.GetPointer(`{"allow_model_protocol_override":true,"model_protocol_override_targets":["anthropic"]}`),
	}

	channelType, _, overridden := ResolveModelRoute(disabled, "claude-sonnet-4")
	require.Equal(t, constant.ChannelTypeOpenAI, channelType)
	require.False(t, overridden)

	channelType, _, overridden = ResolveModelRoute(enabled, "claude-sonnet-4")
	require.Equal(t, constant.ChannelTypeAnthropic, channelType)
	require.True(t, overridden)
}

func TestResolveModelRoute_GlobalDefaultsRequireTargetAllowlist(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{})
	channel := &Channel{
		Id:      3,
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer("https://gateway.example"),
		Setting: common.GetPointer(`{"allow_model_protocol_override":true}`),
	}

	decision := ResolveModelRouteDecision(channel, "claude-sonnet-4")
	require.Equal(t, ModelRouteSourceNative, decision.Source)
	require.Equal(t, constant.ChannelTypeOpenAI, decision.ChannelType)
	require.False(t, decision.Overridden)
}

func TestResolveModelRouteDecisionKeepsGatewayBaseURL(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{})
	channel := &Channel{
		Id:      4,
		Type:    constant.ChannelTypeAnthropic,
		BaseURL: common.GetPointer("https://gateway.example"),
		Setting: common.GetPointer(`{"allow_model_protocol_override":true,"model_protocol_override_targets":["openai"]}`),
	}

	decision := ResolveModelRouteDecision(channel, "gpt-4.1")
	require.Equal(t, ModelRouteSourceGlobal, decision.Source)
	require.Equal(t, constant.EndpointTypeOpenAI, decision.Endpoint)
	require.Equal(t, "https://gateway.example", decision.BaseURL)
	require.True(t, decision.Overridden)
}

func TestResolveModelRoute_ExplicitModelEndpointCanOverrideWithoutChannelOptIn(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{
		10: {
			"claude-sonnet-4": {
				ChannelId:   10,
				Model:       "claude-sonnet-4",
				ChannelType: common.GetPointer(constant.ChannelTypeAnthropic),
			},
		},
	})
	channel := &Channel{Id: 10, Type: constant.ChannelTypeOpenAI, BaseURL: common.GetPointer("")}

	channelType, baseURL, overridden := ResolveModelRoute(channel, "claude-sonnet-4")

	require.Equal(t, constant.ChannelTypeAnthropic, channelType)
	require.Equal(t, "https://api.anthropic.com", baseURL)
	require.True(t, overridden)
}

func TestResolveModelRoute_AutoModelEndpointUsesNameInferenceWithoutChannelOptIn(t *testing.T) {
	configureModelEndpointRouteTest(t, map[int]map[string]*ModelEndpoint{
		11: {
			"gemini-2.5-pro": {
				ChannelId: 11,
				Model:     "gemini-2.5-pro",
			},
		},
	})
	channel := &Channel{Id: 11, Type: constant.ChannelTypeOpenAI, BaseURL: common.GetPointer("")}

	channelType, baseURL, overridden := ResolveModelRoute(channel, "gemini-2.5-pro")

	require.Equal(t, constant.ChannelTypeGemini, channelType)
	require.Equal(t, "https://generativelanguage.googleapis.com", baseURL)
	require.True(t, overridden)
}
