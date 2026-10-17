package common

import (
	"context"
	"io"
	"net/http"
	"sync"
)

// NewCancelOnCloseBody interrupts a pending upstream read before closing the
// body. The cancellation must belong to this attempt, never the parent request.
func NewCancelOnCloseBody(body io.ReadCloser, cancel context.CancelFunc) io.ReadCloser {
	return &cancelOnCloseBody{body: body, cancel: cancel}
}

type cancelOnCloseBody struct {
	body      io.ReadCloser
	cancel    context.CancelFunc
	readMu    sync.Mutex
	closeOnce sync.Once
	closed    bool
	closeErr  error
}

func (b *cancelOnCloseBody) Read(p []byte) (int, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	if b.closed {
		return 0, http.ErrBodyReadAfterClose
	}
	return b.body.Read(p)
}

func (b *cancelOnCloseBody) Close() error {
	b.closeOnce.Do(func() {
		b.cancel()
		b.readMu.Lock()
		defer b.readMu.Unlock()
		b.closed = true
		b.closeErr = b.body.Close()
	})
	return b.closeErr
}
