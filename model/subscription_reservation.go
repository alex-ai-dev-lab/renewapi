package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// AdjustSubscriptionPreConsume atomically adjusts both the request-scoped
// pre-consume record and the subscription balance it represents. Fallback
// reservations must use this path so request-scoped refunds can later restore
// the complete reservation by requestId.
func AdjustSubscriptionPreConsume(requestId string, userSubscriptionId int, delta int64) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).
			Where("request_id = ?", requestId).
			First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return errors.New("subscription pre-consume already refunded")
		}
		if record.Status != "consumed" {
			return fmt.Errorf("invalid subscription pre-consume status: %s", record.Status)
		}
		if record.UserSubscriptionId != userSubscriptionId {
			return fmt.Errorf("subscription pre-consume subscription mismatch: record=%d requested=%d", record.UserSubscriptionId, userSubscriptionId)
		}

		newPreConsumed := record.PreConsumed + delta
		if newPreConsumed < 0 {
			return fmt.Errorf("subscription pre-consume would become negative: current=%d delta=%d", record.PreConsumed, delta)
		}

		var sub UserSubscription
		if err := lockForUpdate(tx).
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		newUsed := sub.AmountUsed + delta
		if newUsed < 0 {
			return fmt.Errorf("subscription used would become negative: current=%d delta=%d", sub.AmountUsed, delta)
		}
		if delta > 0 && sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
			return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
		}

		sub.AmountUsed = newUsed
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		record.PreConsumed = newPreConsumed
		return tx.Save(&record).Error
	})
}

// RefundSubscriptionPreConsumeReservation is the request-scoped legacy refund
// path used by BillingSession. Post-usage settlement may legitimately leave a
// subscription temporarily over its plan total; refunding another concurrent
// request must still subtract that request's reservation instead of failing
// merely because the aggregate remains above the plan limit.
func RefundSubscriptionPreConsumeReservation(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).
			Where("request_id = ?", requestId).
			First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.Status != "consumed" {
			return fmt.Errorf("invalid subscription pre-consume status: %s", record.Status)
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		if record.UserSubscriptionId <= 0 {
			return errors.New("subscription pre-consume subscription id is missing")
		}

		var sub UserSubscription
		if err := lockForUpdate(tx).
			Where("id = ?", record.UserSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		next := sub.AmountUsed - record.PreConsumed
		if next < 0 {
			return fmt.Errorf("subscription refund would become negative: used=%d refund=%d", sub.AmountUsed, record.PreConsumed)
		}
		sub.AmountUsed = next
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}
