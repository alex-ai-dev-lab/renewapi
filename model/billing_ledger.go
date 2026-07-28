package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	BillingLedgerStateReserved          = "reserved"
	BillingLedgerStateSettling          = "settling"
	BillingLedgerStateSettled           = "settled"
	BillingLedgerStateCompensating      = "compensating"
	BillingLedgerStateRefunded          = "refunded"
	BillingLedgerStateReconcileRequired = "reconcile_required"
)

const (
	BillingLedgerDesiredSettle = "settled"
	BillingLedgerDesiredRefund = "refunded"
)

type BillingLedger struct {
	ID             uint64 `json:"id" gorm:"primaryKey"`
	RequestID      string `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	Kind           string `json:"kind" gorm:"type:varchar(24);not null;default:'request';index"`
	Mode           string `json:"mode" gorm:"type:varchar(16);not null;index"`
	State          string `json:"state" gorm:"type:varchar(32);not null;index"`
	DesiredState   string `json:"desired_state" gorm:"type:varchar(32);index"`
	FundingSource  string `json:"funding_source" gorm:"type:varchar(24);not null"`
	UserID         int    `json:"user_id" gorm:"not null;index"`
	TokenID        int    `json:"token_id" gorm:"index"`
	ChannelID      int    `json:"channel_id" gorm:"index"`
	SubscriptionID int    `json:"subscription_id" gorm:"index"`
	ReservedQuota  int64  `json:"reserved_quota" gorm:"not null;default:0"`
	ActualQuota    int64  `json:"actual_quota" gorm:"not null;default:0"`
	AppliedQuota   int64  `json:"applied_quota" gorm:"not null;default:0"`
	DesiredQuota   int64  `json:"desired_quota" gorm:"not null;default:0"`
	CountedQuota   int64  `json:"counted_quota" gorm:"not null;default:0"`
	RequestCounted bool   `json:"request_counted" gorm:"not null;default:false"`
	TokenUnlimited bool   `json:"token_unlimited" gorm:"not null;default:false"`
	Playground     bool   `json:"playground" gorm:"not null;default:false"`
	Version        int64  `json:"version" gorm:"not null;default:1"`
	Attempts       int    `json:"attempts" gorm:"not null;default:0"`
	NextRetryAt    int64  `json:"next_retry_at" gorm:"not null;default:0;index"`
	LastError      string `json:"last_error" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"not null;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"not null;index"`
}

type BillingReservation struct {
	RequestID      string
	Kind           string
	Mode           string
	FundingSource  string
	UserID         int
	TokenID        int
	ChannelID      int
	SubscriptionID int
	Quota          int64
	TokenUnlimited bool
	Playground     bool
	ApplyBalances  bool
}

type BillingReservationResult struct {
	Ledger          BillingLedger
	Subscription    *UserSubscription
	AlreadyReserved bool
}

