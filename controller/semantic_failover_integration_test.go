package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/stretchr/testify/require"
)

func TestSemanticFailoverFaultInjection(t *testing.T) {
	for index, fault := range []string{"401", "403", "429", "500", "502", "503", "refused", "tls", "empty", "created_eof", "malformed", "failed", "keepalive_stall", "client_cancel", "committed_eof"} {
		t.Run(fault, func(t *testing.T) {
			db := setupEmptyStreamRecoveryDB(t)
			seedRecoveryPrincipal(t, db)
			id := 7000 + index*2
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var callsB atomic.Int32
			serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				callsB.Add(1)
				writeFailoverSuccess(w)
			}))
			t.Cleanup(serverB.Close)
			serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var code int
				if _, err := fmt.Sscan(fault, &code); err == nil {
					w.WriteHeader(code)
					_, _ = io.WriteString(w, `{"error":{"message":"private-upstream-error","type":"upstream_error"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if fault == "empty" {
					return
				}
				_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_failed\"}}\n\n")
				switch fault {
				case "malformed":
					_, _ = io.WriteString(w, "data: {invalid-json\n\n")
				case "failed":
					_, _ = io.WriteString(w, "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"private-upstream-error\",\"type\":\"server_error\"}}}\n\n")
				case "keepalive_stall":
					ticker := time.NewTicker(100 * time.Millisecond)
					defer ticker.Stop()
					for {
						w.(http.Flusher).Flush()
						select {
						case <-r.Context().Done():
							return
						case <-ticker.C:
							_, _ = io.WriteString(w, ": keepalive\n\n")
						}
					}
				case "client_cancel":
					cancel()
				case "committed_eof":
					_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial-from-a\"}\n\n")
				}
			}))
			t.Cleanup(serverA.Close)
			baseA := serverA.URL
			if fault == "refused" {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				baseA = "http://" + listener.Addr().String()
				require.NoError(t, listener.Close())
			}
			if fault == "tls" {
				tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeFailoverSuccess(w) }))
				t.Cleanup(tlsServer.Close)
				baseA = tlsServer.URL
			}
			for _, channel := range []*model.Channel{
				recoveryIntegrationChannel(id, "failure-a", baseA, 100, failoverTestModel),
				recoveryIntegrationChannel(id+1, "success-b", serverB.URL, 90, failoverTestModel),
			} {
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			model.InitChannelCache()
			previousTimeout := common.RelayFirstByteTimeout
			common.RelayFirstByteTimeout = 15
			t.Cleanup(func() { common.RelayFirstByteTimeout = previousTimeout })
			router, used, _ := recoveryIntegrationRouter("semantic-fault-" + fault)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"recovery-integration-model","input":"hi","stream":true}`)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			started := time.Now()
			router.ServeHTTP(recorder, req)
			if fault == "committed_eof" || fault == "client_cancel" {
				require.Zero(t, callsB.Load())
				require.Len(t, *used, 1)
			} else {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.EqualValues(t, 1, callsB.Load())
				require.Len(t, *used, 2)
				require.Contains(t, recorder.Body.String(), "winner")
				require.NotContains(t, recorder.Body.String(), "resp_failed")
				require.NotContains(t, recorder.Body.String(), "private-upstream-error")
			}
			if fault == "keepalive_stall" {
				require.GreaterOrEqual(t, time.Since(started), 15*time.Second)
				require.Less(t, time.Since(started), 20*time.Second)
			}
			perfmetrics.WaitForPendingSamples()
		})
	}
}
