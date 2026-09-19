package main

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stretchr/testify/require"
)

func TestBackgroundWorkersStopWithinFiveSeconds(t *testing.T) {
	workers := newBackgroundWorkers()
	started := make(chan struct{})
	workers.Go("blocking-worker", func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	})
	<-started

	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	startedAt := time.Now()
	require.NoError(t, workers.Stop(deadline))
	require.Less(t, time.Since(startedAt), time.Second)
	require.NoError(t, workers.Stop(deadline))
}

func TestBackgroundWorkersStopHonorsDeadline(t *testing.T) {
	workers := newBackgroundWorkers()
	release := make(chan struct{})
	workers.Go("stuck-worker", func(context.Context) { <-release })
	t.Cleanup(func() { close(release) })

	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, workers.Stop(deadline), context.DeadlineExceeded)
}

func TestBackgroundWorkersStopWaitsForGoPool(t *testing.T) {
	workers := newBackgroundWorkers()
	started := make(chan struct{})
	release := make(chan struct{})
	gopool.Go(func() {
		close(started)
		<-release
	})
	<-started

	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() {
		stopped <- workers.Stop(deadline)
	}()

	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before pooled work completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-stopped)
	require.Zero(t, gopool.WorkerCount())
}
