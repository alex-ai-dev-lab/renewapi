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
// transaction. Post-usage token settlement must record the complete actual
// usage even when it exceeds the token's remaining quota; limited tokens are
// exhausted at zero instead of leaving stale positive quota that could be used
// again after the already-delivered response.
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
			if err := adjustSubscriptionQuotaTx(tx, input.SubscriptionID, input.Delta); err != nil {
				return err
			}
		} else if err := adjustWalletQuotaTx(tx, input.UserID, input.Delta); err != nil {
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
