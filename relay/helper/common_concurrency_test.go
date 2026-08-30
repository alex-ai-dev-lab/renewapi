package helper

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type overlapDetectWriter struct {
	header  http.Header
	active  atomic.Int32
	overlap atomic.Bool
}

func (w *overlapDetectWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *overlapDetectWriter) WriteHeader(int) {}

func (w *overlapDetectWriter) Write(p []byte) (int, error) {
	if w.active.Add(1) != 1 {
		w.overlap.Store(true)
	}
	defer w.active.Add(-1)
	time.Sleep(time.Millisecond)
	return len(p), nil
}

func (w *overlapDetectWriter) Flush() {
	if w.active.Add(1) != 1 {
		w.overlap.Store(true)
	}
	defer w.active.Add(-1)
	time.Sleep(time.Millisecond)
}

func TestStreamWritesAreSerializedAcrossPingAndBusinessData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := &overlapDetectWriter{}
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	const calls = 24
	start := make(chan struct{})
	errCh := make(chan error, calls)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				errCh <- PingData(ctx)
				return
			}
			errCh <- StringData(ctx, "chunk")
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	require.False(t, writer.overlap.Load(), "stream response writer was used concurrently")
}
