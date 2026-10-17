package model

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func paymentFixture(t *testing.T) billingLedgerFixture {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}))
	t.Cleanup(func() {
		if DB.Dialector.Name() != "sqlite" {
			_ = DB.Migrator().DropTable(&TopUp{}, &SubscriptionOrder{}, &SubscriptionPlan{})
		}
	})
	return fixture
}

func TestStripeSnapshotDiscountAndDuplicate(t *testing.T) {
	fixture := paymentFixture(t)
	initial := fixture.user.Quota
	order := TopUp{UserId: fixture.user.Id, TradeNo: "coupon", Money: 80, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe,
		PaymentSnapshot: PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 8000, CreditQuota: 500, CheckoutSessionID: "cs_coupon", AllowPromotionCodes: true}}
	require.NoError(t, order.Insert())
	receipt := StripeReceipt{TradeNo: order.TradeNo, SessionID: "cs_coupon", Currency: "usd", Subtotal: 8000, Discount: 1600, Total: 6400}
	result, err := CompleteStripeCheckout(receipt)
	require.NoError(t, err)
	require.Equal(t, PaymentFulfilled, result)
	result, err = CompleteStripeCheckout(receipt)
	require.NoError(t, err)
	require.Equal(t, PaymentAlreadyProcessed, result)
	var user User
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, initial+500, user.Quota)
}

func TestStripeSnapshotConflictsAreDurable(t *testing.T) {
	for _, name := range []string{"currency", "session", "underpaid", "promotion", "shipping", "legacy"} {
		t.Run(name, func(t *testing.T) {
			fixture := paymentFixture(t)
			order := TopUp{UserId: fixture.user.Id, TradeNo: name, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe,
				PaymentSnapshot: PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 100, CreditQuota: 500, CheckoutSessionID: "cs_test"}}
			receipt := StripeReceipt{TradeNo: name, SessionID: "cs_test", Currency: "usd", Subtotal: 100, Total: 100}
			switch name {
			case "shipping":
				receipt.Shipping = 10
			case "currency":
				receipt.Currency = "jpy"
			case "session":
				receipt.SessionID = "wrong"
			case "underpaid":
				receipt.Total = 50
			case "promotion":
				receipt.Discount, receipt.Total = 50, 50
			case "legacy":
				order.QuoteVersion = 0
			}
			require.NoError(t, order.Insert())
			result, err := CompleteStripeCheckout(receipt)
			require.NoError(t, err)
			require.Equal(t, PaymentConflict, result)
			require.NoError(t, DB.First(&order, order.Id).Error)
			require.Equal(t, "paid_review", order.PaymentState)
			require.NotEmpty(t, order.FulfillmentError)
			var user User
			require.NoError(t, DB.First(&user, fixture.user.Id).Error)
			require.Equal(t, fixture.user.Quota, user.Quota)
		})
	}
}

func TestStripeBindingTerminalOrderingAndManualCredit(t *testing.T) {
	fixture := paymentFixture(t)
	order := TopUp{UserId: fixture.user.Id, TradeNo: "ordering", Money: 1, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe,
		PaymentSnapshot: PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 100, CreditQuota: 200}}
	require.NoError(t, order.Insert())
	receipt := StripeReceipt{TradeNo: order.TradeNo, SessionID: "cs_test", Currency: "usd", Subtotal: 100, Total: 100}
	_, err := CompleteStripeCheckout(receipt)
	require.ErrorContains(t, err, "binding is pending")
	require.NoError(t, BindStripeCheckout(order.TradeNo, receipt.SessionID))
	require.NoError(t, BindStripeCheckout(order.TradeNo, receipt.SessionID))
	require.Error(t, BindStripeCheckout(order.TradeNo, "another_session"))
	require.NoError(t, EndStripeCheckout(order.TradeNo, receipt.SessionID, common.TopUpStatusExpired))
	result, err := CompleteStripeCheckout(receipt)
	require.NoError(t, err)
	require.Equal(t, PaymentFulfilled, result, "late paid event is verified before fulfillment")
	require.NoError(t, EndStripeCheckout(order.TradeNo, receipt.SessionID, common.TopUpStatusFailed))
	require.NoError(t, DB.First(&order, order.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, order.Status)
	require.NoError(t, ManualCompleteTopUp(order.TradeNo, ""))
	var user User
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, fixture.user.Quota+200, user.Quota)
	legacy := TopUp{UserId: fixture.user.Id, TradeNo: "legacy-manual", Money: 2, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe}
	require.NoError(t, legacy.Insert())
	require.ErrorContains(t, ManualCompleteTopUp(legacy.TradeNo, ""), "historical credit")
}

