package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// modelEndpointPayload is the request shape accepted when replacing a channel's
// per-model endpoint overrides. ChannelType is a pointer so the client can send
// null to mean "auto-infer the protocol from the model name".
type modelEndpointPayload struct {
	Model       string `json:"model"`
	BaseURL     string `json:"base_url"`
	ChannelType *int   `json:"channel_type"`
}

// GetChannelModelEndpoints returns the per-model endpoint overrides for a channel.
func GetChannelModelEndpoints(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	endpoints, err := model.GetChannelModelEndpoints(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    endpoints,
	})
}

// UpdateChannelModelEndpoints atomically replaces the per-model endpoint
// overrides for a channel with the posted set.
func UpdateChannelModelEndpoints(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	var payload []modelEndpointPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	endpoints := make([]*model.ModelEndpoint, 0, len(payload))
	for _, p := range payload {
		endpoints = append(endpoints, &model.ModelEndpoint{
			ChannelId:   id,
			Model:       p.Model,
			BaseURL:     p.BaseURL,
			ChannelType: p.ChannelType,
		})
	}
	if err := model.ReplaceChannelModelEndpoints(id, endpoints); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
