package channel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TestHTTPRequestCloseCancelsOnlyCurrentAttempt(t *testing.T) {
	service.InitHttpClient()
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "stream"}[stream], func(t *testing.T) {
			cancelled := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(cancelled)
			}))
			defer upstream.Close()
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(parent)
			req, _ := http.NewRequest(http.MethodGet, upstream.URL, nil)
			info := &relaycommon.RelayInfo{IsStream: stream, ChannelMeta: &relaycommon.ChannelMeta{}}
			resp, err := DoRequest(c, req, info)
			if err != nil {
				t.Fatal(err)
			}
			readDone := make(chan struct{})
			go func() { defer close(readDone); _, _ = io.ReadAll(resp.Body) }()
			closed := make(chan struct{})
			go func() { defer close(closed); _ = resp.Body.Close() }()
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("close stalled")
			}
			select {
			case <-cancelled:
			case <-time.After(time.Second):
				t.Fatal("upstream not cancelled")
			}
			<-readDone
			if parent.Err() != nil {
				t.Fatal("parent cancelled; retry would fail")
			}
		})
	}
}
