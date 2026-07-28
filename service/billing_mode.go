package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	BillingLedgerModeOff     = "off"
	BillingLedgerModeShadow  = "shadow"
	BillingLedgerModeEnforce = "enforce"
)

func BillingLedgerMode() string {
	mode := strings.ToLower(strings.TrimSpace(common.GetEnvOrDefaultString("BILLING_LEDGER_MODE", BillingLedgerModeShadow)))
	switch mode {
	case BillingLedgerModeOff, BillingLedgerModeShadow, BillingLedgerModeEnforce:
		return mode
	default:
		common.SysLog("invalid BILLING_LEDGER_MODE, falling back to shadow: " + mode)
		return BillingLedgerModeShadow
	}
}

func ledgerModeOwnsBalances(mode string) bool {
	return mode == BillingLedgerModeEnforce
}
