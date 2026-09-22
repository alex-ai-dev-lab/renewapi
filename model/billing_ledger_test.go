package model

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
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
	oldMySQL, oldPostgres := common.UsingMySQL, common.UsingPostgreSQL
	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite = oldSQLite
		common.UsingMySQL, common.UsingPostgreSQL = oldMySQL, oldPostgres
		initCol()
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
	})

	dialect := func(name string) gorm.Dialector {
		driver, dsn := os.Getenv("BILLING_TEST_DRIVER"), os.Getenv("BILLING_TEST_DSN")
		if driver != "" && dsn == "" {
			t.Fatal("计费测试必须指定独立数据库 DSN")
		}
		switch driver {
		case "mysql":
			return mysql.Open(dsn)
		case "postgres":
			return postgres.Open(dsn)
		case "":
			return sqlite.Open(fmt.Sprintf("file:%s-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", name, time.Now().UnixNano()))
		default:
			t.Fatalf("不支持的计费测试数据库类型 %q", driver)
			return nil
		}
	}
	db, err := gorm.Open(dialect("billing-ledger"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	if db.Dialector.Name() == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(8)
	}
	logDB, err := gorm.Open(dialect("billing-log"), &gorm.Config{})
	require.NoError(t, err)

	DB, LOG_DB = db, logDB
	common.UsingSQLite = db.Dialector.Name() == "sqlite"
	common.UsingMySQL = db.Dialector.Name() == "mysql"
	common.UsingPostgreSQL = db.Dialector.Name() == "postgres"
	initCol()
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	models := []any{&User{}, &Token{}, &Channel{}, &Task{}, &Midjourney{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}, &BillingLedger{}, &BillingOutbox{}}
	if !common.UsingSQLite {
		require.NoError(t, db.Migrator().DropTable(models...))
		require.NoError(t, logDB.Migrator().DropTable(&BillingAuditEvent{}))
	}
	t.Cleanup(func() {
		if db.Dialector.Name() != "sqlite" {
			_ = db.Migrator().DropTable(models...)
			_ = logDB.Migrator().DropTable(&BillingAuditEvent{})
		}
		_ = sqlDB.Close()
		logSQL, _ := logDB.DB()
		_ = logSQL.Close()
	})
	require.NoError(t, DB.AutoMigrate(models...))
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

func TestShadowLedgerBalanceTransitionRollsBackAndRetries(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-atomic", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)
	assertLedgerBalances(t, fixture, 600, 0, 400, 0)
	require.Equal(t, BillingComponentStateApplied, reservation.Ledger.FundingState)
	require.Equal(t, BillingComponentStateApplied, reservation.Ledger.TokenState)
	persisted, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingComponentStateApplied, persisted.FundingState)
	require.Equal(t, BillingComponentStateApplied, persisted.TokenState)

	// Make the token leg fail after the wallet leg would have been applied.
	// The transaction must roll the wallet update back and leave the ledger
	// retryable rather than claiming a terminal settle.
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Updates(map[string]any{
		"remain_quota": 0,
	}).Error)
	_, err = SettleBillingLedger(reservation.Ledger.ID, 500)
	require.Error(t, err)
	var userAfterFailure User
	var tokenAfterFailure Token
	require.NoError(t, DB.First(&userAfterFailure, fixture.user.Id).Error)
	require.NoError(t, DB.First(&tokenAfterFailure, fixture.token.Id).Error)
	require.EqualValues(t, 600, userAfterFailure.Quota)
	require.EqualValues(t, 0, tokenAfterFailure.RemainQuota)
	require.EqualValues(t, 400, tokenAfterFailure.UsedQuota)
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateReserved, ledger.State)
	require.EqualValues(t, 400, ledger.AppliedQuota)

	// Restore the failed component and replay the same target. The retry is
	// idempotent and applies the delta exactly once.
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Updates(map[string]any{
		"remain_quota": 100,
	}).Error)
	settled, err := SettleBillingLedger(reservation.Ledger.ID, 500)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)
	var settledUser User
	var settledToken Token
	require.NoError(t, DB.First(&settledUser, fixture.user.Id).Error)
	require.NoError(t, DB.First(&settledToken, fixture.token.Id).Error)
	require.EqualValues(t, 500, settledUser.Quota)
	require.EqualValues(t, 0, settledToken.RemainQuota)
	require.EqualValues(t, 500, settledToken.UsedQuota)
	_, err = SettleBillingLedger(reservation.Ledger.ID, 500)
	require.NoError(t, err)
	require.NoError(t, DB.First(&settledUser, fixture.user.Id).Error)
	require.NoError(t, DB.First(&settledToken, fixture.token.Id).Error)
	require.EqualValues(t, 500, settledUser.Quota)
	require.EqualValues(t, 0, settledToken.RemainQuota)
	require.EqualValues(t, 500, settledToken.UsedQuota)
}

