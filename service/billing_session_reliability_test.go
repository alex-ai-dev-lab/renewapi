package service

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBillingReliabilitySession(t *testing.T, mode, source string) (*gin.Context, *BillingSession) {
	t.Helper()
	truncate(t)
	t.Setenv("BILLING_LEDGER_MODE", mode)
	seedUser(t, 401, 1000)
	seedToken(t, 401, 401, "billing-reliability-token", 1000)
	seedChannel(t, 401)
	if source == BillingSourceSubscription {
		plan := model.SubscriptionPlan{Id: 401, Title: "计费验证", TotalAmount: 1000, QuotaResetPeriod: "never"}
		require.NoError(t, model.DB.Create(&plan).Error)
		sub := model.UserSubscription{Id: 401, UserId: 401, PlanId: 401, AmountTotal: 1000, Status: "active", StartTime: time.Now().Unix() - 60, EndTime: time.Now().Unix() + 3600}
		require.NoError(t, model.DB.Create(&sub).Error)
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 401, TokenId: 401, TokenKey: "billing-reliability-token",
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 401}, RequestId: "billing-reliability-request",
		ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: source + "_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 300)
	require.Nil(t, apiErr)
	return ctx, session
}

func assertBillingReliabilityBalances(t *testing.T, session *BillingSession, quota int) {
	t.Helper()
	if session.funding.Source() == BillingSourceWallet {
		require.Equal(t, 1000-quota, getUserQuota(t, 401))
	} else {
		var sub model.UserSubscription
		require.NoError(t, model.DB.First(&sub, 401).Error)
		require.EqualValues(t, quota, sub.AmountUsed)
		require.Equal(t, 1000, getUserQuota(t, 401))
	}
	require.Equal(t, 1000-quota, getTokenRemainQuota(t, 401))
	require.Equal(t, quota, getTokenUsedQuota(t, 401))
}

func failBillingTokenWrites(t *testing.T) func() {
	t.Helper()
	const callback = "billing_reliability_injected_failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "tokens" {
			tx.AddError(errors.New("注入令牌写入故障"))
		}
	}))
	remove := func() { require.NoError(t, model.DB.Callback().Update().Remove(callback)) }
	t.Cleanup(remove)
	return remove
}

func TestBillingSessionRefundFailureIsAtomicAndRetryable(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeOff, BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		for _, source := range []string{BillingSourceWallet, BillingSourceSubscription} {
			t.Run(mode+"/"+source, func(t *testing.T) {
				ctx, session := newBillingReliabilitySession(t, mode, source)
				require.NoError(t, session.Reserve(500))
				remove := failBillingTokenWrites(t)
				session.Refund(ctx)
				require.True(t, session.NeedsRefund())
				assertBillingReliabilityBalances(t, session, 500)
				if source == BillingSourceSubscription {
					var record model.SubscriptionPreConsumeRecord
					require.NoError(t, model.DB.Where("request_id = ?", session.relayInfo.RequestId).First(&record).Error)
					require.Equal(t, "consumed", record.Status)
				}
				remove()
				session.Refund(ctx)
				session.Refund(ctx)
				assertBillingReliabilityBalances(t, session, 0)
				require.False(t, session.NeedsRefund())
				require.Error(t, session.Settle(200))
				require.Error(t, session.Reserve(600))
				assertBillingReliabilityBalances(t, session, 0)
			})
		}
	}
}

func TestBillingSessionSettleFailureDoesNotPartiallyApply(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeOff, BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		for _, source := range []string{BillingSourceWallet, BillingSourceSubscription} {
			t.Run(mode+"/"+source, func(t *testing.T) {
				ctx, session := newBillingReliabilitySession(t, mode, source)
				remove := failBillingTokenWrites(t)
				require.Error(t, session.Settle(200))
				assertBillingReliabilityBalances(t, session, 300)
				remove()
				require.NoError(t, session.Settle(200))
				require.NoError(t, session.Settle(200))
				session.Refund(ctx)
				assertBillingReliabilityBalances(t, session, 200)
				var user model.User
				require.NoError(t, model.DB.First(&user, 401).Error)
				if mode == BillingLedgerModeEnforce {
					require.Equal(t, 200, user.UsedQuota)
					require.Equal(t, 1, user.RequestCount)
				} else {
					require.Zero(t, user.UsedQuota)
				}
			})
		}
	}
}