func (l *BillingLedger) BeforeCreate(_ *gorm.DB) error {
	now := time.Now().Unix()
	if l.CreatedAt == 0 {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	if l.Version == 0 {
		l.Version = 1
	}
	return nil
}

func ReserveBillingLedger(input BillingReservation) (*BillingReservationResult, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RequestID == "" || len(input.RequestID) > 64 {
		return nil, errors.New("billing request id is invalid")
	}
	if input.UserID <= 0 || input.Quota < 0 {
		return nil, errors.New("billing reservation is invalid")
	}
	if input.Kind == "" {
		input.Kind = "request"
	}
	if input.FundingSource != "wallet" && input.FundingSource != "subscription" {
		return nil, fmt.Errorf("unsupported billing source %q", input.FundingSource)
	}

	result := &BillingReservationResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing BillingLedger
		query := lockForUpdate(tx).Where("request_id = ?", input.RequestID).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.UserID != input.UserID || existing.TokenID != input.TokenID || existing.Mode != input.Mode {
				return errors.New("billing request id already belongs to another reservation")
			}
			result.Ledger = existing
			result.AlreadyReserved = true
			if existing.SubscriptionID > 0 {
				var sub UserSubscription
				if err := tx.First(&sub, existing.SubscriptionID).Error; err != nil {
					return err
				}
				result.Subscription = &sub
			}
			return nil
		}

		ledger := BillingLedger{
			RequestID: input.RequestID, Kind: input.Kind, Mode: input.Mode,
			State: BillingLedgerStateReserved, FundingSource: input.FundingSource,
			UserID: input.UserID, TokenID: input.TokenID, ChannelID: input.ChannelID, SubscriptionID: input.SubscriptionID,
			ReservedQuota: input.Quota, AppliedQuota: input.Quota, DesiredQuota: input.Quota,
			TokenUnlimited: input.TokenUnlimited, Playground: input.Playground,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}

		if input.ApplyBalances {
			if input.FundingSource == "subscription" {
				sub, err := reserveSubscriptionQuotaTx(tx, input.RequestID, input.UserID, input.Quota)
				if err != nil {
					return err
				}
				ledger.SubscriptionID = sub.Id
				result.Subscription = sub
			} else if err := adjustWalletQuotaTx(tx, input.UserID, input.Quota); err != nil {
				return err
			}
			if err := adjustTokenQuotaTx(tx, &ledger, input.Quota); err != nil {
				return err
			}
		}
		ledger.UpdatedAt = time.Now().Unix()
		if err := tx.Model(&BillingLedger{}).Where("id = ?", ledger.ID).Updates(map[string]any{
			"subscription_id": ledger.SubscriptionID, "updated_at": ledger.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		if err := enqueueBillingOutboxTx(tx, &ledger, "reserved"); err != nil {
			return err
		}
		result.Ledger = ledger
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetBillingLedger(id uint64) (*BillingLedger, error) {
	if id == 0 {
		return nil, errors.New("billing ledger id is empty")
	}
	var ledger BillingLedger
	if err := DB.First(&ledger, id).Error; err != nil {
		return nil, err
	}
	return &ledger, nil
}

func InvalidateBillingBalanceCaches(userID int, tokenID int) {
	if !common.RedisEnabled {
		return
	}
	if userID > 0 {
		if err := invalidateUserCache(userID); err != nil {
			common.SysLog("failed to invalidate billing user cache: " + err.Error())
		}
	}
	if tokenID > 0 {
		var token Token
		if err := DB.Select(commonKeyCol).First(&token, tokenID).Error; err != nil {
			common.SysLog("failed to load billing token cache key: " + err.Error())
		} else if token.Key != "" {
			if err := cacheDeleteToken(token.Key); err != nil {
				common.SysLog("failed to invalidate billing token cache: " + err.Error())
			}
		}
	}
}

func SettleBillingLedger(id uint64, targetQuota int64) (*BillingLedger, error) {
	if targetQuota < 0 {
		return nil, errors.New("billing target quota cannot be negative")
	}
	return mutateBillingLedger(id, BillingLedgerDesiredSettle, targetQuota, nil)
}

func SettleBillingLedgerWithTask(id uint64, targetQuota int64, task *Task) (*BillingLedger, error) {
	if task == nil {
		return nil, errors.New("task is nil")
	}
	ledger, err := mutateBillingLedger(id, BillingLedgerDesiredSettle, targetQuota, func(tx *gorm.DB, ledger *BillingLedger) error {
		task.BillingLedgerID = ledger.ID
		task.BillingState = BillingLedgerStateSettled
		task.BillingVersion = ledger.Version + 1
		task.Quota = int(targetQuota)
		return tx.Create(task).Error
	})
	if err != nil {
		return nil, err
	}
	var count int64
	if err := DB.Model(&Task{}).Where("billing_ledger_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("billing ledger settled without a persisted task")
	}
	return ledger, nil
}

func BindBillingLedgerTask(id uint64, task *Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockBillingLedgerTx(tx, id)
		if err != nil {
			return err
		}
		if ledger.State != BillingLedgerStateReserved {
			return fmt.Errorf("billing ledger in state %s cannot bind a pending task", ledger.State)
		}
		var existing Task
		query := tx.Where("billing_ledger_id = ?", id).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			task.ID = existing.ID
			return nil
		}
		ledger.Kind = "task"
		ledger.Version++
		ledger.UpdatedAt = time.Now().Unix()
		if err := tx.Save(ledger).Error; err != nil {
			return err
		}
		task.BillingLedgerID = ledger.ID
		task.BillingState = ledger.State
		task.BillingVersion = ledger.Version
		task.Quota = int(ledger.AppliedQuota)
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		return enqueueBillingOutboxTx(tx, ledger, "task_bound")
	})
}

