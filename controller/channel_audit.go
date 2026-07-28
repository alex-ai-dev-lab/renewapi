package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/channelconfig"

	"github.com/gin-gonic/gin"
)

type channelConfigAuditDTO struct {
	Id            int64                            `json:"id"`
	Action        string                           `json:"action"`
	OperatorId    int                              `json:"operator_id"`
	Reason        string                           `json:"reason"`
	RequestId     string                           `json:"request_id,omitempty"`
	ChangedFields []string                         `json:"changed_fields"`
	Changes       []channelconfig.AuditFieldChange `json:"changes"`
	ConfigVersion int64                            `json:"config_version"`
	CreatedAt     int64                            `json:"created_at"`
}

func GetChannelConfigAudits(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 { c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"}); return }
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	beforeID, _ := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	audits, err := model.ListConfigAuditsBefore("channel", id, beforeID, limit)
	if err != nil { common.ApiError(c, err); return }
	data := make([]channelConfigAuditDTO, 0, len(audits))
	for _, audit := range audits {
		var document struct { ChangedFields []string `json:"changed_fields"`; Changes []channelconfig.AuditFieldChange `json:"changes"` }
		_ = common.Unmarshal([]byte(audit.Diff), &document)
		if document.ChangedFields == nil { document.ChangedFields = []string{} }
		if document.Changes == nil { document.Changes = []channelconfig.AuditFieldChange{} }
		data = append(data, channelConfigAuditDTO{Id: audit.Id, Action: audit.Action, OperatorId: audit.OperatorId, Reason: audit.Reason,
			RequestId: audit.RequestId, ChangedFields: document.ChangedFields, Changes: document.Changes, ConfigVersion: audit.ConfigVersion, CreatedAt: audit.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
