package common

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

type blockedAttemptBody struct {
	ctx     context.Context
	started chan struct{}
	closes  int
}

func (b *blockedAttemptBody) Read([]byte) (int, error) {
	close(b.started)
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *blockedAttemptBody) Close() error {
	if b.ctx.Err() == nil {
		panic("close before cancellation")
	}
	b.closes++
	return io.ErrUnexpectedEOF
}

func TestCancelOnCloseBodyJoinsReadAndPreservesParent(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	ctx, cancel := context.WithCancel(parent)
	source := &blockedAttemptBody{ctx: ctx, started: make(chan struct{})}
	body := NewCancelOnCloseBody(source, cancel)
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _ = body.Read(make([]byte, 1)) }()
	<-source.started
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := body.Close(); err != io.ErrUnexpectedEOF {
					t.Errorf("close = %v", err)
				}
			}()
		}
		wg.Wait()
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close blocked behind upstream read")
	}
	<-readDone
	if source.closes != 1 || parent.Err() != nil {
		t.Fatalf("closes=%d parent=%v", source.closes, parent.Err())
	}
	if _, err := body.Read(nil); err != http.ErrBodyReadAfterClose {
		t.Fatalf("read after close: %v", err)
	}
}
