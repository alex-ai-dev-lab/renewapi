package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func configureResponsesChannelRouteTest(t *testing.T) {
	t.Helper()
	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [
			{"match_type":"prefix","pattern":"claude","channel_type":14,"default_endpoint":"anthropic"},
			{"match_type":"prefix","pattern":"gpt-5","channel_type":1,"default_endpoint":"openai-response"}
		]
	}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})
}

func TestChannelMatchesRetryRequirementsRequiresOpenAIResponsesSupport(t *testing.T) {
	param := &RetryParam{RequireOpenAIResponsesSupport: true}

	require.False(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type: constant.ChannelTypeAnthropic,
	}))
	require.True(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type: constant.ChannelTypeOpenAI,
	}))
	require.True(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type: constant.ChannelTypeCodex,
	}))
	require.False(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type:    constant.ChannelTypeAnthropic,
		BaseURL: common.GetPointer("https://example.test/codex"),
	}))
}

func TestChannelMatchesRetryRequirementsAllowsOptedInDefaultTextEndpoint(t *testing.T) {
	configureResponsesChannelRouteTest(t)
	param := &RetryParam{
		RequireOpenAIResponsesSupport: true,
		ModelName:                     "claude-opus-4-6",
	}

	require.True(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type:    constant.ChannelTypeAnthropic,
		Setting: common.GetPointer(`{"allow_model_protocol_override":true,"model_protocol_override_targets":["anthropic"]}`),
	}))
	require.False(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type: constant.ChannelTypeAnthropic,
	}))
}

func TestChannelMatchesRetryRequirementsStillRequiresResponsesForOptedInResponsesDefault(t *testing.T) {
	configureResponsesChannelRouteTest(t)
	param := &RetryParam{
		RequireOpenAIResponsesSupport: true,
		ModelName:                     "gpt-5.5",
	}

	require.True(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type:    constant.ChannelTypeAnthropic,
		Setting: common.GetPointer(`{"allow_model_protocol_override":true,"model_protocol_override_targets":["openai-response"]}`),
	}))
	require.True(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Setting: common.GetPointer(`{"allow_model_protocol_override":true}`),
	}))
}

func TestChannelMatchesRetryRequirementsFailsClosedWithoutTarget(t *testing.T) {
	configureResponsesChannelRouteTest(t)
	param := &RetryParam{RequireOpenAIResponsesSupport: true, ModelName: "claude-opus-4-6"}
	require.False(t, channelMatchesRetryRequirements(param, &model.Channel{
		Type:    constant.ChannelTypeAnthropic,
		Setting: common.GetPointer(`{"allow_model_protocol_override":true}`),
	}))
}
