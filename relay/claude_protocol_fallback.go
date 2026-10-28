package relay

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// A global default is a preferred protocol, not permission to discard source
// features. Only fall back to a channel's configured native Messages adaptor;
// explicit model endpoints and Codex credentials keep their configured route.
func useNativeClaudeProtocolFallback(c *gin.Context, info *relaycommon.RelayInfo) bool {
	channel, err := model.CacheGetChannel(info.ChannelId)
	if err != nil || !service.CanUseNativeClaudeProtocolFallback(channel, model.ModelRouteDecision{
		Source: model.ModelRouteSource(info.RouteSource), Normalized: info.ProtocolNormalized, Endpoint: info.RouteEndpoint,
	}) {
		return false
	}
	apiType, ok := common.ChannelType2APIType(channel.Type)
	if !ok {
		return false
	}
	// Global normalization retained the original host and credentials. Change
	// only the adaptor/route; do not select a new key or replay model mappings.
	info.ChannelType, info.ApiType = channel.Type, apiType
	info.RouteEndpoint = constant.EndpointTypeAnthropic
	info.RouteSource = string(model.ModelRouteSourceNative)
	info.RouteOverridden, info.ProtocolNormalized = false, false
	common.SetContextKey(c, constant.ContextKeyChannelType, info.ChannelType)
	common.SetContextKey(c, constant.ContextKeyChannelRouteEndpoint, string(info.RouteEndpoint))
	common.SetContextKey(c, constant.ContextKeyChannelRouteSource, info.RouteSource)
	common.SetContextKey(c, constant.ContextKeyChannelRouteOverridden, false)
	common.SetContextKey(c, constant.ContextKeyProtocolNormalized, false)
	service.RecordRequestNormalization(info, "used configured native Messages protocol to preserve unsupported Responses features")
	c.Header("X-NewAPI-Endpoint-Corrected", "")
	return true
}