func AcknowledgeBillingLedgerTask(id uint64, targetQuota int64, task *Task) (*BillingLedger, error) {
	if task == nil || task.ID <= 0 {
		return nil, errors.New("pending task is invalid")
	}
	if targetQuota < 0 {
		return nil, errors.New("billing target quota cannot be negative")
	}
	var result BillingLedger
	err := DB.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockBillingLedgerTx(tx, id)
		if err != nil {
			return err
		}
		if ledger.State == BillingLedgerStateRefunded {
			return errors.New("refunded billing ledger cannot acknowledge an upstream task")
		}
		var current Task
		if err := lockForUpdate(tx).Where("id = ? AND billing_ledger_id = ?", task.ID, id).First(&current).Error; err != nil {
			return err
		}
		if ledger.State == BillingLedgerStateSettled && ledger.AppliedQuota == targetQuota {
			result = *ledger
			return nil
		}
		ledger.Kind = "task"
		ledger.State = BillingLedgerStateReconcileRequired
		ledger.DesiredState = BillingLedgerDesiredSettle
		ledger.DesiredQuota = targetQuota
		ledger.NextRetryAt = time.Now().Unix()
		ledger.LastError = "upstream task accepted; settlement pending"
		ledger.Version++
		ledger.UpdatedAt = time.Now().Unix()

		task.ID = current.ID
		task.CreatedAt = current.CreatedAt
		task.UpdatedAt = time.Now().Unix()
		task.SubmitTime = current.SubmitTime
		task.BillingLedgerID = ledger.ID
		task.BillingState = ledger.State
		task.BillingVersion = ledger.Version
		task.Quota = int(targetQuota)
		update := tx.Model(&Task{}).Where("id = ? AND billing_ledger_id = ?", task.ID, id).Select("*").Updates(task)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("pending task acknowledgement lost")
		}
		if err := tx.Save(ledger).Error; err != nil {
			return err
		}
		result = *ledger
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func SettleBillingLedgerWithMidjourney(id uint64, targetQuota int64, task *Midjourney) (*BillingLedger, error) {
	if task == nil {
		return nil, errors.New("midjourney task is nil")
	}
	ledger, err := mutateBillingLedger(id, BillingLedgerDesiredSettle, targetQuota, func(tx *gorm.DB, ledger *BillingLedger) error {
		task.BillingLedgerID = ledger.ID
		task.BillingState = BillingLedgerStateSettled
		task.BillingVersion = ledger.Version + 1
		task.Quota = int(targetQuota)
		return tx.Create(task).Error
	})
	if err != nil {
		return nil, err
	}
	var count int64
	if err := DB.Model(&Midjourney{}).Where("billing_ledger_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("billing ledger settled without a persisted midjourney task")
	}
	return ledger, nil
}

func RefundBillingLedger(id uint64, reason string) (*BillingLedger, error) {
	return mutateBillingLedger(id, BillingLedgerDesiredRefund, 0, func(tx *gorm.DB, ledger *BillingLedger) error {
		if reason != "" {
			ledger.LastError = truncateBillingError(reason)
		}
		return tx.Model(&Task{}).Where("billing_ledger_id = ? AND status = ?", ledger.ID, TaskStatusNotStart).Updates(map[string]any{
			"status": TaskStatusFailure, "progress": "100%", "fail_reason": truncateBillingError(reason),
		}).Error
	})
}

