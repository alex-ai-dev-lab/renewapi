package model

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// LegacyBillingSettlement describes the balance delta applied after an upstream
// response has already consumed resources in shadow/off billing modes.
type LegacyBillingSettlement struct {
	FundingSource  string
	UserID         int
	SubscriptionID int
	TokenID        int
	Delta          int64
	Playground     bool
}

// SettleLegacyBillingBalances applies the funding and token deltas in one DB
// transaction. Post-usage settlement must record the already-consumed usage
// even when it exceeds the remaining finite quota. Wallet/token balances are
// exhausted at zero; subscription usage records the full overage so concurrent
// reservation refunds cannot make already-consumed quota available again.
func SettleLegacyBillingBalances(input LegacyBillingSettlement) error {
	if input.Delta == 0 {
		return nil
	}
	if input.UserID <= 0 {
		return errors.New("legacy billing user id is invalid")
	}
	if input.FundingSource != "wallet" && input.FundingSource != "subscription" {
		return fmt.Errorf("unsupported billing source %q", input.FundingSource)
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if input.FundingSource == "subscription" {
			if err := adjustSettledSubscriptionQuotaTx(tx, input.SubscriptionID, input.Delta); err != nil {
				return err
			}
		} else if err := adjustSettledWalletQuotaTx(tx, input.UserID, input.Delta); err != nil {
			return err
		}

		if input.Playground || input.TokenID <= 0 {
			return nil
		}
		return adjustSettledTokenQuotaTx(tx, input.TokenID, input.Delta)
	})
	if err != nil {
		return err
	}
	InvalidateBillingBalanceCaches(input.UserID, input.TokenID)
	return nil
}

func adjustSettledWalletQuotaTx(tx *gorm.DB, userID int, delta int64) error {
	var user User
	if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if delta > 0 {
		return tx.Model(&User{}).Where("id = ?", userID).Update(
			"quota",
			gorm.Expr("CASE WHEN quota >= ? THEN quota - ? ELSE 0 END", delta, delta),
		).Error
	}
	return tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", -delta)).Error
}

func adjustSettledSubscriptionQuotaTx(tx *gorm.DB, subscriptionID int, delta int64) error {
	if subscriptionID <= 0 {
		return errors.New("subscription id is missing")
	}
	var sub UserSubscription
	if err := lockForUpdate(tx).First(&sub, subscriptionID).Error; err != nil {
		return err
	}
	next := sub.AmountUsed + delta
	if next < 0 {
		return errors.New("subscription post-usage refund invariant failed")
	}
	return tx.Model(&UserSubscription{}).Where("id = ?", subscriptionID).Update("amount_used", next).Error
}

func adjustSettledTokenQuotaTx(tx *gorm.DB, tokenID int, delta int64) error {
	if delta == 0 {
		return nil
	}
	now := time.Now().Unix()
	if delta > 0 {
		res := tx.Model(&Token{}).Where("id = ?", tokenID).Updates(map[string]any{
			"remain_quota": gorm.Expr(
				"CASE WHEN unlimited_quota = ? THEN remain_quota WHEN remain_quota >= ? THEN remain_quota - ? ELSE 0 END",
				true, delta, delta,
			),
			"used_quota":    gorm.Expr("used_quota + ?", delta),
			"accessed_time": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("token post-usage settlement invariant failed")
		}
		return nil
	}

	amount := -delta
	res := tx.Model(&Token{}).Where("id = ? AND used_quota >= ?", tokenID, amount).Updates(map[string]any{
		"remain_quota": gorm.Expr("CASE WHEN unlimited_quota = ? THEN remain_quota ELSE remain_quota + ? END", true, amount),
		"used_quota":   gorm.Expr("used_quota - ?", amount),
		"accessed_time": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("token post-usage refund invariant failed")
	}
	return nil
}