func TestShadowSubscriptionLedgerReserveMoreAndRefundPreservesAllLegs(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPlan{}))
	plan := SubscriptionPlan{Title: "ledger plan", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	require.NoError(t, DB.Create(&plan).Error)
	subscription := UserSubscription{
		UserId: fixture.user.Id, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 0,
		Status: "active", StartTime: time.Now().Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, DB.Create(&subscription).Error)

	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-subscription", Kind: "request", Mode: "shadow", FundingSource: "subscription",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)
	require.Equal(t, subscription.Id, reservation.Ledger.SubscriptionID)
	require.NoError(t, DB.First(&subscription, subscription.Id).Error)
	require.EqualValues(t, 400, subscription.AmountUsed)

	_, err = ReserveMoreBillingLedger(reservation.Ledger.ID, 600)
	require.NoError(t, err)
	require.NoError(t, DB.First(&subscription, subscription.Id).Error)
	require.EqualValues(t, 600, subscription.AmountUsed)

	settled, err := SettleBillingLedger(reservation.Ledger.ID, 250)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)
	require.Equal(t, BillingComponentStateApplied, settled.SubscriptionState)
	require.NoError(t, DB.First(&subscription, subscription.Id).Error)
	require.EqualValues(t, 250, subscription.AmountUsed)

	refunded, err := RefundBillingLedger(reservation.Ledger.ID, "request failed")
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateRefunded, refunded.State)
	require.NoError(t, DB.First(&subscription, subscription.Id).Error)
	require.EqualValues(t, 0, subscription.AmountUsed)
	var token Token
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.EqualValues(t, 1000, token.RemainQuota)
	require.EqualValues(t, 0, token.UsedQuota)
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", reservation.Ledger.RequestID).First(&record).Error)
	require.Equal(t, "refunded", record.Status)
}

func TestShadowLedgerConcurrentSettleIsExactlyOnce(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-concurrent-settle", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := SettleBillingLedger(reservation.Ledger.ID, 250)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, ledger.State)
	assertLedgerBalances(t, fixture, 750, 0, 250, 0)
}

func TestShadowLedgerConcurrentRefundIsExactlyOnce(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-concurrent-refund", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := RefundBillingLedger(reservation.Ledger.ID, "duplicate delivery")
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateRefunded, ledger.State)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}

func TestShadowLedgerConcurrentSettleAndRefundNeverLeavesPartialBalance(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-concurrent-settle-refund", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)

	start := make(chan struct{})
	settleErr := make(chan error, 1)
	refundErr := make(chan error, 1)
	go func() {
		<-start
		_, err := SettleBillingLedger(reservation.Ledger.ID, 250)
		settleErr <- err
	}()
	go func() {
		<-start
		_, err := RefundBillingLedger(reservation.Ledger.ID, "request failed")
		refundErr <- err
	}()
	close(start)
	_ = <-settleErr // Refund may win first, making settle correctly reject the terminal ledger.
	require.NoError(t, <-refundErr)
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateRefunded, ledger.State)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}

func TestShadowLedgerRefundRollsBackAndRetries(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-refund", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)

	// Force the token refund invariant to fail. Wallet credit must be rolled
	// back instead of leaving a partial refund.
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Update("used_quota", 0).Error)
	_, err = RefundBillingLedger(reservation.Ledger.ID, "simulated refund failure")
	require.Error(t, err)
	assertLedgerBalances(t, fixture, 600, 0, 0, 0)
	ledger, err := GetBillingLedger(reservation.Ledger.ID)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateReserved, ledger.State)

	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Update("used_quota", 400).Error)
	refunded, err := RefundBillingLedger(reservation.Ledger.ID, "retry")
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateRefunded, refunded.State)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
	_, err = RefundBillingLedger(reservation.Ledger.ID, "duplicate")
	require.NoError(t, err)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}