func ReserveMoreBillingLedger(id uint64, targetQuota int64) (*BillingLedger, error) {
	if targetQuota < 0 {
		return nil, errors.New("billing target quota cannot be negative")
	}
	var result BillingLedger
	err := DB.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockBillingLedgerTx(tx, id)
		if err != nil {
			return err
		}
		if ledger.State != BillingLedgerStateReserved || targetQuota <= ledger.AppliedQuota {
			result = *ledger
			return nil
		}
		delta := targetQuota - ledger.AppliedQuota
		if ledger.Mode == "enforce" {
			if err := adjustLedgerBalancesTx(tx, ledger, delta); err != nil {
				return err
			}
		}
		ledger.ReservedQuota = targetQuota
		ledger.AppliedQuota = targetQuota
		ledger.DesiredQuota = targetQuota
		ledger.Version++
		ledger.UpdatedAt = time.Now().Unix()
		if err := tx.Save(ledger).Error; err != nil {
			return err
		}
		if err := enqueueBillingOutboxTx(tx, ledger, "reserved"); err != nil {
			return err
		}
		result = *ledger
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func mutateBillingLedger(id uint64, desired string, targetQuota int64, beforeSave func(*gorm.DB, *BillingLedger) error) (*BillingLedger, error) {
	var result BillingLedger
	err := DB.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockBillingLedgerTx(tx, id)
		if err != nil {
			return err
		}
		changed, err := mutateLockedBillingLedgerTx(tx, ledger, desired, targetQuota, beforeSave)
		if err != nil {
			return err
		}
		_ = changed
		result = *ledger
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func mutateLockedBillingLedgerTx(tx *gorm.DB, ledger *BillingLedger, desired string, targetQuota int64, beforeSave func(*gorm.DB, *BillingLedger) error) (bool, error) {
	if desired != BillingLedgerDesiredSettle && desired != BillingLedgerDesiredRefund {
		return false, errors.New("unsupported billing ledger transition")
	}
	if ledger.State == BillingLedgerStateRefunded {
		if desired == BillingLedgerDesiredRefund {
			return false, nil
		}
		return false, errors.New("refunded billing ledger cannot be settled")
	}
	if desired == BillingLedgerDesiredSettle && ledger.State == BillingLedgerStateSettled && ledger.AppliedQuota == targetQuota {
		return false, nil
	}

	ledger.DesiredState = desired
	ledger.DesiredQuota = targetQuota
	ledger.Attempts++
	if desired == BillingLedgerDesiredSettle {
		ledger.State = BillingLedgerStateSettling
	} else {
		ledger.State = BillingLedgerStateCompensating
	}
	delta := targetQuota - ledger.AppliedQuota
	if ledger.Mode == "enforce" && delta != 0 {
		if err := adjustLedgerBalancesTx(tx, ledger, delta); err != nil {
			return false, err
		}
	}
	if beforeSave != nil {
		if err := beforeSave(tx, ledger); err != nil {
			return false, err
		}
	}
	if ledger.Mode == "enforce" {
		if err := updateBillingCountersTx(tx, ledger, targetQuota); err != nil {
			return false, err
		}
	}
	ledger.AppliedQuota = targetQuota
	ledger.ActualQuota = targetQuota
	ledger.NextRetryAt = 0
	if desired == BillingLedgerDesiredSettle {
		ledger.State = BillingLedgerStateSettled
		ledger.LastError = ""
	} else {
		ledger.State = BillingLedgerStateRefunded
	}
	ledger.Version++
	ledger.UpdatedAt = time.Now().Unix()
	if err := tx.Save(ledger).Error; err != nil {
		return false, err
	}
	taskUpdates := map[string]any{
		"billing_state": ledger.State, "billing_version": ledger.Version,
		"updated_at": ledger.UpdatedAt,
	}
	if desired == BillingLedgerDesiredSettle {
		taskUpdates["quota"] = targetQuota
	}
	if err := tx.Model(&Task{}).Where("billing_ledger_id = ?", ledger.ID).Updates(taskUpdates).Error; err != nil {
		return false, err
	}
	midjourneyUpdates := map[string]any{
		"billing_state": ledger.State, "billing_version": ledger.Version,
	}
	if desired == BillingLedgerDesiredSettle {
		midjourneyUpdates["quota"] = targetQuota
	}
	if err := tx.Model(&Midjourney{}).Where("billing_ledger_id = ?", ledger.ID).Updates(midjourneyUpdates).Error; err != nil {
		return false, err
	}
	if err := enqueueBillingOutboxTx(tx, ledger, ledger.State); err != nil {
		return false, err
	}
	return true, nil
}

func UpdateTaskWithBilling(task *Task, fromStatus TaskStatus, desired string, targetQuota int64, reason string) (bool, *BillingLedger, error) {
	if task == nil || task.ID <= 0 {
		return false, nil, errors.New("task is invalid")
	}
	var result *BillingLedger
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.Status != fromStatus {
			return nil
		}
		if task.BillingLedgerID == 0 {
			task.BillingLedgerID = current.BillingLedgerID
		}
		updateTask := func(tx *gorm.DB, ledger *BillingLedger) error {
			if ledger != nil {
				task.BillingLedgerID = ledger.ID
				task.BillingState = desired
				task.BillingVersion = ledger.Version + 1
				task.Quota = int(targetQuota)
			}
			res := tx.Model(&Task{}).Where("id = ? AND status = ?", task.ID, fromStatus).Select("*").Updates(task)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("task billing status compare-and-swap lost")
			}
			return nil
		}
		if task.BillingLedgerID == 0 {
			if err := updateTask(tx, nil); err != nil {
				return err
			}
			won = true
			return nil
		}
		ledger, err := lockBillingLedgerTx(tx, task.BillingLedgerID)
		if err != nil {
			return err
		}
		terminalLedgerMatches := desired == BillingLedgerDesiredSettle && ledger.State == BillingLedgerStateSettled && ledger.AppliedQuota == targetQuota
		terminalLedgerMatches = terminalLedgerMatches || desired == BillingLedgerDesiredRefund && ledger.State == BillingLedgerStateRefunded
		if terminalLedgerMatches {
			task.BillingLedgerID = ledger.ID
			task.BillingState = ledger.State
			task.BillingVersion = ledger.Version
			if desired == BillingLedgerDesiredSettle {
				task.Quota = int(targetQuota)
			}
			res := tx.Model(&Task{}).Where("id = ? AND status = ?", task.ID, fromStatus).Select("*").Updates(task)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("task billing status compare-and-swap lost")
			}
			copy := *ledger
			result = &copy
			won = true
			return nil
		}
		changed, err := mutateLockedBillingLedgerTx(tx, ledger, desired, targetQuota, func(tx *gorm.DB, ledger *BillingLedger) error {
			if desired == BillingLedgerDesiredRefund && reason != "" {
				ledger.LastError = truncateBillingError(reason)
			}
			return updateTask(tx, ledger)
		})
		if err != nil {
			return err
		}
		if !changed {
			return errors.New("task billing ledger transition was already completed")
		}
		copy := *ledger
		result = &copy
		won = true
		return nil
	})
	return won, result, err
}

