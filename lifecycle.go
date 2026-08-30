package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"golang.org/x/sync/errgroup"
)

const goPoolDrainPollInterval = 5 * time.Millisecond

type backgroundWorkers struct {
	cancel  context.CancelFunc
	group   *errgroup.Group
	ctx     context.Context
	once    sync.Once
	done    chan struct{}
	waitErr error
}

func newBackgroundWorkers() *backgroundWorkers {
	base, cancel := context.WithCancel(context.Background())
	group, ctx := errgroup.WithContext(base)
	return &backgroundWorkers{cancel: cancel, group: group, ctx: ctx, done: make(chan struct{})}
}

func (w *backgroundWorkers) Go(name string, run func(context.Context)) {
	w.group.Go(func() error {
		if run == nil {
			return fmt.Errorf("worker %s has no run function", name)
		}
		run(w.ctx)
		return nil
	})
}

func waitForGoPoolIdle(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if gopool.WorkerCount() == 0 {
		return nil
	}
	ticker := time.NewTicker(goPoolDrainPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if gopool.WorkerCount() == 0 {
				return nil
			}
		}
	}
}

func (w *backgroundWorkers) Stop(ctx context.Context) error {
	w.cancel()
	w.once.Do(func() {
		go func() {
			w.waitErr = w.group.Wait()
			close(w.done)
		}()
	})
	select {
	case <-w.done:
		if w.waitErr != nil {
			return w.waitErr
		}
		if err := waitForGoPoolIdle(ctx); err != nil {
			return fmt.Errorf("drain pooled work: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