func TestShadowLedgerRefundMissingWalletDoesNotBecomeTerminal(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation, err := ReserveBillingLedger(BillingReservation{
		RequestID: "req-shadow-missing-wallet", Kind: "request", Mode: "shadow", FundingSource: "wallet",
		UserID: fixture.user.Id, TokenID: fixture.token.Id, ChannelID: fixture.channel.Id,
		Quota: 400, ApplyBalances: true,
	})
	require.NoError(t, err)
	require.NoError(t, DB.Delete(&User{}, fixture.user.Id).Error)

	_, err = RefundBillingLedger(reservation.Ledger.ID, "user removed during refund")
	require.ErrorContains(t, err, "user quota refund invariant failed")

	var ledger BillingLedger
	var token Token
	require.NoError(t, DB.First(&ledger, reservation.Ledger.ID).Error)
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.Equal(t, BillingLedgerStateReserved, ledger.State)
	require.Equal(t, 600, token.RemainQuota)
	require.Equal(t, 400, token.UsedQuota)
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

	prepared := Task{
		TaskID:     "task-prepared-public",
		Platform:   "prepared-platform",
		UserId:     fixture.user.Id,
		Group:      "prepared-group",
		ChannelId:  fixture.channel.Id,
		Action:     "prepared-action",
		Status:     TaskStatusNotStart,
		SubmitTime: 123456789,
		Progress:   "0%",
		Quota:      400,
		Properties: Properties{
			Input:             "prepared-input",
			UpstreamModelName: "prepared-upstream-model",
			OriginModelName:   "prepared-origin-model",
		},
		PrivateData: TaskPrivateData{
			Key:            "prepared-private-key",
			BillingSource:  "wallet",
			SubscriptionId: 77,
			TokenId:        fixture.token.Id,
			BillingContext: &TaskBillingContext{
				ModelPrice:      2.5,
				GroupRatio:      1.2,
				ModelRatio:      1.5,
				OtherRatios:     map[string]float64{"duration": 2},
				OriginModelName: "prepared-video-model",
				PerCallBilling:  true,
			},
		},
	}
	require.NoError(t, BindBillingLedgerTask(reservation.Ledger.ID, &prepared))
	require.NotZero(t, prepared.ID)

	ack := Task{
		ID:         prepared.ID,
		TaskID:     "ack-must-not-replace-public-id",
		Platform:   "ack-platform",
		UserId:     prepared.UserId + 100,
		Group:      "ack-group",
		ChannelId:  prepared.ChannelId + 100,
		Quota:      999,
		Action:     "ack-action",
		Status:     TaskStatusSubmitted,
		SubmitTime: prepared.SubmitTime + 100,
		StartTime:  prepared.SubmitTime + 1,
		FinishTime: prepared.SubmitTime + 2,
		Progress:   "10%",
		Properties: Properties{
			Input:             "ack-input",
			UpstreamModelName: "ack-upstream-model",
			OriginModelName:   "ack-origin-model",
		},
		PrivateData: TaskPrivateData{
			Key:            "ack-private-key",
			UpstreamTaskID: "upstream-accepted-1",
			ResultURL:      "https://example.invalid/result",
			BillingSource:  "ack-source",
			SubscriptionId: 999,
			TokenId:        999,
			BillingContext: &TaskBillingContext{
				ModelPrice:      99,
				GroupRatio:      99,
				ModelRatio:      99,
				OriginModelName: "ack-model",
			},
		},
		Data: []byte(`{"id":"upstream-accepted-1"}`),
	}

	acknowledged, err := AcknowledgeBillingLedgerTask(reservation.Ledger.ID, 300, &ack)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateReconcileRequired, acknowledged.State)

	// 调用者拿到的 task 也必须是合并后的持久化身份，而不是 fresh object。
	require.Equal(t, prepared.TaskID, ack.TaskID)
	require.Equal(t, prepared.Platform, ack.Platform)
	require.Equal(t, prepared.UserId, ack.UserId)
	require.Equal(t, prepared.Group, ack.Group)
	require.Equal(t, prepared.ChannelId, ack.ChannelId)
	require.Equal(t, prepared.Action, ack.Action)
	require.Equal(t, prepared.SubmitTime, ack.SubmitTime)
	require.Equal(t, prepared.Properties, ack.Properties)
	require.Equal(t, prepared.PrivateData.Key, ack.PrivateData.Key)
	require.Equal(t, prepared.PrivateData.BillingSource, ack.PrivateData.BillingSource)
	require.Equal(t, prepared.PrivateData.SubscriptionId, ack.PrivateData.SubscriptionId)
	require.Equal(t, prepared.PrivateData.TokenId, ack.PrivateData.TokenId)
	require.Equal(t, prepared.PrivateData.BillingContext, ack.PrivateData.BillingContext)

	require.Equal(t, TaskStatus(TaskStatusSubmitted), ack.Status)
	require.Equal(t, "10%", ack.Progress)
	require.Equal(t, prepared.SubmitTime+1, ack.StartTime)
	require.Equal(t, prepared.SubmitTime+2, ack.FinishTime)
	require.Equal(t, "upstream-accepted-1", ack.PrivateData.UpstreamTaskID)
	require.Equal(t, "https://example.invalid/result", ack.PrivateData.ResultURL)
	require.Equal(t, 300, ack.Quota)

	settled, err := SettleBillingLedger(reservation.Ledger.ID, 300)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)

	var stored Task
	require.NoError(t, DB.First(&stored, prepared.ID).Error)

	require.Equal(t, prepared.TaskID, stored.TaskID)
	require.Equal(t, prepared.Platform, stored.Platform)
	require.Equal(t, prepared.UserId, stored.UserId)
	require.Equal(t, prepared.Group, stored.Group)
	require.Equal(t, prepared.ChannelId, stored.ChannelId)
	require.Equal(t, prepared.Action, stored.Action)
	require.Equal(t, prepared.SubmitTime, stored.SubmitTime)
	require.Equal(t, prepared.Properties, stored.Properties)

	require.Equal(t, "prepared-private-key", stored.PrivateData.Key)
	require.Equal(t, "upstream-accepted-1", stored.PrivateData.UpstreamTaskID)
	require.Equal(t, "wallet", stored.PrivateData.BillingSource)
	require.Equal(t, 77, stored.PrivateData.SubscriptionId)
	require.Equal(t, fixture.token.Id, stored.PrivateData.TokenId)
	require.Equal(t, prepared.PrivateData.BillingContext, stored.PrivateData.BillingContext)

	require.Equal(t, TaskStatus(TaskStatusSubmitted), stored.Status)
	require.Equal(t, "10%", stored.Progress)
	require.Equal(t, prepared.SubmitTime+1, stored.StartTime)
	require.Equal(t, prepared.SubmitTime+2, stored.FinishTime)
	require.Equal(t, "https://example.invalid/result", stored.PrivateData.ResultURL)
	require.JSONEq(t, `{"id":"upstream-accepted-1"}`, string(stored.Data))

	require.Equal(t, 300, stored.Quota)
	require.Equal(t, BillingLedgerStateSettled, stored.BillingState)
	require.Equal(t, settled.Version, stored.BillingVersion)
	assertLedgerBalances(t, fixture, 700, 300, 300, 300)
}

