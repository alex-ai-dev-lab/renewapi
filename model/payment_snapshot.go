package model

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PaymentSnapshot is embedded in existing orders. Payment confirmation and
// fulfillment are independent facts; paid_review is durable and never credits.
type PaymentSnapshot struct {
	QuoteVersion        int    `json:"quote_version" gorm:"default:0"`
	Currency            string `json:"currency" gorm:"size:8;default:''"`
	ExpectedMinor       int64  `json:"expected_minor" gorm:"default:0"`
	CreditQuota         int64  `json:"credit_quota" gorm:"default:0"`
	Quantity            int64  `json:"quantity" gorm:"default:0"`
	CheckoutSessionID   string `json:"checkout_session_id" gorm:"size:255;default:''"`
	CheckoutPriceID     string `json:"checkout_price_id" gorm:"size:255;default:''"`
	AllowPromotionCodes bool   `json:"allow_promotion_codes" gorm:"default:false"`
	PaidMinor           int64  `json:"paid_minor" gorm:"default:0"`
	PaymentState        string `json:"payment_state" gorm:"size:32;default:''"`
	FulfillmentError    string `json:"fulfillment_error" gorm:"type:text"`
}

type PaymentResult string

const (
	PaymentFulfilled        PaymentResult = "fulfilled"
	PaymentAlreadyProcessed PaymentResult = "already_processed"
	PaymentConflict         PaymentResult = "paid_review"
)

type StripeReceipt struct {
	TradeNo, SessionID, CustomerID, Currency string
	Total, Subtotal, Discount, Tax, Shipping int64
}

func CurrencyMinorAmount(amount float64, currency string) (int64, error) {
	if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, errors.New("invalid payment amount")
	}
	exponent := int32(2)
	switch strings.ToLower(currency) {
	case "jpy", "krw", "vnd", "clp", "djf", "gnf", "kmf", "mga", "pyg", "rwf", "ugx", "vuv", "xaf", "xof", "xpf", "bif":
		exponent = 0
	case "bhd", "jod", "kwd", "omr", "tnd":
		exponent = 3
	case "isk", "huf", "twd": // Stripe charge amounts use two decimals for these currencies.
	}
	value := decimal.NewFromFloat(amount).Shift(exponent).Round(0)
	if !value.IsPositive() || value.GreaterThan(decimal.NewFromInt(999999999999)) {
		return 0, errors.New("invalid payment amount")
	}
	return value.IntPart(), nil
}

func migratePaymentSnapshotsV1() error {
	// Add only the newly introduced columns. Avoid SQLite table reconstruction.
	for _, target := range []any{&TopUp{}, &SubscriptionOrder{}} {
		for _, field := range []string{"QuoteVersion", "Currency", "ExpectedMinor", "CreditQuota", "Quantity", "CheckoutSessionID", "CheckoutPriceID", "AllowPromotionCodes", "PaidMinor", "PaymentState", "FulfillmentError"} {
			if !DB.Migrator().HasColumn(target, field) {
				if err := DB.Migrator().AddColumn(target, field); err != nil {
					return err
				}
			}
		}
	}
	if !DB.Migrator().HasColumn(&SubscriptionOrder{}, "PlanSnapshot") {
		return DB.Migrator().AddColumn(&SubscriptionOrder{}, "PlanSnapshot")
	}
	return nil
}

// Only an authenticated provider terminal event can release a pending slot.
func EndStripeCheckout(tradeNo, sessionID, status string) error {
	if tradeNo == "" || sessionID == "" {
		return errors.New("Stripe terminal event missing reference")
	}
	return purchaseTransaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		err := tx.Where("trade_no = ?", tradeNo).First(&order).Error
		var target any
		var snapshot PaymentSnapshot
		if err == nil {
			if err := lockPurchaseUserTx(tx, order.UserId); err != nil {
				return err
			}
			target, snapshot = &order, order.PaymentSnapshot
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			var topUp TopUp
			if err := tx.Where("trade_no = ?", tradeNo).First(&topUp).Error; err != nil {
				return err
			}
			if topUp.PaymentProvider != PaymentProviderStripe {
				return ErrPaymentMethodMismatch
			}
			target, snapshot = &topUp, topUp.PaymentSnapshot
		} else {
			return err
		}
		if order.Id > 0 && order.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}
		if snapshot.CheckoutSessionID != "" && snapshot.CheckoutSessionID != sessionID {
			return errors.New("checkout session mismatch")
		}
		return tx.Model(target).Where("status = ? AND (payment_state = ? OR payment_state = ? OR payment_state = ?)", common.TopUpStatusPending, "", "unpaid", "checkout_unknown").Updates(map[string]any{"status": status, "complete_time": common.GetTimestamp()}).Error
	})
}

