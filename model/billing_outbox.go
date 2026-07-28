package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BillingOutbox struct {
	ID          uint64 `json:"id" gorm:"primaryKey"`
	EventKey    string `json:"event_key" gorm:"type:varchar(191);not null;uniqueIndex"`
	LedgerID    uint64 `json:"ledger_id" gorm:"not null;index"`
	RequestID   string `json:"request_id" gorm:"type:varchar(64);not null;index"`
	EventType   string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Payload     string `json:"payload" gorm:"type:text;not null"`
	Attempts    int    `json:"attempts" gorm:"not null;default:0"`
	AvailableAt int64  `json:"available_at" gorm:"not null;index"`
	ProcessedAt int64  `json:"processed_at" gorm:"not null;default:0;index"`
	LastError   string `json:"last_error" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"not null"`
	UpdatedAt   int64  `json:"updated_at" gorm:"not null"`
}

type BillingAuditEvent struct {
	ID        uint64 `json:"id" gorm:"primaryKey"`
	EventKey  string `json:"event_key" gorm:"type:varchar(191);not null;uniqueIndex"`
	LedgerID  uint64 `json:"ledger_id" gorm:"not null;index"`
	RequestID string `json:"request_id" gorm:"type:varchar(64);not null;index"`
	EventType string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Payload   string `json:"payload" gorm:"type:text;not null"`
	CreatedAt int64  `json:"created_at" gorm:"not null;index"`
}

type billingEventPayload struct {
	LedgerID       uint64 `json:"ledger_id"`
	RequestID      string `json:"request_id"`
	Kind           string `json:"kind"`
	Mode           string `json:"mode"`
	State          string `json:"state"`
	FundingSource  string `json:"funding_source"`
	UserID         int    `json:"user_id"`
	TokenID        int    `json:"token_id"`
	ChannelID      int    `json:"channel_id"`
	SubscriptionID int    `json:"subscription_id"`
	ReservedQuota  int64  `json:"reserved_quota"`
	ActualQuota    int64  `json:"actual_quota"`
	AppliedQuota   int64  `json:"applied_quota"`
	Version        int64  `json:"version"`
}

func enqueueBillingOutboxTx(tx *gorm.DB, ledger *BillingLedger, eventType string) error {
	payload, err := common.Marshal(billingEventPayload{
		LedgerID: ledger.ID, RequestID: ledger.RequestID, Kind: ledger.Kind, Mode: ledger.Mode, State: ledger.State,
		FundingSource: ledger.FundingSource, UserID: ledger.UserID, TokenID: ledger.TokenID, ChannelID: ledger.ChannelID, SubscriptionID: ledger.SubscriptionID,
		ReservedQuota: ledger.ReservedQuota, ActualQuota: ledger.ActualQuota, AppliedQuota: ledger.AppliedQuota, Version: ledger.Version,
	})
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	outbox := BillingOutbox{
		EventKey: fmt.Sprintf("%s:%s:%d", ledger.RequestID, eventType, ledger.Version),
		LedgerID: ledger.ID, RequestID: ledger.RequestID, EventType: eventType, Payload: string(payload),
		AvailableAt: now, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&outbox).Error
}

func ProcessBillingOutbox(limit int) (int, error) {
	return ProcessBillingOutboxContext(context.Background(), limit)
}

func ProcessBillingOutboxContext(ctx context.Context, limit int) (int, error) {
	if DB == nil || LOG_DB == nil {
		return 0, errors.New("billing outbox databases are not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	now := GetDBTimestamp()
	db := DB.WithContext(ctx)
	logDB := LOG_DB.WithContext(ctx)
	var items []BillingOutbox
	if err := db.Where("processed_at = 0 AND available_at <= ?", now).Order("id asc").Limit(limit).Find(&items).Error; err != nil {
		return 0, err
	}
	processed := 0
	for i := range items {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		item := items[i]
		event := BillingAuditEvent{
			EventKey: item.EventKey, LedgerID: item.LedgerID, RequestID: item.RequestID,
			EventType: item.EventType, Payload: item.Payload, CreatedAt: item.CreatedAt,
		}
		if err := logDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
			_ = db.Model(&BillingOutbox{}).Where("id = ? AND processed_at = 0", item.ID).Updates(map[string]any{
				"attempts": gorm.Expr("attempts + 1"), "available_at": now + billingRetryDelay(item.Attempts+1),
				"last_error": truncateBillingError(err.Error()), "updated_at": now,
			}).Error
			continue
		}
		res := db.Model(&BillingOutbox{}).Where("id = ? AND processed_at = 0", item.ID).Updates(map[string]any{
			"processed_at": now, "last_error": "", "updated_at": now,
		})
		if res.Error != nil {
			return processed, res.Error
		}
		if res.RowsAffected == 1 {
			processed++
		}
	}
	return processed, nil
}
