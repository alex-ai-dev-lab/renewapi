package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type responsesCapabilityProbeRequest struct {
	Model string `json:"model"`
}

func GetChannelModelCapabilities(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.ApiError(c, fmt.Errorf("invalid channel id"))
		return
	}
	capability := strings.TrimSpace(c.DefaultQuery("capability", model.ChannelCapabilityResponsesCompaction))
	if !strings.EqualFold(capability, model.ChannelCapabilityResponsesCompaction) {
		common.ApiError(c, fmt.Errorf("unsupported capability %q", capability))
		return
	}
	records, err := model.ListChannelModelCapabilities(channelID, model.ChannelCapabilityResponsesCompaction)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": records})
}

func ProbeChannelResponsesCompactionCapability(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.ApiError(c, fmt.Errorf("invalid channel id"))
		return
	}
	var request responsesCapabilityProbeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		common.ApiError(c, fmt.Errorf("model is required"))
		return
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	routable := false
	for _, candidate := range channel.GetRoutingModels() {
		if strings.EqualFold(candidate, request.Model) {
			routable = true
			break
		}
	}
	if !routable {
		common.ApiError(c, fmt.Errorf("model %q is not routable on channel %d", request.Model, channelID))
		return
	}
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	record, probeErr := probeResponsesCompactionCapabilityModel(channel, testUserID, request.Model)
	if probeErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": probeErr.Error(),
			"data":    record,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": record})
}
