package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func ReconcileBillingOnce(ctx context.Context, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reservedTimeout := common.GetEnvOrDefault("BILLING_RESERVED_TASK_TIMEOUT_SECONDS", 900)
	if reservedTimeout < 60 {
		reservedTimeout = 60
	}
	stale, err := model.ListStaleReservedTaskLedgers(time.Now().Unix()-int64(reservedTimeout), limit)
	if err != nil {
		return 0, err
	}
	resolved := 0
	for i := range stale {
		if err := ctx.Err(); err != nil {
			return resolved, err
		}
		ledger := stale[i]
		if _, err := model.RefundBillingLedger(ledger.ID, "pending upstream task submission expired"); err != nil {
			_ = model.MarkBillingLedgerForReconcile(ledger.ID, model.BillingLedgerDesiredRefund, 0, err)
			continue
		}
		model.InvalidateBillingBalanceCaches(ledger.UserID, ledger.TokenID)
		resolved++
	}
	requestTimeout := common.GetEnvOrDefault("BILLING_RESERVED_REQUEST_TIMEOUT_SECONDS", 21600)
	if requestTimeout < 300 {
		requestTimeout = 300
	}
	orphanedRequests, err := model.ListStaleReservedBillingLedgers("request", time.Now().Unix()-int64(requestTimeout), limit)
	if err != nil {
		return resolved, err
	}
	for i := range orphanedRequests {
		if err := ctx.Err(); err != nil {
			return resolved, err
		}
		ledger := orphanedRequests[i]
		if _, err := model.RefundBillingLedger(ledger.ID, "reserved request expired before settlement"); err != nil {
			_ = model.MarkBillingLedgerForReconcile(ledger.ID, model.BillingLedgerDesiredRefund, 0, err)
			continue
		}
		model.InvalidateBillingBalanceCaches(ledger.UserID, ledger.TokenID)
		resolved++
	}

	ledgers, err := model.ListBillingLedgersForReconcile(limit)
	if err != nil {
		return resolved, err
	}
	for i := range ledgers {
		if err := ctx.Err(); err != nil {
			return resolved, err
		}
		ledger := ledgers[i]
		var reconcileErr error
		switch ledger.DesiredState {
		case model.BillingLedgerDesiredSettle:
			_, reconcileErr = model.SettleBillingLedger(ledger.ID, ledger.DesiredQuota)
		case model.BillingLedgerDesiredRefund:
			_, reconcileErr = model.RefundBillingLedger(ledger.ID, ledger.LastError)
		default:
			reconcileErr = fmt.Errorf("unknown desired billing state %q", ledger.DesiredState)
		}
		if reconcileErr != nil {
			_ = model.MarkBillingLedgerForReconcile(ledger.ID, ledger.DesiredState, ledger.DesiredQuota, reconcileErr)
			continue
		}
		model.InvalidateBillingBalanceCaches(ledger.UserID, ledger.TokenID)
		resolved++
	}
	processed, outboxErr := model.ProcessBillingOutbox(limit)
	if outboxErr != nil {
		return resolved, outboxErr
	}
	return resolved + processed, nil
}

func StartBillingReconciler(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	intervalSeconds := common.GetEnvOrDefault("BILLING_RECONCILE_INTERVAL_SECONDS", 15)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	batchSize := common.GetEnvOrDefault("BILLING_RECONCILE_BATCH_SIZE", 100)
	if batchSize < 1 || batchSize > 500 {
		batchSize = 100
	}
	go func() {
		run := func() {
			if _, err := ReconcileBillingOnce(ctx, batchSize); err != nil && ctx.Err() == nil {
				common.SysLog("billing reconciler error: " + err.Error())
			}
		}
		run()
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