func TestBillingLedgerReconcileSnapshotFence(t *testing.T) {
	t.Run("stale refund snapshot cannot undo newer settlement", func(t *testing.T) {
		fixture := setupBillingLedgerTest(t)
		reservation := reserveWalletLedger(t, fixture, "req-reconcile-stale-settlement", 400)

		require.NoError(t, MarkBillingLedgerForReconcile(
			reservation.Ledger.ID,
			BillingLedgerDesiredRefund,
			0,
			fmt.Errorf("refund retry required"),
		))

		snapshot, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateReconcileRequired, snapshot.State)

		settled, err := SettleBillingLedger(reservation.Ledger.ID, 300)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateSettled, settled.State)
		require.Greater(t, settled.Version, snapshot.Version)

		applied, err := ReplayBillingLedgerReconcileContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			snapshot.LastError,
		)
		require.NoError(t, err)
		require.False(t, applied)

		marked, err := MarkBillingLedgerReconcileRetryContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			fmt.Errorf("stale replay failure"),
		)
		require.NoError(t, err)
		require.False(t, marked)

		stored, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateSettled, stored.State)
		require.Equal(t, settled.Version, stored.Version)
		require.EqualValues(t, 300, stored.AppliedQuota)

		assertLedgerBalances(t, fixture, 700, 300, 300, 300)
	})

	t.Run("stale settle snapshot cannot cross newer acknowledgement", func(t *testing.T) {
		fixture := setupBillingLedgerTest(t)
		reservation := reserveWalletLedger(t, fixture, "req-reconcile-stale-ack", 400)

		prepared := Task{
			TaskID:    "task-reconcile-ack",
			UserId:    fixture.user.Id,
			ChannelId: fixture.channel.Id,
			Status:    TaskStatusNotStart,
			Progress:  "0%",
		}
		require.NoError(t, BindBillingLedgerTask(reservation.Ledger.ID, &prepared))

		firstAck := Task{
			ID:       prepared.ID,
			Status:   TaskStatusSubmitted,
			Progress: "0%",
			PrivateData: TaskPrivateData{
				UpstreamTaskID: "upstream-first",
			},
		}
		ledger, err := AcknowledgeBillingLedgerTask(reservation.Ledger.ID, 300, &firstAck)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateReconcileRequired, ledger.State)

		snapshot, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)

		lateAck := Task{
			ID:       prepared.ID,
			Status:   TaskStatusSubmitted,
			Progress: "5%",
			PrivateData: TaskPrivateData{
				UpstreamTaskID: "upstream-late",
			},
		}
		newer, err := AcknowledgeBillingLedgerTask(reservation.Ledger.ID, 300, &lateAck)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateReconcileRequired, newer.State)
		require.Greater(t, newer.Version, snapshot.Version)

		applied, err := ReplayBillingLedgerReconcileContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			snapshot.LastError,
		)
		require.NoError(t, err)
		require.False(t, applied)

		marked, err := MarkBillingLedgerReconcileRetryContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			fmt.Errorf("stale replay failure"),
		)
		require.NoError(t, err)
		require.False(t, marked)

		stored, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateReconcileRequired, stored.State)
		require.Equal(t, newer.Version, stored.Version)

		var task Task
		require.NoError(t, DB.First(&task, prepared.ID).Error)
		require.Equal(t, "upstream-late", task.PrivateData.UpstreamTaskID)
		require.Equal(t, "5%", task.Progress)

		assertLedgerBalances(t, fixture, 600, 0, 400, 0)
	})

	t.Run("matching settle snapshot applies", func(t *testing.T) {
		fixture := setupBillingLedgerTest(t)
		reservation := reserveWalletLedger(t, fixture, "req-reconcile-matching-settle", 400)

		require.NoError(t, MarkBillingLedgerForReconcile(
			reservation.Ledger.ID,
			BillingLedgerDesiredSettle,
			300,
			fmt.Errorf("settle retry required"),
		))

		snapshot, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)

		applied, err := ReplayBillingLedgerReconcileContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			snapshot.LastError,
		)
		require.NoError(t, err)
		require.True(t, applied)

		stored, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateSettled, stored.State)
		require.EqualValues(t, 300, stored.AppliedQuota)

		assertLedgerBalances(t, fixture, 700, 300, 300, 300)
	})

	t.Run("matching refund snapshot applies", func(t *testing.T) {
		fixture := setupBillingLedgerTest(t)
		reservation := reserveWalletLedger(t, fixture, "req-reconcile-matching-refund", 400)

		require.NoError(t, MarkBillingLedgerForReconcile(
			reservation.Ledger.ID,
			BillingLedgerDesiredRefund,
			0,
			fmt.Errorf("refund retry required"),
		))

		snapshot, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)

		applied, err := ReplayBillingLedgerReconcileContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			snapshot.LastError,
		)
		require.NoError(t, err)
		require.True(t, applied)

		stored, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateRefunded, stored.State)
		require.Zero(t, stored.AppliedQuota)

		assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
	})

	t.Run("matching retry marker advances version once", func(t *testing.T) {
		fixture := setupBillingLedgerTest(t)
		reservation := reserveWalletLedger(t, fixture, "req-reconcile-retry-marker", 400)

		require.NoError(t, MarkBillingLedgerForReconcile(
			reservation.Ledger.ID,
			BillingLedgerDesiredSettle,
			300,
			fmt.Errorf("initial settlement failure"),
		))

		snapshot, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)

		marked, err := MarkBillingLedgerReconcileRetryContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			fmt.Errorf("replay failed"),
		)
		require.NoError(t, err)
		require.True(t, marked)

		afterRetry, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, BillingLedgerStateReconcileRequired, afterRetry.State)
		require.Equal(t, snapshot.Version+1, afterRetry.Version)
		require.Equal(t, snapshot.DesiredState, afterRetry.DesiredState)
		require.Equal(t, snapshot.DesiredQuota, afterRetry.DesiredQuota)
		require.ErrorContains(t, errors.New(afterRetry.LastError), "replay failed")

		marked, err = MarkBillingLedgerReconcileRetryContext(
			nil,
			snapshot.ID,
			snapshot.Version,
			snapshot.DesiredState,
			snapshot.DesiredQuota,
			fmt.Errorf("stale retry must not win"),
		)
		require.NoError(t, err)
		require.False(t, marked)

		stored, err := GetBillingLedger(reservation.Ledger.ID)
		require.NoError(t, err)
		require.Equal(t, afterRetry.Version, stored.Version)
		require.Equal(t, afterRetry.LastError, stored.LastError)

		assertLedgerBalances(t, fixture, 600, 0, 400, 0)
	})
}

