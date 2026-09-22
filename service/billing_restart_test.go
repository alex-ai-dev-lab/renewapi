package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBillingReconcilerSurvivesProcessRestart(t *testing.T) {
	for _, mode := range []string{BillingLedgerModeShadow, BillingLedgerModeEnforce} {
		for _, scenario := range []string{"settle", "refund", "orphan", "uncommitted"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				dsn := filepath.Join(t.TempDir(), "restart.db")
				binary, err := os.Executable()
				require.NoError(t, err)
				run := func(phase string) ([]byte, error) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					command := exec.CommandContext(ctx, binary, "-test.run=^TestBillingRestartWorker$", "-test.v")
					for _, entry := range os.Environ() {
						key := strings.SplitN(entry, "=", 2)[0]
						if strings.HasPrefix(key, "BILLING_TEST_") || key == "BILLING_LEDGER_MODE" || strings.HasPrefix(key, "RENEWAPI_RESTART_") {
							continue
						}
						command.Env = append(command.Env, entry)
					}
					command.Env = append(command.Env, "BILLING_TEST_DRIVER=sqlite", "BILLING_TEST_DSN="+dsn,
						"BILLING_LEDGER_MODE="+mode, "RENEWAPI_RESTART_PHASE="+phase, "RENEWAPI_RESTART_SCENARIO="+scenario)
					return command.CombinedOutput()
				}
				output, err := run("crash")
				var exitError *exec.ExitError
				require.ErrorAs(t, err, &exitError, string(output))
				require.Equal(t, 23, exitError.ExitCode(), string(output))
				output, err = run("recover")
				require.NoError(t, err, string(output))
			})
		}
	}
}

// TestBillingRestartWorker 仅由上面的测试启动；异常退出跳过 defer 与内存清理。
func TestBillingRestartWorker(t *testing.T) {
	phase := os.Getenv("RENEWAPI_RESTART_PHASE")
	if phase == "" {
		t.Skip("由进程重启测试调用")
	}
	mode, scenario := os.Getenv("BILLING_LEDGER_MODE"), os.Getenv("RENEWAPI_RESTART_SCENARIO")
	if phase == "crash" {
		ctx, session := newBillingReliabilitySession(t, mode, BillingSourceWallet)
		switch scenario {
		case "settle", "refund":
			failBillingTokenWrites(t)
			if scenario == "settle" {
				require.Error(t, session.Settle(200))
			} else {
				session.Refund(ctx)
			}
			ledger, err := model.GetBillingLedger(session.ledgerID)
			require.NoError(t, err)
			require.Equal(t, model.BillingLedgerStateReconcileRequired, ledger.State)
			require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", ledger.ID).Update("next_retry_at", 0).Error)
		case "orphan", "uncommitted":
			require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("id = ?", session.ledgerID).
				Update("updated_at", time.Now().Unix()-21601).Error)
			if scenario == "uncommitted" {
				tx := model.DB.Begin()
				require.NoError(t, tx.Error)
				require.NoError(t, tx.Model(&model.User{}).Where("id = ?", 401).Update("quota", 1).Error)
			}
		default:
			t.Fatal("未知重启场景")
		}
		os.Exit(23)
	}
	require.Equal(t, "recover", phase)
	_, err := ReconcileBillingOnce(context.Background(), 100)
	require.NoError(t, err)
	_, err = ReconcileBillingOnce(context.Background(), 100)
	require.NoError(t, err)
	var ledger model.BillingLedger
	require.NoError(t, model.DB.Where("request_id = ?", "billing-reliability-request").First(&ledger).Error)
	expected := 0
	if scenario == "settle" {
		expected = 200
		require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	} else {
		require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
	}
	require.Equal(t, 1000-expected, getUserQuota(t, 401))
	require.Equal(t, 1000-expected, getTokenRemainQuota(t, 401))
	require.Equal(t, expected, getTokenUsedQuota(t, 401))
	var user model.User
	require.NoError(t, model.DB.First(&user, 401).Error)
	if mode == BillingLedgerModeEnforce {
		require.Equal(t, expected, user.UsedQuota)
	}
	var pending int64
	require.NoError(t, model.DB.Model(&model.BillingLedger{}).Where("state = ?", model.BillingLedgerStateReconcileRequired).Count(&pending).Error)
	require.Zero(t, pending)
}