func UpdateMidjourneyWithBilling(task *Midjourney, fromStatus string, desired string, targetQuota int64, reason string) (bool, *BillingLedger, error) {
	if task == nil || task.Id <= 0 {
		return false, nil, errors.New("midjourney task is invalid")
	}
	var result *BillingLedger
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Midjourney
		if err := lockForUpdate(tx).First(&current, task.Id).Error; err != nil {
			return err
		}
		if current.Status != fromStatus {
			return nil
		}
		if task.BillingLedgerID == 0 {
			task.BillingLedgerID = current.BillingLedgerID
		}
		updateTask := func(tx *gorm.DB, ledger *BillingLedger) error {
			if ledger != nil {
				task.BillingLedgerID = ledger.ID
				task.BillingState = ledger.State
				task.BillingVersion = ledger.Version + 1
				if desired == BillingLedgerDesiredSettle {
					task.Quota = int(targetQuota)
				}
			}
			res := tx.Model(&Midjourney{}).Where("id = ? AND status = ?", task.Id, fromStatus).Select("*").Updates(task)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("midjourney billing status compare-and-swap lost")
			}
			return nil
		}
		if task.BillingLedgerID == 0 {
			if err := updateTask(tx, nil); err != nil {
				return err
			}
			won = true
			return nil
		}
		ledger, err := lockBillingLedgerTx(tx, task.BillingLedgerID)
		if err != nil {
			return err
		}
		terminalLedgerMatches := desired == BillingLedgerDesiredSettle && ledger.State == BillingLedgerStateSettled && ledger.AppliedQuota == targetQuota
		terminalLedgerMatches = terminalLedgerMatches || desired == BillingLedgerDesiredRefund && ledger.State == BillingLedgerStateRefunded
		if terminalLedgerMatches {
			task.BillingLedgerID = ledger.ID
			task.BillingState = ledger.State
			task.BillingVersion = ledger.Version
			if desired == BillingLedgerDesiredSettle {
				task.Quota = int(targetQuota)
			}
			if err := updateTask(tx, nil); err != nil {
				return err
			}
			copy := *ledger
			result = &copy
			won = true
			return nil
		}
		changed, err := mutateLockedBillingLedgerTx(tx, ledger, desired, targetQuota, func(tx *gorm.DB, ledger *BillingLedger) error {
			if desired == BillingLedgerDesiredRefund && reason != "" {
				ledger.LastError = truncateBillingError(reason)
			}
			return updateTask(tx, ledger)
		})
		if err != nil {
			return err
		}
		if !changed {
			return errors.New("midjourney billing ledger transition was already completed")
		}
		copy := *ledger
		result = &copy
		won = true
		return nil
	})
	return won, result, err
}

