package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// LegacyBillingRefund 保留两侧实际预扣额，兼容补充预扣只完成资金侧的旧会话。
type LegacyBillingRefund struct {
	FundingSource string
	RequestID     string
	UserID        int
	TokenID       int
	FundingQuota  int64
	TokenQuota    int64
	Playground    bool
}

// RefundLegacyBillingBalances 由 BillingSession 的终态锁保证请求内幂等。
// off 没有持久化请求账本；需要跨进程恢复时必须使用 shadow 或 enforce。
func RefundLegacyBillingBalances(input LegacyBillingRefund) error {
	if input.UserID <= 0 || input.FundingQuota < 0 || input.TokenQuota < 0 {
		return errors.New("legacy billing refund is invalid")
	}
	if input.FundingSource != "wallet" && input.FundingSource != "subscription" {
		return fmt.Errorf("unsupported billing source %q", input.FundingSource)
	}
	if input.FundingSource == "subscription" && strings.TrimSpace(input.RequestID) == "" {
		return errors.New("subscription refund request id is empty")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if input.FundingSource == "subscription" {
			if err := refundSubscriptionPreConsumeReservationTx(tx, input.RequestID); err != nil {
				return err
			}
		} else if err := adjustWalletQuotaTx(tx, input.UserID, -input.FundingQuota); err != nil {
			return err
		}
		if input.Playground || input.TokenID <= 0 {
			return nil
		}
		return adjustSettledTokenQuotaTx(tx, input.TokenID, -input.TokenQuota)
	})
	if err == nil {
		InvalidateBillingBalanceCaches(input.UserID, input.TokenID)
	}
	return err
}
