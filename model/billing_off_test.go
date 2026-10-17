package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOffReservationSecondLegFailureLeavesNoDebitOrLedger(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	_, err := ReserveBillingLedger(BillingReservation{Mode: "off", RequestID: "off-atomic", UserID: fixture.user.Id, TokenID: 999999, FundingSource: "wallet", Quota: 50, ApplyBalances: true})
	require.Error(t, err)
	var user User
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, fixture.user.Quota, user.Quota)
	var count int64
	require.NoError(t, DB.Model(&BillingLedger{}).Where("request_id = ?", "off-atomic").Count(&count).Error)
	require.Zero(t, count)
}

func TestOffSubscriptionReservationAndAdditionalFailureAreAtomic(t *testing.T) {
	fixture := paymentFixture(t)
	plan := SubscriptionPlan{Title: "off-initial", Enabled: true, DurationUnit: SubscriptionDurationDay, DurationValue: 1}
	require.NoError(t, DB.Create(&plan).Error)
	sub := UserSubscription{UserId: fixture.user.Id, PlanId: plan.Id, Status: "active", AmountTotal: 1000, EndTime: time.Now().Unix() + 3600}
	require.NoError(t, DB.Create(&sub).Error)
	input := BillingReservation{Mode: "off", RequestID: "off-initial", UserID: fixture.user.Id, TokenID: 999999, FundingSource: "subscription", Quota: 50, ApplyBalances: true}
	_, err := ReserveBillingLedger(input)
	require.Error(t, err)
	require.NoError(t, DB.First(&sub, sub.Id).Error)
	require.Zero(t, sub.AmountUsed)
	var count int64
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", input.RequestID).Count(&count).Error)
	require.Zero(t, count)
	input.TokenID = fixture.token.Id
	reservation, err := ReserveBillingLedger(input)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Update("remain_quota", 0).Error)
	_, err = ReserveMoreBillingLedger(reservation.Ledger.ID, 100)
	require.Error(t, err)
	latest, record := loadSubscriptionReservation(t, sub.Id, input.RequestID)
	require.EqualValues(t, 50, latest.AmountUsed)
	require.EqualValues(t, 50, record.PreConsumed)
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.EqualValues(t, 50, ledger.AppliedQuota)
	require.Equal(t, BillingLedgerStateReserved, ledger.State)
}
