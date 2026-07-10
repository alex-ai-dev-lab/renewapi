package service

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesPolicy(policy, channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesGlobal(channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesForChannel(channelID int, channelType int, baseURL string, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesForChannel(channelID, channelType, baseURL, model)
}

func ForcedModelDefaultEndpointForRelay(info *relaycommon.RelayInfo) (constant.EndpointType, bool) {
	if info == nil || info.ChannelMeta == nil || !info.RouteOverridden || info.RouteEndpoint == "" || !info.ChannelSetting.AllowModelProtocolOverride {
		return "", false
	}
	targetAllowed := false
	for _, configured := range info.ChannelSetting.ModelProtocolOverrideTargets {
		if constant.EndpointType(strings.TrimSpace(configured)) == info.RouteEndpoint {
			targetAllowed = true
			break
		}
	}
	if !targetAllowed {
		return "", false
	}
	switch info.RouteEndpoint {
	case constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAIResponseCompact,
		constant.EndpointTypeAnthropic:
		return info.RouteEndpoint, true
	default:
		return "", false
	}
}

func ShouldUseModelDefaultResponsesForRelay(info *relaycommon.RelayInfo) bool {
	endpoint, ok := ForcedModelDefaultEndpointForRelay(info)
	if !ok {
		return false
	}
	return endpoint == constant.EndpointTypeOpenAIResponse || endpoint == constant.EndpointTypeOpenAIResponseCompact
}

func ShouldUseModelDefaultTextEndpointForResponses(info *relaycommon.RelayInfo) (constant.EndpointType, bool) {
	endpoint, ok := ForcedModelDefaultEndpointForRelay(info)
	if !ok {
		return "", false
	}
	switch endpoint {
	case constant.EndpointTypeOpenAI, constant.EndpointTypeAnthropic:
		return endpoint, true
	default:
		return "", false
	}
}
