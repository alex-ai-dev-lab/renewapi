package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldUseModelDefaultResponsesForRelayRequiresChannelOptIn(t *testing.T) {
	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [
			{"match_type":"exact","pattern":"gpt5.5","channel_type":1,"default_endpoint":"openai-response"}
		]
	}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})

	disabled := &relaycommon.RelayInfo{
		OriginModelName: "gpt5.5",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{}},
	}
	enabled := &relaycommon.RelayInfo{
		OriginModelName: "gpt5.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			RouteEndpoint:   constant.EndpointTypeOpenAIResponse,
			RouteOverridden: true,
			ChannelSetting: dto.ChannelSettings{
				AllowModelProtocolOverride:   true,
				ModelProtocolOverrideTargets: []string{string(constant.EndpointTypeOpenAIResponse)},
			},
		},
	}

	require.False(t, ShouldUseModelDefaultResponsesForRelay(disabled))
	require.True(t, ShouldUseModelDefaultResponsesForRelay(enabled))
	disabled.ChannelMeta.RouteEndpoint = constant.EndpointTypeOpenAIResponse
	disabled.ChannelMeta.RouteOverridden = true
	require.False(t, ShouldUseModelDefaultResponsesForRelay(disabled))
}

func TestShouldUseModelDefaultTextEndpointForResponses(t *testing.T) {
	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [
			{"match_type":"prefix","pattern":"claude","channel_type":14,"default_endpoint":"anthropic"}
		]
	}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-8",
		ChannelMeta: &relaycommon.ChannelMeta{
			RouteEndpoint:   constant.EndpointTypeAnthropic,
			RouteOverridden: true,
			ChannelSetting: dto.ChannelSettings{
				AllowModelProtocolOverride:   true,
				ModelProtocolOverrideTargets: []string{string(constant.EndpointTypeAnthropic)},
			},
		},
	}

	endpoint, ok := ShouldUseModelDefaultTextEndpointForResponses(info)
	require.True(t, ok)
	require.Equal(t, constant.EndpointTypeAnthropic, endpoint)
	require.False(t, ShouldUseModelDefaultResponsesForRelay(info))
}
