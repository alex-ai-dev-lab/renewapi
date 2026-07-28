package model

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type billingLedgerFixture struct {
	user    User
	token   Token
	channel Channel
}

func setupBillingLedgerTest(t *testing.T) billingLedgerFixture {
	t.Helper()
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite := common.UsingSQLite
	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite = oldSQLite
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
	})

	dsn := fmt.Sprintf("file:billing-ledger-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	logDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:billing-log-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)

	DB, LOG_DB = db, logDB
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	require.NoError(t, DB.AutoMigrate(&User{}, &Token{}, &Channel{}, &Task{}, &Midjourney{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}, &BillingLedger{}, &BillingOutbox{}))
	require.NoError(t, LOG_DB.AutoMigrate(&BillingAuditEvent{}))

	fixture := billingLedgerFixture{
		user:    User{Username: "ledger-user", Password: "password123", Quota: 1000},
		token:   Token{UserId: 1, Key: "sk-ledger", Status: common.TokenStatusEnabled, RemainQuota: 1000},
		channel: Channel{Name: "ledger-channel", Key: "key", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, DB.Create(&fixture.user).Error)
	fixture.token.UserId = fixture.user.Id
	require.NoError(t, DB.Create(&fixture.token).Error)
	require.NoError(t, DB.Create(&fixture.channel).Error)
	return fixture
}

func reserveWalletLedger(t *testing.T, fixture billingLedgerFixture, requestID string, quota int64) *BillingReservationResult {
	t.Helper()
	result, err := ReserveBillingLedger(BillingReservation{
		RequestID: requestID, Kind: "request", Mode: "enforce", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: quota, ApplyBalances: true,
	})
	require.NoError(t, err)
	return result
}

func assertLedgerBalances(t *testing.T, fixture billingLedgerFixture, remaining, userUsed, tokenUsed, channelUsed int) {
	t.Helper()
	var user User
	var token Token
	var channel Channel
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.NoError(t, DB.First(&channel, fixture.channel.Id).Error)
	require.Equal(t, remaining, user.Quota)
	require.Equal(t, userUsed, user.UsedQuota)
	require.Equal(t, remaining, token.RemainQuota)
	require.Equal(t, tokenUsed, token.UsedQuota)
	require.EqualValues(t, channelUsed, channel.UsedQuota)
}

func TestBillingLedgerReserveSettleRefundIsIdempotent(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-ledger-idempotent", 400)
	assertLedgerBalances(t, fixture, 600, 0, 400, 0)

	settled, err := SettleBillingLedger(reservation.Ledger.ID, 250)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)
	assertLedgerBalances(t, fixture, 750, 250, 250, 250)

	_, err = SettleBillingLedger(reservation.Ledger.ID, 250)
	require.NoError(t, err)
	assertLedgerBalances(t, fixture, 750, 250, 250, 250)

	refunded, err := RefundBillingLedger(reservation.Ledger.ID, "task failed")
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateRefunded, refunded.State)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)

	_, err = RefundBillingLedger(reservation.Ledger.ID, "duplicate callback")
	require.NoError(t, err)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
	_, err = SettleBillingLedger(reservation.Ledger.ID, 300)
	require.ErrorContains(t, err, "cannot be settled")
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}

