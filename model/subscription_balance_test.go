package model

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSubscriptionBalancePurchaseUsesTransactionClock(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPlan{}, &SubscriptionOrder{}))
	require.NoError(t, LOG_DB.AutoMigrate(&Log{}))
	planDB := DB
	t.Cleanup(func() {
		if !common.UsingSQLite {
			_ = planDB.Migrator().DropTable(&SubscriptionOrder{}, &SubscriptionPlan{})
		}
	})
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	// A deadline makes the pre-fix nested-connection wait fail deterministically.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	DB = DB.WithContext(ctx)
	initialQuota := int(common.QuotaPerUnit) * 2
	require.NoError(t, DB.Model(&User{}).Where("id = ?", fixture.user.Id).Update("quota", initialQuota).Error)
	plan := SubscriptionPlan{Title: "Transaction clock", Enabled: true, PriceAmount: 1, Currency: "USD", DurationUnit: SubscriptionDurationDay, DurationValue: 30, QuotaResetPeriod: SubscriptionResetNever, MaxPurchasePerUser: 1, TotalAmount: 1000}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	require.NoError(t, PurchaseSubscriptionWithBalance(fixture.user.Id, plan.Id))
	var user User
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, initialQuota-int(common.QuotaPerUnit), user.Quota)
	var subscriptions, orders int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&subscriptions).Error)
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&orders).Error)
	require.EqualValues(t, 1, subscriptions)
	require.EqualValues(t, 1, orders)
	require.Error(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id))
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Equal(t, initialQuota-int(common.QuotaPerUnit), user.Quota, "purchase-limit failure must roll back the debit")
}
