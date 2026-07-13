package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type channelAuditSummary struct {
	ChangedFields      []string `json:"changed_fields"`
	KeyChanged         bool     `json:"key_changed"`
	ModelEndpointCount *int     `json:"model_endpoint_count,omitempty"`
}

type channelConfigAuditDTO struct {
	Id            int64               `json:"id"`
	Action        string              `json:"action"`
	OperatorId    int                 `json:"operator_id"`
	Reason        string              `json:"reason"`
	RequestId     string              `json:"request_id,omitempty"`
	Summary       channelAuditSummary `json:"summary"`
	ConfigVersion int64               `json:"config_version"`
	CreatedAt     int64               `json:"created_at"`
}

func toChannelConfigAuditDTO(audit model.ConfigAudit) channelConfigAuditDTO {
	summary := channelAuditSummary{ChangedFields: make([]string, 0)}
	_ = common.Unmarshal([]byte(audit.Diff), &summary)
	if summary.ChangedFields == nil {
		summary.ChangedFields = make([]string, 0)
	}
	return channelConfigAuditDTO{
		Id:            audit.Id,
		Action:        audit.Action,
		OperatorId:    audit.OperatorId,
		Reason:        audit.Reason,
		RequestId:     audit.RequestId,
		Summary:       summary,
		ConfigVersion: audit.ConfigVersion,
		CreatedAt:     audit.CreatedAt,
	}
}

func GetChannelConfigAudits(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	beforeId, _ := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	audits, err := model.ListConfigAuditsBefore("channel", id, beforeId, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]channelConfigAuditDTO, 0, len(audits))
	for _, audit := range audits {
		data = append(data, toChannelConfigAuditDTO(audit))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