func MarkBillingLedgerForReconcile(id uint64, desired string, quota int64, cause error) error {
	if id == 0 {
		return nil
	}
	now := time.Now().Unix()
	message := ""
	if cause != nil {
		message = truncateBillingError(cause.Error())
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		ledger, err := lockBillingLedgerTx(tx, id)
		if err != nil {
			return err
		}
		if ledger.State == BillingLedgerStateRefunded || (ledger.State == BillingLedgerStateSettled && desired == BillingLedgerDesiredSettle && ledger.AppliedQuota == quota) {
			return nil
		}
		ledger.State = BillingLedgerStateReconcileRequired
		ledger.DesiredState = desired
		ledger.DesiredQuota = quota
		ledger.LastError = message
		ledger.NextRetryAt = now + billingRetryDelay(ledger.Attempts)
		ledger.UpdatedAt = now
		ledger.Version++
		if err := tx.Save(ledger).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"billing_state": BillingLedgerStateReconcileRequired, "billing_version": ledger.Version,
		}
		if err := tx.Model(&Task{}).Where("billing_ledger_id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&Midjourney{}).Where("billing_ledger_id = ?", id).Updates(updates).Error
	})
}

func ListBillingLedgersForReconcile(limit int) ([]BillingLedger, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	now := time.Now().Unix()
	var ledgers []BillingLedger
	err := DB.Where("state = ? AND next_retry_at <= ?", BillingLedgerStateReconcileRequired, now).
		Order("next_retry_at asc, id asc").Limit(limit).Find(&ledgers).Error
	return ledgers, err
}

func ListStaleReservedTaskLedgers(cutoff int64, limit int) ([]BillingLedger, error) {
	return ListStaleReservedBillingLedgers("task", cutoff, limit)
}

func ListStaleReservedBillingLedgers(kind string, cutoff int64, limit int) ([]BillingLedger, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var ledgers []BillingLedger
	err := DB.Where("kind = ? AND state = ? AND created_at <= ?", kind, BillingLedgerStateReserved, cutoff).
		Order("created_at asc, id asc").Limit(limit).Find(&ledgers).Error
	return ledgers, err
}

func lockBillingLedgerTx(tx *gorm.DB, id uint64) (*BillingLedger, error) {
	if id == 0 {
		return nil, errors.New("billing ledger id is empty")
	}
	var ledger BillingLedger
	if err := lockForUpdate(tx).First(&ledger, id).Error; err != nil {
		return nil, err
	}
	return &ledger, nil
}

func adjustLedgerBalancesTx(tx *gorm.DB, ledger *BillingLedger, delta int64) error {
	if delta == 0 {
		return nil
	}
	if ledger.FundingSource == "subscription" {
		if err := adjustSubscriptionQuotaTx(tx, ledger.SubscriptionID, delta); err != nil {
			return err
		}
	} else if err := adjustWalletQuotaTx(tx, ledger.UserID, delta); err != nil {
		return err
	}
	return adjustTokenQuotaTx(tx, ledger, delta)
}

