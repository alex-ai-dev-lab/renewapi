package main

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

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
		return w.waitErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
