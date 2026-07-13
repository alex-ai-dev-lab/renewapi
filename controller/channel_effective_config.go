package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/adminconfig"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func pointerLayer(source adminconfig.Source, sourceID string, value any, present bool) adminconfig.Layer {
	return adminconfig.Layer{Source: source, SourceID: sourceID, Present: present, Value: value}
}

func resolveChannelValue(channel *model.Channel, key string, value any, present bool, masked bool) adminconfig.EffectiveValue {
	channelID := strconv.Itoa(channel.Id)
	return adminconfig.Resolve(key, masked,
		pointerLayer(adminconfig.SourceGlobal, "", nil, false),
		pointerLayer(adminconfig.SourceGroup, channel.Group, nil, false),
		pointerLayer(adminconfig.SourceChannel, channelID, value, present),
		pointerLayer(adminconfig.SourceRequest, "", nil, false),
	)
}

// GetChannelEffectiveConfig exposes the same channel route and protocol
// decisions used at runtime without probing or contacting the upstream.
func GetChannelEffectiveConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		return
	}

	items := []adminconfig.EffectiveValue{
		resolveChannelValue(channel, "status", channel.Status, true, false),
		resolveChannelValue(channel, "base_url", channel.BaseURL, channel.BaseURL != nil, false),
		resolveChannelValue(channel, "group", channel.Group, strings.TrimSpace(channel.Group) != "", false),
		resolveChannelValue(channel, "priority", channel.Priority, channel.Priority != nil, false),
		resolveChannelValue(channel, "weight", channel.Weight, channel.Weight != nil, false),
		resolveChannelValue(channel, "models", channel.Models, strings.TrimSpace(channel.Models) != "", false),
		resolveChannelValue(channel, "model_mapping", channel.ModelMapping, channel.ModelMapping != nil, true),
		resolveChannelValue(channel, "key", channel.Key, strings.TrimSpace(channel.Key) != "", true),
	}

	data := gin.H{
		"generated_at": time.Now().Unix(),
		"channel_id":   channel.Id,
		"items":        items,
	}

	modelName := strings.TrimSpace(c.Query("model"))
	if modelName != "" {
		clientEndpoint := constant.EndpointType(c.DefaultQuery("client_endpoint", string(constant.EndpointTypeOpenAIResponse)))
		var clientFormat types.RelayFormat = types.RelayFormatOpenAIResponses
		switch clientEndpoint {
		case constant.EndpointTypeOpenAI:
			clientFormat = types.RelayFormatOpenAI
		case constant.EndpointTypeAnthropic:
			clientFormat = types.RelayFormatClaude
		}
		data["model"] = modelName
		data["route"] = model.ResolveModelRouteDecision(channel, modelName)
		data["capability"] = service.EvaluateChannelProtocolCapability(channel, modelName, clientFormat, nil)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