func updateBillingCountersTx(tx *gorm.DB, ledger *BillingLedger, targetQuota int64) error {
	delta := targetQuota - ledger.CountedQuota
	countDelta := 0
	if !ledger.RequestCounted && targetQuota > 0 {
		countDelta = 1
	}
	if delta != 0 || countDelta != 0 {
		query := tx.Model(&User{}).Where("id = ?", ledger.UserID)
		if delta < 0 {
			query = query.Where("used_quota >= ?", -delta)
		}
		res := query.Updates(map[string]any{
			"used_quota":    gorm.Expr("used_quota + ?", delta),
			"request_count": gorm.Expr("request_count + ?", countDelta),
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("user billing counter invariant failed")
		}
	}
	if ledger.ChannelID > 0 && delta != 0 {
		query := tx.Model(&Channel{}).Where("id = ?", ledger.ChannelID)
		if delta < 0 {
			query = query.Where("used_quota >= ?", -delta)
		}
		res := query.Update("used_quota", gorm.Expr("used_quota + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("channel billing counter invariant failed")
		}
	}
	ledger.CountedQuota = targetQuota
	ledger.RequestCounted = ledger.RequestCounted || countDelta == 1
	return nil
}

func adjustWalletQuotaTx(tx *gorm.DB, userID int, delta int64) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		res := tx.Model(&User{}).Where("id = ? AND quota >= ?", userID, delta).
			Update("quota", gorm.Expr("quota - ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrInsufficientQuota
		}
		return nil
	}
	return tx.Model(&User{}).Where("id = ?", userID).
		Update("quota", gorm.Expr("quota + ?", -delta)).Error
}

func adjustTokenQuotaTx(tx *gorm.DB, ledger *BillingLedger, delta int64) error {
	if delta == 0 || ledger.Playground || ledger.TokenID <= 0 {
		return nil
	}
	now := time.Now().Unix()
	if delta > 0 {
		res := tx.Model(&Token{}).Where("id = ? AND (unlimited_quota = ? OR remain_quota >= ?)", ledger.TokenID, true, delta).Updates(map[string]any{
			"remain_quota": gorm.Expr("CASE WHEN unlimited_quota = ? THEN remain_quota ELSE remain_quota - ? END", true, delta),
			"used_quota":   gorm.Expr("used_quota + ?", delta), "accessed_time": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrInsufficientQuota
		}
		return nil
	}
	amount := -delta
	res := tx.Model(&Token{}).Where("id = ? AND used_quota >= ?", ledger.TokenID, amount).Updates(map[string]any{
		"remain_quota": gorm.Expr("CASE WHEN unlimited_quota = ? THEN remain_quota ELSE remain_quota + ? END", true, amount),
		"used_quota":   gorm.Expr("used_quota - ?", amount), "accessed_time": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("token quota refund invariant failed")
	}
	return nil
}

func reserveSubscriptionQuotaTx(tx *gorm.DB, requestID string, userID int, amount int64) (*UserSubscription, error) {
	if amount <= 0 {
		return nil, errors.New("subscription reservation must be positive")
	}
	now := time.Now().Unix()
	var subs []UserSubscription
	if err := lockForUpdate(tx).Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", now).
		Order("end_time asc, id asc").Find(&subs).Error; err != nil {
		return nil, err
	}
	for i := range subs {
		sub := subs[i]
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, err
		}
		if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
			return nil, err
		}
		if sub.AmountTotal > 0 && sub.AmountUsed+amount > sub.AmountTotal {
			continue
		}
		record := SubscriptionPreConsumeRecord{RequestId: requestID, UserId: userID, UserSubscriptionId: sub.Id, PreConsumed: amount, Status: "consumed"}
		if err := tx.Create(&record).Error; err != nil {
			return nil, err
		}
		sub.AmountUsed += amount
		if err := tx.Save(&sub).Error; err != nil {
			return nil, err
		}
		return &sub, nil
	}
	return nil, fmt.Errorf("subscription quota insufficient, need=%d", amount)
}

func adjustSubscriptionQuotaTx(tx *gorm.DB, subscriptionID int, delta int64) error {
	if subscriptionID <= 0 {
		return errors.New("subscription id is missing")
	}
	if delta == 0 {
		return nil
	}
	var sub UserSubscription
	if err := lockForUpdate(tx).First(&sub, subscriptionID).Error; err != nil {
		return err
	}
	next := sub.AmountUsed + delta
	if next < 0 {
		return errors.New("subscription refund invariant failed")
	}
	if sub.AmountTotal > 0 && next > sub.AmountTotal {
		return fmt.Errorf("subscription used exceeds total, used=%d total=%d", next, sub.AmountTotal)
	}
	return tx.Model(&UserSubscription{}).Where("id = ?", subscriptionID).Update("amount_used", next).Error
}

func billingRetryDelay(attempt int) int64 {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return int64(1 << attempt)
}

func truncateBillingError(message string) string {
	const max = 1024
	if len(message) <= max {
		return message
	}
	return message[:max]
}