func TestBillingSessionFailedReserveKeepsLiveReservation(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			ctx, session := newBillingReliabilitySession(t, mode, BillingSourceWallet)
			remove := failBillingTokenWrites(t)
			require.Error(t, session.Reserve(600))
			assertBillingReliabilityBalances(t, session, 300)
			ledger, err := model.GetBillingLedger(session.ledgerID)
			require.NoError(t, err)
			require.Equal(t, model.BillingLedgerStateReserved, ledger.State)
			remove()
			require.NoError(t, session.Reserve(600))
			require.NoError(t, session.Reserve(600))
			assertBillingReliabilityBalances(t, session, 600)
			session.Refund(ctx)
			_, err = model.ReserveMoreBillingLedger(session.ledgerID, 700)
			require.Error(t, err)
		})
	}
}

func TestBillingSessionConcurrentTerminalTransitions(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeOff, BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			ctx, session := newBillingReliabilitySession(t, mode, BillingSourceWallet)
			var workers sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 16; i++ {
				workers.Add(1)
				go func(index int) {
					defer workers.Done()
					<-start
					if index%2 == 0 {
						session.Refund(ctx)
					} else {
						_ = session.Settle(200)
					}
				}(i)
			}
			close(start)
			workers.Wait()
			quota := getTokenUsedQuota(t, 401)
			require.Contains(t, []int{0, 200}, quota, fmt.Sprint(mode))
			assertBillingReliabilityBalances(t, session, quota)
			require.False(t, session.NeedsRefund())
		})
	}
}

func TestBillingSessionRefundsUnmatchedLegacyFundingReserve(t *testing.T) {
	truncate(t)
	seedUser(t, 401, 900)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	session := &BillingSession{
		ledgerMode: BillingLedgerModeOff,
		relayInfo:  &relaycommon.RelayInfo{UserId: 401, IsPlayground: true},
		funding:    &WalletFunding{userId: 401, consumed: 100},
	}
	require.True(t, session.NeedsRefund())
	session.Refund(ctx)
	require.Equal(t, 1000, getUserQuota(t, 401))
	require.False(t, session.NeedsRefund())
}

func TestBillingSessionReconcilePreservesSuccessfulChannel(t *testing.T) {
	_, session := newBillingReliabilitySession(t, BillingLedgerModeEnforce, BillingSourceWallet)
	seedChannel(t, 402)
	session.relayInfo.ChannelMeta.ChannelId = 402
	remove := failBillingTokenWrites(t)
	require.Error(t, session.Settle(200))
	remove()
	ledger, err := model.GetBillingLedger(session.ledgerID)
	require.NoError(t, err)
	require.Equal(t, 402, ledger.ChannelID)
	require.Equal(t, model.BillingLedgerStateReconcileRequired, ledger.State)
	require.NoError(t, model.DB.Model(ledger).Update("next_retry_at", 0).Error)
	_, err = ReconcileBillingOnce(nil, 100)
	require.NoError(t, err)
	require.NoError(t, session.Settle(200))
	assertBillingReliabilityBalances(t, session, 200)
	var failed, winner model.Channel
	require.NoError(t, model.DB.First(&failed, 401).Error)
	require.NoError(t, model.DB.First(&winner, 402).Error)
	require.Zero(t, failed.UsedQuota)
	require.EqualValues(t, 200, winner.UsedQuota)
}

func TestBillingSessionZeroReservationFinalizesRefund(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			truncate(t)
			t.Setenv("BILLING_LEDGER_MODE", mode)
			seedUser(t, 401, 1000)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{UserId: 401, RequestId: "zero-reservation", IsPlayground: true,
				UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}, ForcePreConsume: true}
			session, apiErr := NewBillingSession(ctx, info, 0)
			require.Nil(t, apiErr)
			require.True(t, session.NeedsRefund())
			session.Refund(ctx)
			ledger, err := model.GetBillingLedger(session.ledgerID)
			require.NoError(t, err)
			require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
			require.Equal(t, 1000, getUserQuota(t, 401))
		})
	}
}