func TestStaleReservedBillingLedgerUsesUpdatedAtAndVersionFence(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-stale-activity-fence", 400)

	cutoff := time.Now().Unix() - 60
	staleAt := cutoff - 1
	require.NoError(t, DB.Model(&BillingLedger{}).
		Where("id = ?", reservation.Ledger.ID).
		UpdateColumns(map[string]any{
			"created_at": staleAt,
			"updated_at": staleAt,
		}).Error)

	stale, err := ListStaleReservedBillingLedgers("request", cutoff, 10)
	require.NoError(t, err)
	require.Len(t, stale, 1)
	observed := stale[0]

	refreshed, err := ReserveMoreBillingLedger(reservation.Ledger.ID, 450)
	require.NoError(t, err)
	require.Greater(t, refreshed.Version, observed.Version)
	require.Greater(t, refreshed.UpdatedAt, cutoff)

	stale, err = ListStaleReservedBillingLedgers("request", cutoff, 10)
	require.NoError(t, err)
	require.Empty(t, stale)

	refunded, err := RefundStaleReservedBillingLedger(
		observed.ID,
		"request",
		observed.Version,
		cutoff,
		"stale snapshot",
	)
	require.NoError(t, err)
	require.False(t, refunded)

	var stored BillingLedger
	require.NoError(t, DB.First(&stored, reservation.Ledger.ID).Error)
	require.Equal(t, BillingLedgerStateReserved, stored.State)
	require.EqualValues(t, 450, stored.AppliedQuota)
	assertLedgerBalances(t, fixture, 550, 0, 450, 0)
}

