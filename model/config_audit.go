package model

import (
	"errors"

	"gorm.io/gorm"
)

type ConfigAudit struct {
	Id            int64  `json:"id" gorm:"primaryKey;index:idx_config_audit_resource,priority:3"`
	ResourceType  string `json:"resource_type" gorm:"type:varchar(32);index:idx_config_audit_resource,priority:1;not null"`
	ResourceId    int    `json:"resource_id" gorm:"index:idx_config_audit_resource,priority:2;not null"`
	Action        string `json:"action" gorm:"type:varchar(32);not null"`
	OperatorId    int    `json:"operator_id" gorm:"index"`
	Reason        string `json:"reason" gorm:"type:varchar(255)"`
	RequestId     string `json:"request_id" gorm:"type:varchar(128);index"`
	Diff          string `json:"diff" gorm:"type:text;not null"`
	ConfigVersion int64  `json:"config_version" gorm:"bigint;not null;default:0"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
}

func (ConfigAudit) TableName() string { return "config_audits" }

func CreateConfigAuditTx(tx *gorm.DB, audit *ConfigAudit) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if audit == nil {
		return errors.New("audit is required")
	}
	return tx.Create(audit).Error
}

func ListConfigAuditsBefore(resourceType string, resourceID int, beforeID int64, limit int) ([]ConfigAudit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	audits := make([]ConfigAudit, 0, limit)
	query := DB.Where("resource_type = ? AND resource_id = ?", resourceType, resourceID)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	err := query.Order("id desc").Limit(limit).Find(&audits).Error
	return audits, err
}
