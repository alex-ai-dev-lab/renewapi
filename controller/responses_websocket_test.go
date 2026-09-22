package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func responsesWSTestGateway(t *testing.T, db *gorm.DB) string {
	t.Helper()
	initModelListColumnNames(t)
	require.NoError(t, i18n.Init())
	model.DB, model.LOG_DB = db, db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionPreConsumeRecord{}))
	seedRecoveryPrincipal(t, db)
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 1).Update("key", "wsintegration").Error)
	model.InitChannelCache()
	var active atomic.Int32
	engine := gin.New()
	engine.GET("/v1/responses", middleware.TokenAuth(), func(c *gin.Context) {
		active.Add(1)
		defer active.Add(-1)
		ResponsesWebSocket(c)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(func() {
		server.Close()
		require.Eventually(t, func() bool { return active.Load() == 0 }, 5*time.Second, 10*time.Millisecond)
		perfmetrics.WaitForPendingSamples()
	})
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses"
}

func dialResponsesWSTest(t *testing.T, endpoint string) *websocket.Conn {
	t.Helper()
	conn, response, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Authorization": {"Bearer sk-wsintegration"}})
	if err != nil && response != nil {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("WebSocket handshake: %v, %s", err, body)
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func sendResponsesWSTest(t *testing.T, conn *websocket.Conn, value any) {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, data))
}

func readResponsesWSTest(t *testing.T, conn *websocket.Conn) gjson.Result {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.True(t, gjson.ValidBytes(data), string(data))
	return gjson.ParseBytes(data)
}

func readResponsesWSTerminal(t *testing.T, conn *websocket.Conn) ([]gjson.Result, gjson.Result) {
	t.Helper()
	var events []gjson.Result
	for range 50 {
		event := readResponsesWSTest(t, conn)
		events = append(events, event)
		switch event.Get("type").String() {
		case "response.completed", "response.incomplete", "response.cancelled", "error":
			return events, event
		}
	}
	t.Fatal("未收到 WebSocket terminal")
	return nil, gjson.Result{}
}

func responsesWSTestSuccess(w http.ResponseWriter, id, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range []any{
		map[string]any{"type": "response.created", "response": map[string]any{"id": id, "status": "in_progress"}},
		map[string]any{"type": "response.output_text.delta", "delta": text},
		map[string]any{"type": "response.completed", "response": map[string]any{"id": id, "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": text}}}}, "usage": map[string]int{"input_tokens": 11, "output_tokens": 3, "total_tokens": 14}}},
	} {
		data, _ := common.Marshal(event)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
}

func TestResponsesWSHTTPContinuationAndPerTurnBilling(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	var requests [][]byte
	var mu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, body)
		n := len(requests)
		mu.Unlock()
		responsesWSTestSuccess(w, fmt.Sprintf("resp_%d", n), "hello")
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8101, "http-ws", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	for n := 1; n <= 2; n++ {
		event := map[string]any{"type": "response.create", "stream_id": "main", "event_id": fmt.Sprint(n), "model": failoverTestModel, "input": "hi", "store": false}
		if n == 2 {
			event["previous_response_id"] = "resp_1"
		}
		sendResponsesWSTest(t, conn, event)
		events, terminal := readResponsesWSTerminal(t, conn)
		require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
		for _, event := range events {
			require.Equal(t, "main", event.Get("stream_id").String())
		}
		var ledgers []model.BillingLedger
		require.NoError(t, db.Order("id").Find(&ledgers).Error)
		require.Len(t, ledgers, n)
		for _, ledger := range ledgers {
			require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 2)
	for _, request := range requests {
		require.False(t, gjson.GetBytes(request, "stream_id").Exists())
		require.False(t, gjson.GetBytes(request, "event_id").Exists())
	}
	require.False(t, gjson.GetBytes(requests[1], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(requests[1], "input").Array(), 3)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 2, user.RequestCount)
}

func TestResponsesWSFailoverAndCapacityCorrelation(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			db := setupEmptyStreamRecoveryDB(t)
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if n == 6 && success {
					responsesWSTestSuccess(w, "resp_success", "ok")
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_bad\"}}\n\n")
				_, _ = io.WriteString(w, "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"type\":\"secret_plugin\",\"message\":\"private-upstream-host api_key=secret\"}}}\n\n")
			}))
			t.Cleanup(upstream.Close)
			for i := range 7 {
				id := 8201 + i
				if success {
					id += 100
				}
				channel := recoveryIntegrationChannel(id, fmt.Sprint(i), upstream.URL, int64(100-i), failoverTestModel)
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
			sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "model": failoverTestModel, "input": "hi", "stream_id": "agent", "event_id": "event_1"})
			events, terminal := readResponsesWSTerminal(t, conn)
			require.EqualValues(t, 6, calls.Load())
			for _, event := range events {
				require.NotContains(t, event.Raw, "private-upstream-host")
				require.NotContains(t, event.Raw, "secret_plugin")
				require.NotContains(t, event.Raw, "resp_bad")
			}
			if success {
				require.Equal(t, "response.completed", terminal.Get("type").String())
			} else {
				require.Equal(t, "error", terminal.Get("type").String())
				require.Equal(t, "event_1", terminal.Get("event_id").String())
				require.Equal(t, "agent", terminal.Get("stream_id").String())
				require.Equal(t, types.PublicModelCapacityMessage, terminal.Get("error.message").String())
			}
		})
	}
}

func TestResponsesWSCancelLaneAndRevalidateToken(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	entered := make(chan struct{}, 1)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if gjson.GetBytes(body, "input.0.content").String() == "stall" {
			entered <- struct{}{}
			<-r.Context().Done()
			return
		}
		responsesWSTestSuccess(w, "resp_other", "other")
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8401, "cancel", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "stream_id": "slow", "model": failoverTestModel, "input": "stall"})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("未收到上游请求")
	}
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "stream_id": "fast", "model": failoverTestModel, "input": "hi"})
	_, terminal := readResponsesWSTerminal(t, conn)
	require.Equal(t, "fast", terminal.Get("stream_id").String())
	require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.cancel", "stream_id": "slow", "event_id": "cancel_1"})
	_, terminal = readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.cancelled", terminal.Get("type").String(), terminal.Raw)
	require.Equal(t, "cancel_1", terminal.Get("event_id").String())
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 1).Update("status", common.TokenStatusDisabled).Error)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "stream_id": "fast", "event_id": "blocked", "model": failoverTestModel, "input": "hi"})
	_, terminal = readResponsesWSTerminal(t, conn)
	require.Equal(t, "error", terminal.Get("type").String(), terminal.Raw)
	require.EqualValues(t, 401, terminal.Get("status").Int())
	require.Equal(t, "blocked", terminal.Get("event_id").String())
	require.EqualValues(t, 2, calls.Load())
	var ledgers []model.BillingLedger
	require.NoError(t, db.Find(&ledgers).Error)
	require.Len(t, ledgers, 2)
	states := []string{ledgers[0].State, ledgers[1].State}
	require.ElementsMatch(t, []string{model.BillingLedgerStateSettled, model.BillingLedgerStateRefunded}, states)
}