func (snapshot PaymentSnapshot) validate(receipt StripeReceipt) error {
	if snapshot.QuoteVersion == 0 {
		return errors.New("legacy order requires payment reconciliation: historical quote missing")
	}
	if receipt.SessionID == "" || snapshot.CheckoutSessionID != receipt.SessionID {
		return errors.New("checkout session mismatch or not yet bound")
	}
	if !strings.EqualFold(snapshot.Currency, receipt.Currency) {
		return errors.New("payment currency mismatch")
	}
	if receipt.Subtotal != snapshot.ExpectedMinor {
		return errors.New("checkout subtotal mismatch")
	}
	if receipt.Shipping != 0 {
		return errors.New("unsupported checkout shipping charge")
	}
	if receipt.Total < 0 || receipt.Discount < 0 || receipt.Discount > receipt.Subtotal || receipt.Tax < 0 {
		return errors.New("invalid paid amount")
	}
	if receipt.Discount > 0 && !snapshot.AllowPromotionCodes {
		return errors.New("promotion not allowed for this order")
	}
	if receipt.Total != receipt.Subtotal-receipt.Discount+receipt.Tax {
		return errors.New("checkout total mismatch")
	}
	return nil
}

// lockPurchaseUserTx takes a write lock before reading purchase counts. A real
// write is required on SQLite, where SELECT FOR UPDATE has no effect.
func lockPurchaseUserTx(tx *gorm.DB, userID int) error {
	if tx.Dialector.Name() == "sqlite" {
		res := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("quota", gorm.Expr("quota"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	var user User
	return lockForUpdate(tx).Select("id").First(&user, userID).Error
}

// Retry only transactions known to have rolled back; never retry external IO.
func purchaseTransaction(fn func(*gorm.DB) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = DB.Transaction(fn)
		if err == nil {
			return nil
		}
		message := strings.ToLower(err.Error())
		if !(strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") || strings.Contains(message, "deadlock") || strings.Contains(message, "serialization failure")) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}

func checkPurchaseCapacityTx(tx *gorm.DB, userID int, plan *SubscriptionPlan, excludeOrderID int) error {
	if plan.MaxPurchasePerUser <= 0 {
		return nil
	}
	var purchased, pending int64
	if err := tx.Model(&UserSubscription{}).Where("user_id = ? AND plan_id = ?", userID, plan.Id).Count(&purchased).Error; err != nil {
		return err
	}
	if err := tx.Model(&SubscriptionOrder{}).Where("user_id = ? AND plan_id = ? AND status = ? AND id <> ?", userID, plan.Id, common.TopUpStatusPending, excludeOrderID).Count(&pending).Error; err != nil {
		return err
	}
	if purchased+pending >= int64(plan.MaxPurchasePerUser) {
		return ErrSubscriptionPurchaseLimit
	}
	return nil
}

var ErrSubscriptionPurchaseLimit = errors.New("已达到该套餐购买上限（含待支付订单）")

func BindStripeCheckout(tradeNo, sessionID string) error {
	var order SubscriptionOrder
	var target any
	err := DB.Where("trade_no = ?", tradeNo).First(&order).Error
	if err == nil {
		target = &order
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		var topUp TopUp
		if err := DB.Where("trade_no = ?", tradeNo).First(&topUp).Error; err != nil {
			return err
		}
		target = &topUp
	} else {
		return err
	}
	res := DB.Model(target).Where("payment_provider = ? AND (checkout_session_id = ? OR checkout_session_id = ?)", PaymentProviderStripe, "", sessionID).Update("checkout_session_id", sessionID)
	if res.Error != nil {
		return res.Error
	}
	// MySQL can report zero changed rows when the binding already matches.
	if res.RowsAffected == 0 {
		var count int64
		if err := DB.Model(target).Where("payment_provider = ? AND checkout_session_id = ?", PaymentProviderStripe, sessionID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return errors.New("checkout binding conflict")
		}
	}
	return nil
}

// CompleteStripeCheckout is the only Stripe paid-event fulfillment path.
// A committed conflict is ACK-able; any database failure must be retried.
func CompleteStripeCheckout(receipt StripeReceipt) (PaymentResult, error) {
	var seed SubscriptionOrder
	lookupErr := DB.Where("trade_no = ?", receipt.TradeNo).First(&seed).Error
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return "", lookupErr
	}
	result := PaymentFulfilled
	var userID, tokenID int
	err := purchaseTransaction(func(tx *gorm.DB) error {
		result = PaymentFulfilled
		var order SubscriptionOrder
		if lookupErr == nil {
			if err := lockPurchaseUserTx(tx, seed.UserId); err != nil {
				return err
			}
			if err := lockForUpdate(tx).First(&order, seed.Id).Error; err != nil {
				return err
			}
			if order.PaymentProvider != PaymentProviderStripe {
				return ErrPaymentMethodMismatch
			}
			if order.Status == common.TopUpStatusSuccess {
				result = PaymentAlreadyProcessed
				return nil
			}
			// Session binding may race the first webhook. Leave this retryable.
			if order.QuoteVersion > 0 && order.CheckoutSessionID == "" {
				return errors.New("checkout session binding is pending")
			}
			conflict := order.PaymentSnapshot.validate(receipt)
			var plan SubscriptionPlan
			if conflict == nil {
				if err := common.UnmarshalJsonStr(order.PlanSnapshot, &plan); err != nil {
					conflict = errors.New("subscription snapshot invalid")
				}
			}
			if conflict == nil {
				if err := checkPurchaseCapacityTx(tx, order.UserId, &plan, order.Id); err != nil {
					if !errors.Is(err, ErrSubscriptionPurchaseLimit) {
						return err
					}
					conflict = err
				}
			}
			order.PaidMinor, order.PaymentState = receipt.Total, "paid"
			if conflict != nil {
				order.PaymentState, order.FulfillmentError = string(PaymentConflict), conflict.Error()
				result = PaymentConflict
				return tx.Save(&order).Error
			}
			if _, err := createUserSubscriptionFromPlanTx(tx, order.UserId, &plan, "order"); err != nil {
				return err
			}
			if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
				return err
			}
			order.Status, order.CompleteTime, order.FulfillmentError = common.TopUpStatusSuccess, common.GetTimestamp(), ""
			userID = order.UserId
			return tx.Save(&order).Error
		}
		var topUp TopUp
		if err := lockForUpdate(tx).Where("trade_no = ?", receipt.TradeNo).First(&topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			result = PaymentAlreadyProcessed
			return nil
		}
		if topUp.QuoteVersion > 0 && topUp.CheckoutSessionID == "" {
			return errors.New("checkout session binding is pending")
		}
		conflict := topUp.PaymentSnapshot.validate(receipt)
		topUp.PaidMinor, topUp.PaymentState = receipt.Total, "paid"
		if conflict == nil && topUp.CreditQuota <= 0 {
			conflict = errors.New("invalid credit snapshot")
		}
		if conflict != nil {
			topUp.PaymentState, topUp.FulfillmentError = string(PaymentConflict), conflict.Error()
			result = PaymentConflict
			return tx.Save(&topUp).Error
		}
		// Conditional transition is the idempotency boundary on every database.
		res := tx.Model(&TopUp{}).Where("id = ? AND status <> ?", topUp.Id, common.TopUpStatusSuccess).Updates(map[string]any{
			"status": common.TopUpStatusSuccess, "complete_time": common.GetTimestamp(),
			"paid_minor": receipt.Total, "payment_state": "paid", "fulfillment_error": "",
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("payment transition conflict")
		}
		res = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(map[string]any{
			"quota": gorm.Expr("quota + ?", topUp.CreditQuota), "stripe_customer": receipt.CustomerID,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("payment user missing: %d", topUp.UserId)
		}
		userID = topUp.UserId
		return nil
	})
	if err == nil && userID > 0 {
		InvalidateBillingBalanceCaches(userID, tokenID)
		_ = InvalidateUserCache(userID)
	}
	return result, err
}
