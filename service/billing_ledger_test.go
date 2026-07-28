package service

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBillingSessionEnforceOwnsAllBalanceWrites(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeEnforce)
	seedUser(t, 101, 1000)
	seedToken(t, 101, 101, "sk-enforce-session", 1000)
	seedChannel(t, 101)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 101, TokenId: 101, TokenKey: "sk-enforce-session", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 101},
		RequestId: "req-enforce-session", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	require.NotZero(t, session.ledgerID)
	require.True(t, session.ownsLedgerAccounting())
	require.Equal(t, 600, getUserQuota(t, 101))
	require.Equal(t, 600, getTokenRemainQuota(t, 101))
	require.Equal(t, 400, getTokenUsedQuota(t, 101))

	duplicateInfo := *info
	duplicateInfo.Billing = nil
	_, duplicateErr := NewBillingSession(ctx, &duplicateInfo, 400)
	require.NotNil(t, duplicateErr)
	require.Equal(t, 600, getUserQuota(t, 101))
	require.Equal(t, 600, getTokenRemainQuota(t, 101))

	require.NoError(t, session.Reserve(600))
	require.Equal(t, 400, getUserQuota(t, 101))
	require.Equal(t, 400, getTokenRemainQuota(t, 101))

	require.NoError(t, session.Settle(250))
	require.Equal(t, 750, getUserQuota(t, 101))
	require.Equal(t, 750, getTokenRemainQuota(t, 101))
	require.Equal(t, 250, getTokenUsedQuota(t, 101))

	var user model.User
	var channel model.Channel
	require.NoError(t, model.DB.First(&user, 101).Error)
	require.NoError(t, model.DB.First(&channel, 101).Error)
	require.Equal(t, 250, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
	require.EqualValues(t, 250, channel.UsedQuota)
}

func TestBillingReconcilerRecoversSettleAndRefund(t *testing.T) {
	truncate(t)
	seedUser(t, 102, 1000)
	seedToken(t, 102, 102, "sk-reconcile", 1000)
	seedChannel(t, 102)

	settleReservation, err := model.ReserveBillingLedger(model.BillingReservation{
		RequestID: "req-reconcile-settle", Kind: "request", Mode: BillingLedgerModeEnforce,
		FundingSource: BillingSourceWallet, UserID: 102, TokenID: 102, ChannelID: 102,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)
	require.NoError(t, model.MarkBillingLedgerForReconcile(settleReservation.Ledger.ID, model.BillingLedgerDesiredSettle, 250, errors.New("simulated process interruption")))
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", settleReservation.Ledger.ID).Update("next_retry_at", 0).Error)

	processed, err := ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	require.Positive(t, processed)
	settled, err := model.GetBillingLedger(settleReservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateSettled, settled.State)
	require.Equal(t, 750, getUserQuota(t, 102))
	require.Equal(t, 750, getTokenRemainQuota(t, 102))

	refundReservation, err := model.ReserveBillingLedger(model.BillingReservation{
		RequestID: "req-reconcile-refund", Kind: "task", Mode: BillingLedgerModeEnforce,
		FundingSource: BillingSourceWallet, UserID: 102, TokenID: 102, ChannelID: 102,
		Quota: 200, ApplyBalances: true,
	})
	require.NoError(t, err)
	require.NoError(t, model.MarkBillingLedgerForReconcile(refundReservation.Ledger.ID, model.BillingLedgerDesiredRefund, 0, errors.New("simulated refund interruption")))
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", refundReservation.Ledger.ID).Update("next_retry_at", 0).Error)

	_, err = ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	refunded, err := model.GetBillingLedger(refundReservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateRefunded, refunded.State)
	require.Equal(t, 750, getUserQuota(t, 102))
	require.Equal(t, 750, getTokenRemainQuota(t, 102))
}

func TestBillingReconcilerRefundsStalePreparedTask(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_RESERVED_TASK_TIMEOUT_SECONDS", "60")
	seedUser(t, 103, 1000)
	seedToken(t, 103, 103, "sk-stale-task", 1000)
	seedChannel(t, 103)
	reservation, err := model.ReserveBillingLedger(model.BillingReservation{
		RequestID: "req-stale-task", Kind: "request", Mode: BillingLedgerModeEnforce,
		FundingSource: BillingSourceWallet, UserID: 103, TokenID: 103, ChannelID: 103,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)
	task := model.Task{TaskID: "task-stale", UserId: 103, ChannelId: 103, Quota: 400,
		Status: model.TaskStatusNotStart, Progress: "0%"}
	require.NoError(t, model.BindBillingLedgerTask(reservation.Ledger.ID, &task))
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", reservation.Ledger.ID).Update("created_at", 1).Error)

	_, err = ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	ledger, err := model.GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
	require.Equal(t, 1000, getUserQuota(t, 103))
	require.Equal(t, 1000, getTokenRemainQuota(t, 103))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	require.Equal(t, 400, stored.Quota)
}
