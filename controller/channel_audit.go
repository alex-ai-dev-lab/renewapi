package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type channelAuditDiff struct {
	ChangedFields      []string `json:"changed_fields"`
	KeyChanged         bool     `json:"key_changed"`
	ModelEndpointCount *int     `json:"model_endpoint_count,omitempty"`
}

func buildChannelConfigAudit(
	c *gin.Context,
	channel *PatchChannel,
	present map[string]json.RawMessage,
) (*model.ConfigAudit, error) {
	diff := channelAuditDiff{ChangedFields: make([]string, 0, len(present))}
	ignored := map[string]bool{
		"id":              true,
		"key":             true,
		"key_mode":        true,
		"clear_key":       true,
		"change_reason":   true,
		"model_endpoints": true,
	}
	for field := range present {
		if !ignored[field] {
			diff.ChangedFields = append(diff.ChangedFields, field)
		}
	}
	_, diff.KeyChanged = present["key"]
	if channel.ClearKey {
		diff.KeyChanged = true
	}
	if channel.ModelEndpoints != nil {
		count := len(*channel.ModelEndpoints)
		diff.ModelEndpointCount = &count
	}
	sort.Strings(diff.ChangedFields)
	diffJSON, err := common.Marshal(diff)
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(channel.ChangeReason)
	if reason == "" {
		reason = "channel editor update"
	}
	if len([]rune(reason)) > 255 {
		return nil, fmt.Errorf("change_reason must not exceed 255 characters")
	}
	return &model.ConfigAudit{
		ResourceType: "channel",
		ResourceId:   channel.Id,
		Action:       "update",
		OperatorId:   c.GetInt("id"),
		Reason:       reason,
		RequestId:    strings.TrimSpace(c.GetHeader("X-Request-ID")),
		Diff:         string(diffJSON),
		CreatedAt:    common.GetTimestamp(),
	}, nil
}

func GetChannelConfigAudits(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	audits, err := model.ListConfigAudits("channel", id, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": audits})
}
