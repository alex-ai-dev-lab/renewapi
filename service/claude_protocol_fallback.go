package service

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// CanUseNativeClaudeProtocolFallback is shared by candidate selection and the
// runtime conversion path. It never overrides an explicit model endpoint.
func CanUseNativeClaudeProtocolFallback(channel *model.Channel, route model.ModelRouteDecision) bool {
	if channel == nil || channel.Type != constant.ChannelTypeAnthropic ||
		route.Source != model.ModelRouteSourceGlobal || !route.Normalized ||
		route.Endpoint != constant.EndpointTypeOpenAIResponse {
		return false
	}
	setting := channel.GetSetting()
	return setting.ResponsesRequestProfile != "codex" &&
		(setting.RequiresCodexIdentity == nil || !*setting.RequiresCodexIdentity)
}