func TestStripeGenericCompletionCannotBypassReceipt(t *testing.T) {
	fixture := paymentFixture(t)
	plan := SubscriptionPlan{Title: "guard", Enabled: true, PriceAmount: 1, Currency: "USD", DurationUnit: SubscriptionDurationDay, DurationValue: 1}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{UserId: fixture.user.Id, PlanId: plan.Id, TradeNo: "guard", Money: 1, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe}
	require.NoError(t, order.Insert())
	require.ErrorContains(t, CompleteSubscriptionOrder(order.TradeNo, "", PaymentProviderStripe, ""), "verified Checkout")
	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", fixture.user.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestStripeSnapshotRollbackAndRetry(t *testing.T) {
	fixture := paymentFixture(t)
	order := TopUp{UserId: fixture.user.Id, TradeNo: "rollback", Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe,
		PaymentSnapshot: PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 100, CreditQuota: 50, CheckoutSessionID: "cs_test"}}
	require.NoError(t, order.Insert())
	receipt := StripeReceipt{TradeNo: order.TradeNo, SessionID: "cs_test", Currency: "usd", Subtotal: 100, Total: 100}
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register("test:payment-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(errors.New("injected storage failure"))
		}
	}))
	_, err := CompleteStripeCheckout(receipt)
	require.ErrorContains(t, err, "injected storage failure")
	require.NoError(t, DB.Callback().Update().Remove("test:payment-failure"))
	require.NoError(t, DB.First(&order, order.Id).Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
	_, err = CompleteStripeCheckout(receipt)
	require.NoError(t, err)
}

func TestSubscriptionPendingCapacityAndSnapshot(t *testing.T) {
	fixture := paymentFixture(t)
	plan := SubscriptionPlan{Title: "reserved", Enabled: true, PriceAmount: 10, Currency: "USD", DurationUnit: SubscriptionDurationDay, DurationValue: 30, TotalAmount: 1000, MaxPurchasePerUser: 1}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{UserId: fixture.user.Id, PlanId: plan.Id, TradeNo: "reserved", Money: 10, Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderStripe,
		PaymentSnapshot: PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 1000, CheckoutSessionID: "cs_sub"}}
	require.NoError(t, order.Insert())
	other := order
	other.Id, other.TradeNo = 0, "other"
	require.ErrorIs(t, other.Insert(), ErrSubscriptionPurchaseLimit)
	_, err := AdminBindSubscription(fixture.user.Id, plan.Id, "test")
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)
	require.Error(t, PurchaseSubscriptionWithBalance(fixture.user.Id, plan.Id))
	require.NoError(t, DB.Model(&plan).Update("total_amount", 9999).Error)
	result, err := CompleteStripeCheckout(StripeReceipt{TradeNo: order.TradeNo, SessionID: "cs_sub", Currency: "usd", Subtotal: 1000, Total: 1000})
	require.NoError(t, err)
	require.Equal(t, PaymentFulfilled, result)
	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ?", fixture.user.Id).First(&sub).Error)
	require.EqualValues(t, 1000, sub.AmountTotal, "fulfillment uses promised snapshot")
}

func TestSubscriptionConcurrentReservations(t *testing.T) {
	fixture := paymentFixture(t)
	plan := SubscriptionPlan{Title: "race", Enabled: true, PriceAmount: 1, Currency: "USD", DurationUnit: SubscriptionDurationDay, DurationValue: 1, MaxPurchasePerUser: 1}
	require.NoError(t, DB.Create(&plan).Error)
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	var wg sync.WaitGroup
	errorsCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			order := SubscriptionOrder{UserId: fixture.user.Id, PlanId: plan.Id, TradeNo: fmt.Sprintf("race-%d", i), Money: 1, Status: common.TopUpStatusPending}
			errorsCh <- order.Insert()
		}(i)
	}
	wg.Wait()
	close(errorsCh)
	success := 0
	for err := range errorsCh {
		if err == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	var count int64
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestCurrencyMinorAmount(t *testing.T) {
	for currency, expected := range map[string]int64{"USD": 1234, "JPY": 12, "KWD": 12340} {
		actual, err := CurrencyMinorAmount(12.34, currency)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}
