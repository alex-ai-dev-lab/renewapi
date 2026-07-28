// Package scheduler provides the entry point for per-channel test scheduling.
// The actual scheduling logic lives in controller/channel_test_scheduler.go;
// this package centralizes the startup hook for cleanliness.
package scheduler

import (
	"context"
	"github.com/QuantumNous/new-api/controller"
)

// Start launches the channel test scheduler runner.
// This is the unified entry point for main.go.
func Start() {
	controller.StartChannelUpstreamModelUpdateTask()
}

func Run(ctx context.Context) {
	controller.RunChannelUpstreamModelUpdateTask(ctx)
}
