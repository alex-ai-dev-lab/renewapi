package service

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
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

func TestBillingSessionShadowUsesPersistentLedgerForAllBalanceLegs(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 104, 1000)
	seedToken(t, 104, 104, "sk-shadow-session", 1000)
	seedChannel(t, 104)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 104, TokenId: 104, TokenKey: "sk-shadow-session", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 104},
		RequestId: "req-shadow-session", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	require.NotZero(t, session.ledgerID)
	require.False(t, session.ownsLedgerAccounting())
	require.Equal(t, 600, getUserQuota(t, 104))
	require.Equal(t, 600, getTokenRemainQuota(t, 104))

	require.NoError(t, session.Reserve(600))
	require.Equal(t, 400, getUserQuota(t, 104))
	require.Equal(t, 400, getTokenRemainQuota(t, 104))

	require.NoError(t, session.Settle(250))
	require.Equal(t, 750, getUserQuota(t, 104))
	require.Equal(t, 750, getTokenRemainQuota(t, 104))
	require.Equal(t, 250, getTokenUsedQuota(t, 104))

	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	require.Equal(t, model.BillingComponentStateApplied, ledger.FundingState)
	require.Equal(t, model.BillingComponentStateApplied, ledger.TokenState)

	// Replaying settle must not apply the delta a second time.
	require.NoError(t, session.Settle(250))
	require.Equal(t, 750, getUserQuota(t, 104))
	require.Equal(t, 750, getTokenRemainQuota(t, 104))
}

func TestBillingSessionShadowRefundIsDurableAndIdempotent(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 105, 1000)
	seedToken(t, 105, 105, "sk-shadow-refund", 1000)
	seedChannel(t, 105)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 105, TokenId: 105, TokenKey: "sk-shadow-refund", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 105},
		RequestId: "req-shadow-refund-session", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 300)
	require.Nil(t, apiErr)
	require.Equal(t, 700, getUserQuota(t, 105))
	require.Equal(t, 700, getTokenRemainQuota(t, 105))

	session.Refund(ctx)
	require.Equal(t, 1000, getUserQuota(t, 105))
	require.Equal(t, 1000, getTokenRemainQuota(t, 105))
	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)

	session.Refund(ctx)
	require.Equal(t, 1000, getUserQuota(t, 105))
	require.Equal(t, 1000, getTokenRemainQuota(t, 105))
}

func TestBillingSessionShadowAsyncTaskUsesDurableLedger(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 108, 1000)
	seedToken(t, 108, 108, "sk-shadow-task", 1000)
	seedChannel(t, 108)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 108, TokenId: 108, TokenKey: "sk-shadow-task", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 108},
		RequestId: "req-shadow-task", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	info.Billing = session
	require.NotZero(t, session.ledgerID)
	require.Equal(t, BillingLedgerModeShadow, session.ledgerMode)
	require.True(t, session.usesPersistentBalanceLedger())
	require.NoError(t, PrepareAsyncTaskBilling(info, constant.TaskPlatformSuno))
	require.NotZero(t, session.pendingTaskID)

	var pending model.Task
	require.NoError(t, model.DB.First(&pending, session.pendingTaskID).Error)
	pending.PrivateData.UpstreamTaskID = "upstream-shadow-task"
	pending.Data = []byte(`{"id":"upstream-shadow-task"}`)
	pending.Quota = 250
	require.NoError(t, SettleBillingAndInsertTask(ctx, info, 250, &pending))

	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	require.Equal(t, 750, getUserQuota(t, 108))
	require.Equal(t, 750, getTokenRemainQuota(t, 108))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, session.pendingTaskID).Error)
	require.Equal(t, model.BillingLedgerStateSettled, stored.BillingState)
	require.Equal(t, "upstream-shadow-task", stored.PrivateData.UpstreamTaskID)
}

func TestBillingSessionShadowAsyncTaskReconcilesAfterAcknowledgement(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 109, 1000)
	seedToken(t, 109, 109, "sk-shadow-task-retry", 1000)
	seedChannel(t, 109)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 109, TokenId: 109, TokenKey: "sk-shadow-task-retry", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 109},
		RequestId: "req-shadow-task-retry", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	info.Billing = session
	require.NoError(t, PrepareAsyncTaskBilling(info, constant.TaskPlatformSuno))

	var pending model.Task
	require.NoError(t, model.DB.First(&pending, session.pendingTaskID).Error)
	pending.PrivateData.UpstreamTaskID = "upstream-shadow-task-retry"
	pending.Data = []byte(`{"id":"upstream-shadow-task-retry"}`)
	acknowledged, err := model.AcknowledgeBillingLedgerTask(session.ledgerID, 250, &pending)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateReconcileRequired, acknowledged.State)
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", session.ledgerID).Update("next_retry_at", 0).Error)

	processed, err := ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	require.Positive(t, processed)
	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	require.Equal(t, 750, getUserQuota(t, 109))
	require.Equal(t, 750, getTokenRemainQuota(t, 109))
}

func TestBillingReconcilerReplaysShadowBalanceFailure(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 106, 1000)
	seedToken(t, 106, 106, "sk-shadow-reconcile", 1000)
	seedChannel(t, 106)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 106, TokenId: 106, TokenKey: "sk-shadow-reconcile", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 106},
		RequestId: "req-shadow-reconcile", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 106).Update("remain_quota", 0).Error)
	require.Error(t, session.Settle(500))

	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateReconcileRequired, ledger.State)
	require.Equal(t, 1, ledger.Attempts)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 106).Update("remain_quota", 100).Error)
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", ledger.ID).Update("next_retry_at", 0).Error)

	processed, err := ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	require.Positive(t, processed)
	ledger, err = model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	require.Equal(t, 500, getUserQuota(t, 106))
	require.Equal(t, 0, getTokenRemainQuota(t, 106))
	processed, err = ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Equal(t, 500, getUserQuota(t, 106))
	require.Equal(t, 0, getTokenRemainQuota(t, 106))
}

func TestBillingReconcilerReplaysShadowRefundBalanceFailure(t *testing.T) {
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", BillingLedgerModeShadow)
	seedUser(t, 107, 1000)
	seedToken(t, 107, 107, "sk-shadow-reconcile-refund", 1000)
	seedChannel(t, 107)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 107, TokenId: 107, TokenKey: "sk-shadow-reconcile-refund", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 107},
		RequestId: "req-shadow-reconcile-refund", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 400)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 107).Update("used_quota", 0).Error)
	session.Refund(ctx)

	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateReconcileRequired, ledger.State)
	require.Equal(t, 1, ledger.Attempts)
	require.Equal(t, 600, getUserQuota(t, 107))
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 107).Update("used_quota", 400).Error)
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", ledger.ID).Update("next_retry_at", 0).Error)

	processed, err := ReconcileBillingOnce(t.Context(), 100)
	require.NoError(t, err)
	require.Positive(t, processed)
	ledger, err = model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
	require.Equal(t, 1000, getUserQuota(t, 107))
	require.Equal(t, 1000, getTokenRemainQuota(t, 107))
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
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", reservation.Ledger.ID).Updates(map[string]any{
		"created_at": 1,
		"updated_at": 1,
	}).Error)

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
