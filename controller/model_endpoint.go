package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

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

func normalizeModelEndpointPayloads(channelID int, payload []modelEndpointPayload) ([]*model.ModelEndpoint, error) {
	if channelID <= 0 {
		return nil, fmt.Errorf("invalid channel id")
	}
	endpoints := make([]*model.ModelEndpoint, 0, len(payload))
	seen := make(map[string]struct{}, len(payload))
	for index, item := range payload {
		modelName := strings.TrimSpace(item.Model)
		if modelName == "" {
			return nil, fmt.Errorf("model endpoint %d: model is required", index+1)
		}
		if len([]rune(modelName)) > 255 {
			return nil, fmt.Errorf("model endpoint %d: model exceeds 255 characters", index+1)
		}
		if _, exists := seen[modelName]; exists {
			return nil, fmt.Errorf("duplicate model endpoint: %s", modelName)
		}
		seen[modelName] = struct{}{}

		baseURL := strings.TrimSpace(item.BaseURL)
		if len([]rune(baseURL)) > 512 {
			return nil, fmt.Errorf("model endpoint %d: base_url exceeds 512 characters", index+1)
		}
		if baseURL != "" {
			parsed, err := url.Parse(baseURL)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return nil, fmt.Errorf("model endpoint %d: base_url must be an absolute HTTP(S) URL", index+1)
			}
		}

		endpoints = append(endpoints, &model.ModelEndpoint{
			ChannelId:   channelID,
			Model:       modelName,
			BaseURL:     baseURL,
			ChannelType: item.ChannelType,
		})
	}
	return endpoints, nil
}

// PreviewChannelModelRoute shows the same route and bridge decision used by
// production channel selection without sending an upstream request.
func PreviewChannelModelRoute(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model is required"})
		return
	}
	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		return
	}
	clientEndpoint := constant.EndpointType(c.DefaultQuery("client_endpoint", string(constant.EndpointTypeOpenAIResponse)))
	var clientFormat types.RelayFormat = types.RelayFormatOpenAIResponses
	switch clientEndpoint {
	case constant.EndpointTypeOpenAI:
		clientFormat = types.RelayFormatOpenAI
	case constant.EndpointTypeAnthropic:
		clientFormat = types.RelayFormatClaude
	}
	decision := model.ResolveModelRouteDecision(channel, modelName)
	capability := service.EvaluateChannelProtocolCapability(channel, modelName, clientFormat, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"model":           modelName,
			"client_endpoint": clientEndpoint,
			"route":           decision,
			"capability":      capability,
		},
	})
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
	endpoints, err := normalizeModelEndpointPayloads(id, payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.ReplaceChannelModelEndpoints(id, endpoints); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
