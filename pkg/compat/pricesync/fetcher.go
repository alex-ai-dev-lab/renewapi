// Package pricesync provides the entry point for official model price sync.
// The actual fetching/syncing logic lives in controller/official_price_sync.go
// for now; this package centralizes the startup hook for cleanliness.
package pricesync

import (
	"context"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
)

// Start launches the official models.dev price sync task by default.
// Set OFFICIAL_PRICE_SYNC_ENABLED=false to disable the scheduler explicitly.
func Start() {
	if !autoSyncEnabled() {
		common.SysLog("official models.dev price sync auto task disabled by OFFICIAL_PRICE_SYNC_ENABLED")
		return
	}
	controller.StartOfficialPriceSyncTask()
}

func Run(ctx context.Context) {
	if !autoSyncEnabled() {
		common.SysLog("official models.dev price sync auto task disabled by OFFICIAL_PRICE_SYNC_ENABLED")
		return
	}
	controller.RunOfficialPriceSyncTask(ctx)
}

func autoSyncEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("OFFICIAL_PRICE_SYNC_ENABLED")))
	return value != "0" && value != "false" && value != "no" && value != "off"
}
