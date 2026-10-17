package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

type legacyAuditTopUp struct {
	Id      int `gorm:"primaryKey"`
	TradeNo string
	Money   float64
	Status  string
}

func (legacyAuditTopUp) TableName() string { return "top_ups" }

type legacyAuditSubscriptionOrder struct {
	Id      int `gorm:"primaryKey"`
	TradeNo string
	Status  string
}

func (legacyAuditSubscriptionOrder) TableName() string { return "subscription_orders" }

func TestPaymentSnapshotMigrationPreservesLegacyRowsAndIsRepeatable(t *testing.T) {
	paymentFixture(t)
	require.NoError(t, DB.Migrator().DropTable(&TopUp{}, &SubscriptionOrder{}))
	require.NoError(t, DB.AutoMigrate(&legacyAuditTopUp{}, &legacyAuditSubscriptionOrder{}))
	require.NoError(t, DB.Create(&legacyAuditTopUp{Id: 71, TradeNo: "old-topup", Money: 12.34, Status: common.TopUpStatusPending}).Error)
	require.NoError(t, DB.Create(&legacyAuditSubscriptionOrder{Id: 72, TradeNo: "old-sub", Status: common.TopUpStatusPending}).Error)
	require.NoError(t, migratePaymentSnapshotsV1())
	require.NoError(t, migratePaymentSnapshotsV1())
	var topup TopUp
	require.NoError(t, DB.First(&topup, 71).Error)
	require.Equal(t, "old-topup", topup.TradeNo)
	require.Equal(t, 12.34, topup.Money)
	require.Equal(t, common.TopUpStatusPending, topup.Status)
	require.Zero(t, topup.QuoteVersion)
	require.Empty(t, topup.CheckoutSessionID)
	var order SubscriptionOrder
	require.NoError(t, DB.First(&order, 72).Error)
	require.Equal(t, "old-sub", order.TradeNo)
	require.Empty(t, order.PlanSnapshot)
	require.Zero(t, order.QuoteVersion)
}
