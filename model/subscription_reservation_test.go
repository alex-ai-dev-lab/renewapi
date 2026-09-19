package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createSubscriptionReservationFixture(t *testing.T, total, used, reserved int64) (UserSubscription, SubscriptionPreConsumeRecord) {
	t.Helper()
	fixture := setupBillingLedgerTest(t)
	now := time.Now().Unix()
	sub := UserSubscription{
		UserId:      fixture.user.Id,
		PlanId:      1,
		AmountTotal: total,
		AmountUsed:  used,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "test",
	}
	require.NoError(t, DB.Create(&sub).Error)
	record := SubscriptionPreConsumeRecord{
		RequestId:          "req-subscription-fallback-reserve",
		UserId:             fixture.user.Id,
		UserSubscriptionId: sub.Id,
		PreConsumed:        reserved,
		Status:             "consumed",
	}
	require.NoError(t, DB.Create(&record).Error)
	return sub, record
}

func loadSubscriptionReservation(t *testing.T, subID int, requestID string) (UserSubscription, SubscriptionPreConsumeRecord) {
	t.Helper()
	var sub UserSubscription
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&sub, subID).Error)
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	return sub, record
}

func TestAdjustSubscriptionPreConsumeTracksFallbackReserveForFullRefund(t *testing.T) {
	sub, record := createSubscriptionReservationFixture(t, 1000, 100, 100)

	require.NoError(t, AdjustSubscriptionPreConsume(record.RequestId, sub.Id, 250))
	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 350, sub.AmountUsed)
	require.EqualValues(t, 350, record.PreConsumed)

	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.Zero(t, sub.AmountUsed)
	require.Equal(t, "refunded", record.Status)

	// Refund is request-id idempotent even after fallback reservations were added.
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	sub, _ = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.Zero(t, sub.AmountUsed)
}

func TestAdjustSubscriptionPreConsumeRollbackRestoresRecordAndBalance(t *testing.T) {
	sub, record := createSubscriptionReservationFixture(t, 1000, 100, 100)

	require.NoError(t, AdjustSubscriptionPreConsume(record.RequestId, sub.Id, 250))
	require.NoError(t, AdjustSubscriptionPreConsume(record.RequestId, sub.Id, -250))

	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 100, sub.AmountUsed)
	require.EqualValues(t, 100, record.PreConsumed)
}

func TestAdjustSubscriptionPreConsumeCapacityFailureIsAtomic(t *testing.T) {
	sub, record := createSubscriptionReservationFixture(t, 300, 100, 100)

	err := AdjustSubscriptionPreConsume(record.RequestId, sub.Id, 250)
	require.ErrorContains(t, err, "exceeds total")

	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 100, sub.AmountUsed)
	require.EqualValues(t, 100, record.PreConsumed)
}

func TestAdjustSubscriptionPreConsumeRollbackCanReduceExistingOverage(t *testing.T) {
	sub, record := createSubscriptionReservationFixture(t, 200, 300, 100)

	require.NoError(t, AdjustSubscriptionPreConsume(record.RequestId, sub.Id, -50))

	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 250, sub.AmountUsed)
	require.EqualValues(t, 50, record.PreConsumed)
}

func TestRefundSubscriptionPreConsumeReservationPreservesOtherRequestOverage(t *testing.T) {
	sub, record := createSubscriptionReservationFixture(t, 200, 250, 100)

	require.NoError(t, RefundSubscriptionPreConsumeReservation(record.RequestId))
	sub, record = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 150, sub.AmountUsed)
	require.Equal(t, "refunded", record.Status)

	// The request-id status makes retries safe and does not subtract twice.
	require.NoError(t, RefundSubscriptionPreConsumeReservation(record.RequestId))
	sub, _ = loadSubscriptionReservation(t, sub.Id, record.RequestId)
	require.EqualValues(t, 150, sub.AmountUsed)
}