func TestBillingLedgerReserveRollsBackAllBalances(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Update("remain_quota", 10).Error)

	_, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-ledger-insufficient", Kind: "request", Mode: "enforce", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 100, ApplyBalances: true,
	})
	require.Error(t, err)
	var user User
	var count int64
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, 1000, user.Quota)
	require.NoError(t, DB.Model(&BillingLedger{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBillingLedgerTaskTerminalCASKeepsBalancesConsistent(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-ledger-task-race", 400)
	task := Task{TaskID: "task-ledger-race", UserId: fixture.user.Id, ChannelId: fixture.channel.Id, Quota: 400,
		Status: TaskStatusInProgress, BillingLedgerID: reservation.Ledger.ID, BillingState: BillingLedgerStateReserved}
	require.NoError(t, DB.Create(&task).Error)

	success := task
	success.Status = TaskStatusSuccess
	success.Progress = "100%"
	failure := task
	failure.Status = TaskStatusFailure
	failure.Progress = "100%"
	failure.FailReason = "upstream failed"

	var wg sync.WaitGroup
	wins := make(chan string, 2)
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		won, _, err := UpdateTaskWithBilling(&success, TaskStatusInProgress, BillingLedgerDesiredSettle, 300, "")
		if err != nil {
			errs <- err
		} else if won {
			wins <- "success"
		}
	}()
	go func() {
		defer wg.Done()
		won, _, err := UpdateTaskWithBilling(&failure, TaskStatusInProgress, BillingLedgerDesiredRefund, 0, "upstream failed")
		if err != nil {
			errs <- err
		} else if won {
			wins <- "failure"
		}
	}()
	wg.Wait()
	close(wins)
	close(errs)
	require.Empty(t, errs)
	require.Len(t, wins, 1)

	var storedTask Task
	var ledger BillingLedger
	require.NoError(t, DB.First(&storedTask, task.ID).Error)
	require.NoError(t, DB.First(&ledger, reservation.Ledger.ID).Error)
	switch storedTask.Status {
	case TaskStatusSuccess:
		require.Equal(t, BillingLedgerStateSettled, ledger.State)
		require.EqualValues(t, 300, ledger.AppliedQuota)
		assertLedgerBalances(t, fixture, 700, 300, 300, 300)
	case TaskStatusFailure:
		require.Equal(t, BillingLedgerStateRefunded, ledger.State)
		require.Zero(t, ledger.AppliedQuota)
		assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
	default:
		t.Fatalf("unexpected terminal status %s", storedTask.Status)
	}
}

func TestBillingOutboxDeliveryIsIdempotent(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-ledger-outbox", 100)
	_, err := SettleBillingLedger(reservation.Ledger.ID, 80)
	require.NoError(t, err)

	processed, err := ProcessBillingOutbox(100)
	require.NoError(t, err)
	require.Equal(t, 2, processed)
	processed, err = ProcessBillingOutbox(100)
	require.NoError(t, err)
	require.Zero(t, processed)
	var count int64
	require.NoError(t, LOG_DB.Model(&BillingAuditEvent{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestBillingLedgerRefundedTaskCanPersistTerminalStatus(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-ledger-refunded-task", 400)
	task := Task{TaskID: "task-ledger-refunded", UserId: fixture.user.Id, ChannelId: fixture.channel.Id, Quota: 400,
		Status: TaskStatusInProgress, BillingLedgerID: reservation.Ledger.ID, BillingState: BillingLedgerStateReserved}
	require.NoError(t, DB.Create(&task).Error)
	_, err := RefundBillingLedger(reservation.Ledger.ID, "request already failed")
	require.NoError(t, err)

	task.Status = TaskStatusFailure
	task.Progress = "100%"
	task.FailReason = "upstream failed"
	won, ledger, err := UpdateTaskWithBilling(&task, TaskStatusInProgress, BillingLedgerDesiredRefund, 0, task.FailReason)
	require.NoError(t, err)
	require.True(t, won)
	require.Equal(t, BillingLedgerStateRefunded, ledger.State)

	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	require.Equal(t, TaskStatus(TaskStatusFailure), stored.Status)
	require.Equal(t, 400, stored.Quota)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}

func TestBillingLedgerMidjourneyTerminalRaceIsExactlyOnce(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-ledger-midjourney-race", 400)
	task := Midjourney{MjId: "mj-ledger-race", UserId: fixture.user.Id, ChannelId: fixture.channel.Id, Quota: 400,
		Status: "", Progress: "0%", BillingLedgerID: reservation.Ledger.ID, BillingState: BillingLedgerStateReserved}
	ledger, err := SettleBillingLedgerWithMidjourney(reservation.Ledger.ID, 400, &task)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, ledger.State)

	success := task
	success.Status = string(TaskStatusSuccess)
	success.Progress = "100%"
	failure := task
	failure.Status = string(TaskStatusFailure)
	failure.Progress = "100%"
	failure.FailReason = "upstream failed"

	var wg sync.WaitGroup
	wins := make(chan string, 2)
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		won, _, err := UpdateMidjourneyWithBilling(&success, "", BillingLedgerDesiredSettle, 400, "")
		if err != nil {
			errs <- err
		} else if won {
			wins <- "success"
		}
	}()
	go func() {
		defer wg.Done()
		won, _, err := UpdateMidjourneyWithBilling(&failure, "", BillingLedgerDesiredRefund, 0, failure.FailReason)
		if err != nil {
			errs <- err
		} else if won {
			wins <- "failure"
		}
	}()
	wg.Wait()
	close(wins)
	close(errs)
	require.Empty(t, errs)
	require.Len(t, wins, 1)

	var stored Midjourney
	require.NoError(t, DB.First(&stored, task.Id).Error)
	require.Equal(t, 400, stored.Quota)
	if stored.Status == string(TaskStatusSuccess) {
		assertLedgerBalances(t, fixture, 600, 400, 400, 400)
	} else {
		require.Equal(t, string(TaskStatusFailure), stored.Status)
		assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
	}
}

func TestRefundSubscriptionPreConsumeUsesOwningTransaction(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	subscription := UserSubscription{UserId: fixture.user.Id, AmountTotal: 1000, AmountUsed: 400, Status: "active", EndTime: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, DB.Create(&subscription).Error)
	record := SubscriptionPreConsumeRecord{RequestId: "req-sub-refund", UserId: fixture.user.Id, UserSubscriptionId: subscription.Id, PreConsumed: 100, Status: "consumed"}
	require.NoError(t, DB.Create(&record).Error)

	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))

	require.NoError(t, DB.First(&subscription, subscription.Id).Error)
	require.EqualValues(t, 300, subscription.AmountUsed)
	require.NoError(t, DB.First(&record, record.Id).Error)
	require.Equal(t, "refunded", record.Status)
}

func TestPreparedTaskAcknowledgementSurvivesSettlementRetry(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-prepared-task", 400)
	task := Task{TaskID: "task-prepared", UserId: fixture.user.Id, ChannelId: fixture.channel.Id,
		Status: TaskStatusNotStart, Progress: "0%", Quota: 400}
	require.NoError(t, BindBillingLedgerTask(reservation.Ledger.ID, &task))
	require.NotZero(t, task.ID)

	task.PrivateData.UpstreamTaskID = "upstream-accepted-1"
	task.Data = []byte(`{"id":"upstream-accepted-1"}`)
	acknowledged, err := AcknowledgeBillingLedgerTask(reservation.Ledger.ID, 300, &task)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateReconcileRequired, acknowledged.State)

	settled, err := SettleBillingLedger(reservation.Ledger.ID, 300)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)
	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	require.Equal(t, "upstream-accepted-1", stored.PrivateData.UpstreamTaskID)
	require.JSONEq(t, `{"id":"upstream-accepted-1"}`, string(stored.Data))
	require.Equal(t, BillingLedgerStateSettled, stored.BillingState)
	assertLedgerBalances(t, fixture, 700, 300, 300, 300)
}
