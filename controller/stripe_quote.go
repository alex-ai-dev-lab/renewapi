package controller

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func stripeTopUpQuote(amount int64, group string) (model.PaymentSnapshot, error) {
	if amount < getStripeMinTopup() || common.QuotaPerUnit <= 0 {
		return model.PaymentSnapshot{}, errors.New("invalid topup quantity")
	}
	money := getStripePayMoney(float64(amount), group)
	if math.IsNaN(money) || math.IsInf(money, 0) {
		return model.PaymentSnapshot{}, errors.New("invalid Stripe price")
	}
	minor, err := model.CurrencyMinorAmount(money, "usd")
	if err != nil {
		return model.PaymentSnapshot{}, err
	}
	units := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		units = units.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	if units.GreaterThan(decimal.NewFromInt(10000)) {
		return model.PaymentSnapshot{}, errors.New("topup quantity exceeds limit")
	}
	ratio := common.GetTopupGroupRatio(group)
	if ratio == 0 {
		ratio = 1
	}
	quota := units.Mul(decimal.NewFromFloat(ratio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0).IntPart()
	if quota <= 0 {
		return model.PaymentSnapshot{}, errors.New("invalid topup quota")
	}
	return model.PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: minor,
		CreditQuota: quota, Quantity: amount, CheckoutPriceID: setting.StripePriceId,
		AllowPromotionCodes: setting.StripePromotionCodesEnabled, PaymentState: "unpaid"}, nil
}

func stripeQuoteFingerprint(quote model.PaymentSnapshot) string {
	return common.Sha1([]byte(fmt.Sprintf("%d:%s:%d:%d:%d:%s:%t", quote.QuoteVersion, quote.Currency, quote.ExpectedMinor,
		quote.CreditQuota, quote.Quantity, quote.CheckoutPriceID, quote.AllowPromotionCodes)))
}
