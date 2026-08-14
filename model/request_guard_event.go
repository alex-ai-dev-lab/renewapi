package model

import "strings"

type RequestGuardEvent struct {
	ID              int64  `json:"id" gorm:"primaryKey"`
	RequestID       string `json:"request_id" gorm:"type:varchar(128);index:idx_request_guard_request_id"`
	UserID          int    `json:"user_id" gorm:"index:idx_request_guard_user_created,priority:1"`
	TokenID         int    `json:"token_id"`
	Group           string `json:"group" gorm:"column:request_group;type:varchar(64)"`
	Protocol        string `json:"protocol" gorm:"type:varchar(32)"`
	Model           string `json:"model" gorm:"type:varchar(191)"`
	Mode            string `json:"mode" gorm:"type:varchar(16)"`
	Decision        string `json:"decision" gorm:"type:varchar(16);index:idx_request_guard_decision_created,priority:1"`
	ReasonCode      string `json:"reason_code" gorm:"type:varchar(128)"`
	CategoriesText  string `json:"categories_text" gorm:"type:text"`
	PromptHMAC      string `json:"prompt_hmac" gorm:"type:varchar(64)"`
	PromptRunes     int    `json:"prompt_runes"`
	Truncated       bool   `json:"truncated"`
	GuardEndpointID string `json:"guard_endpoint_id" gorm:"type:varchar(64)"`
	GuardModel      string `json:"guard_model" gorm:"type:varchar(191)"`
	PolicyVersion   string `json:"policy_version" gorm:"type:varchar(64)"`
	LatencyMs       int64  `json:"latency_ms"`
	RedactedPreview string `json:"redacted_preview,omitempty" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index:idx_request_guard_created;index:idx_request_guard_user_created,priority:2;index:idx_request_guard_decision_created,priority:2"`
}

func (RequestGuardEvent) TableName() string { return "request_guard_events" }

func CreateRequestGuardEvent(event *RequestGuardEvent) error {
	if event == nil {
		return nil
	}
	return DB.Create(event).Error
}

func ListRequestGuardEvents(beforeID int64, limit int, decision string) ([]RequestGuardEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events := make([]RequestGuardEvent, 0, limit)
	query := DB.Model(&RequestGuardEvent{})
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	if decision = strings.TrimSpace(decision); decision != "" {
		query = query.Where("decision = ?", decision)
	}
	err := query.Order("id desc").Limit(limit).Find(&events).Error
	return events, err
}