func TestStaleReservedBillingLedgerDoesNotUndoCompletedSettlement(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-stale-settle-race", 400)

	cutoff := time.Now().Unix() - 60
	staleAt := cutoff - 1
	require.NoError(t, DB.Model(&BillingLedger{}).
		Where("id = ?", reservation.Ledger.ID).
		UpdateColumns(map[string]any{
			"created_at": staleAt,
			"updated_at": staleAt,
		}).Error)

	stale, err := ListStaleReservedBillingLedgers("request", cutoff, 10)
	require.NoError(t, err)
	require.Len(t, stale, 1)
	observed := stale[0]

	settled, err := SettleBillingLedger(reservation.Ledger.ID, 300)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)

	refunded, err := RefundStaleReservedBillingLedger(
		observed.ID,
		"request",
		observed.Version,
		cutoff,
		"stale snapshot must not win",
	)
	require.NoError(t, err)
	require.False(t, refunded)

	var stored BillingLedger
	require.NoError(t, DB.First(&stored, reservation.Ledger.ID).Error)
	require.Equal(t, BillingLedgerStateSettled, stored.State)
	require.EqualValues(t, 300, stored.AppliedQuota)
	assertLedgerBalances(t, fixture, 700, 300, 300, 300)
}

func TestStaleReservedBillingLedgerRefundsUnchangedReservation(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	reservation := reserveWalletLedger(t, fixture, "req-stale-refund", 400)

	cutoff := time.Now().Unix() - 60
	staleAt := cutoff - 1
	require.NoError(t, DB.Model(&BillingLedger{}).
		Where("id = ?", reservation.Ledger.ID).
		UpdateColumns(map[string]any{
			"created_at": staleAt,
			"updated_at": staleAt,
		}).Error)

	stale, err := ListStaleReservedBillingLedgers("request", cutoff, 10)
	require.NoError(t, err)
	require.Len(t, stale, 1)
	observed := stale[0]

	refunded, err := RefundStaleReservedBillingLedger(
		observed.ID,
		"request",
		observed.Version,
		cutoff,
		"reserved request expired before settlement",
	)
	require.NoError(t, err)
	require.True(t, refunded)

	refunded, err = RefundStaleReservedBillingLedger(
		observed.ID,
		"request",
		observed.Version,
		cutoff,
		"duplicate stale delivery",
	)
	require.NoError(t, err)
	require.False(t, refunded)

	var stored BillingLedger
	require.NoError(t, DB.First(&stored, reservation.Ledger.ID).Error)
	require.Equal(t, BillingLedgerStateRefunded, stored.State)
	require.Zero(t, stored.AppliedQuota)
	assertLedgerBalances(t, fixture, 1000, 0, 0, 0)
}
