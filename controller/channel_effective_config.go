package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/channelconfig"

	"github.com/gin-gonic/gin"
)

// GetChannelEffectiveConfig exposes persisted route and protocol decisions
// without probing or contacting the upstream.
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

	modelName := strings.TrimSpace(c.Query("model"))
	clientFormat, _ := relayFormatForAdminClientEndpoint(constant.EndpointTypeOpenAIResponse)
	if modelName != "" {
		clientEndpoint := constant.EndpointType(c.DefaultQuery("client_endpoint", string(constant.EndpointTypeOpenAIResponse)))
		var supported bool
		clientFormat, supported = relayFormatForAdminClientEndpoint(clientEndpoint)
		if !supported {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported client_endpoint"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channelconfig.BuildEffectiveConfigSnapshot(channel, modelName, clientFormat)})
}
