package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func TestResponsesWSWarmupHistoryIsolationAndValidation(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	requests := make(chan []byte, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- body
		responsesWSTestSuccess(w, "resp_generated", "ok")
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8801, "warmup", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	endpoint := responsesWSTestGateway(t, db)
	conn := dialResponsesWSTest(t, endpoint)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "generate": false, "model": failoverTestModel, "input": "warm context", "store": false})
	events, terminal := readResponsesWSTerminal(t, conn)
	require.Len(t, events, 1)
	require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
	require.False(t, terminal.Get("stream_id").Exists())
	require.Empty(t, terminal.Get("response.output").Array())
	require.Zero(t, len(requests))
	var count int64
	require.NoError(t, db.Model(&model.BillingLedger{}).Count(&count).Error)
	require.Zero(t, count)
	previous := terminal.Get("response.id").String()
	other := dialResponsesWSTest(t, endpoint)
	sendResponsesWSTest(t, other, map[string]any{"type": "response.create", "model": failoverTestModel, "store": false, "previous_response_id": previous, "input": "next"})
	_, rejected := readResponsesWSTerminal(t, other)
	require.Equal(t, "previous_response_not_found", rejected.Get("error.code").String(), rejected.Raw)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "model": failoverTestModel, "store": false, "previous_response_id": previous, "input": "next"})
	_, terminal = readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
	body := <-requests
	require.Len(t, gjson.GetBytes(body, "input").Array(), 2)
	require.Contains(t, gjson.GetBytes(body, "input").Raw, "warm context")
	require.False(t, gjson.GetBytes(body, "previous_response_id").Exists())
	for _, invalid := range []struct {
		event any
		code  string
	}{
		{map[string]any{"type": "response.create", "stream_id": "invalid space"}, "invalid_stream_id"},
		{map[string]any{"type": "response.create", "stream_id": ""}, "invalid_stream_id"},
		{map[string]any{"type": "response.create", "stream_id": 9}, "invalid_stream_id"},
		{map[string]any{"type": "response.create", "input": false}, "invalid_request"},
		{map[string]any{"type": "response.create", "generate": 1}, "invalid_request"},
		{map[string]any{"type": "response.create", "background": true}, "invalid_request"},
		{map[string]any{"type": "response.cancel", "response_id": 3}, "invalid_request"},
	} {
		sendResponsesWSTest(t, conn, invalid.event)
		_, rejected = readResponsesWSTerminal(t, conn)
		require.Equal(t, invalid.code, rejected.Get("error.code").String(), rejected.Raw)
	}
	require.Zero(t, len(requests))
}

func TestResponsesWSFIFOQueueLimitAndDisconnectCancellation(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	entered := make(chan struct{})
	exited := make(chan struct{})
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(exited)
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8811, "queue", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	create := map[string]any{"type": "response.create", "stream_id": "same", "model": failoverTestModel, "input": "hi"}
	sendResponsesWSTest(t, conn, create)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("未收到首轮请求")
	}
	for range responsesWSMaxPending {
		sendResponsesWSTest(t, conn, create)
	}
	_, rejected := readResponsesWSTerminal(t, conn)
	require.Equal(t, "websocket_queue_full", rejected.Get("error.code").String(), rejected.Raw)
	require.EqualValues(t, 1, calls.Load())
	require.NoError(t, conn.Close())
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("客户端退出没有取消上游")
	}
	require.Eventually(t, func() bool {
		var ledger model.BillingLedger
		return db.First(&ledger).Error == nil && ledger.State == model.BillingLedgerStateRefunded
	}, 3*time.Second, 10*time.Millisecond)
	require.EqualValues(t, 1, calls.Load())
}

func TestResponsesWSLaneAndConnectionCapacityReleased(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	t.Setenv("RESPONSES_WS_MAX_CONNECTIONS_PER_TOKEN", "1")
	channel := recoveryIntegrationChannel(8821, "capacity", "http://127.0.0.1:1", 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	endpoint := responsesWSTestGateway(t, db)
	conn := dialResponsesWSTest(t, endpoint)
	for n := 0; n < responsesWSMaxStreams; n++ {
		sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "stream_id": fmt.Sprint(n), "model": failoverTestModel, "generate": false})
		_, terminal := readResponsesWSTerminal(t, conn)
		require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
	}
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "stream_id": "overflow", "model": failoverTestModel, "generate": false})
	_, terminal := readResponsesWSTerminal(t, conn)
	require.Equal(t, "websocket_stream_limit_reached", terminal.Get("error.code").String(), terminal.Raw)
	_, response, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Authorization": {"Bearer sk-wsintegration"}})
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	_ = response.Body.Close()
	require.NoError(t, conn.Close())
	var replacement *websocket.Conn
	require.Eventually(t, func() bool {
		var resp *http.Response
		replacement, resp, err = websocket.DefaultDialer.Dial(endpoint, http.Header{"Authorization": {"Bearer sk-wsintegration"}})
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return err == nil
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, replacement.Close())
}

func TestResponsesWSCombinedBodyLimitAndTokenModelRecheck(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	previousLimit := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 1
	t.Cleanup(func() { constant.MaxRequestBodyMB = previousLimit })
	channel := recoveryIntegrationChannel(8831, "limits", "http://127.0.0.1:1", 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	event := map[string]any{"type": "response.create", "model": failoverTestModel, "generate": false, "store": false, "input": strings.Repeat("x", 600_000)}
	sendResponsesWSTest(t, conn, event)
	_, first := readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.completed", first.Get("type").String(), first.Raw)
	event["previous_response_id"] = first.Get("response.id").String()
	sendResponsesWSTest(t, conn, event)
	_, rejected := readResponsesWSTerminal(t, conn)
	require.Equal(t, "request_too_large", rejected.Get("error.code").String(), rejected.Raw)
	require.EqualValues(t, 413, rejected.Get("status").Int())
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 1).Updates(map[string]any{"model_limits_enabled": true, "model_limits": "another-model"}).Error)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "model": failoverTestModel, "generate": false, "input": "hi"})
	_, rejected = readResponsesWSTerminal(t, conn)
	require.Equal(t, "error", rejected.Get("type").String(), rejected.Raw)
	require.EqualValues(t, 403, rejected.Get("status").Int())
}

func TestResponsesWSSettlementFailureWithholdsTerminalAndReconciles(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	common.LogConsumeEnabled = true
	var fail atomic.Bool
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("ws_settle_failure", func(tx *gorm.DB) {
		if fail.Load() && tx.Statement.Table == "tokens" {
			tx.AddError(errors.New("注入账本结算失败"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("ws_settle_failure") })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fail.Store(true)
		responsesWSTestSuccess(w, "resp_settle", "done")
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8841, "settle", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "model": failoverTestModel, "input": "hi"})
	events, terminal := readResponsesWSTerminal(t, conn)
	require.Equal(t, "error", terminal.Get("type").String(), terminal.Raw)
	require.Equal(t, types.PublicModelCapacityMessage, terminal.Get("error.message").String())
	for _, event := range events {
		require.NotEqual(t, "response.completed", event.Get("type").String())
	}
	var ledger model.BillingLedger
	require.NoError(t, db.First(&ledger).Error)
	require.Equal(t, model.BillingLedgerStateReconcileRequired, ledger.State)
	require.Equal(t, model.BillingLedgerDesiredSettle, ledger.DesiredState)
	fail.Store(false)
	require.NoError(t, db.Model(&model.BillingLedger{}).Where("id = ?", ledger.ID).Update("next_retry_at", 0).Error)
	_, err := service.ReconcileBillingOnce(context.Background(), 10)
	require.NoError(t, err)
	require.NoError(t, db.First(&ledger, ledger.ID).Error)
	require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	var logs []model.Log
	require.Eventually(t, func() bool { return db.Find(&logs).Error == nil && len(logs) > 0 }, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, types.PublicModelCapacityMessage, logs[0].Content)
	require.Contains(t, logs[0].Other, "billing_error")
}

func TestResponsesWSPerTurnConcurrencyRPMAndFIFO(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	entered := make(chan struct{})
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			close(entered)
			<-r.Context().Done()
			return
		}
		responsesWSTestSuccess(w, "resp_fifo", "next")
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8851, "fifo", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 1).Update("concurrency_limit", 1).Error)
	event := map[string]any{"type": "response.create", "stream_id": "first", "model": failoverTestModel, "input": "hi"}
	sendResponsesWSTest(t, conn, event)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("首轮未进入")
	}
	event["stream_id"] = "other"
	sendResponsesWSTest(t, conn, event)
	_, rejected := readResponsesWSTerminal(t, conn)
	require.EqualValues(t, 429, rejected.Get("status").Int(), rejected.Raw)
	require.Equal(t, "other", rejected.Get("stream_id").String())
	event["stream_id"] = "first"
	sendResponsesWSTest(t, conn, event)
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.cancel", "stream_id": "first"})
	_, cancelled := readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.cancelled", cancelled.Get("type").String(), cancelled.Raw)
	_, completed := readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.completed", completed.Get("type").String(), completed.Raw)
	require.EqualValues(t, 2, calls.Load())
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 1).Update("rpm_limit", 1).Error)
	event["generate"] = false
	sendResponsesWSTest(t, conn, event)
	_, completed = readResponsesWSTerminal(t, conn)
	require.Equal(t, "response.completed", completed.Get("type").String(), completed.Raw)
	sendResponsesWSTest(t, conn, event)
	_, rejected = readResponsesWSTerminal(t, conn)
	require.EqualValues(t, 429, rejected.Get("status").Int(), rejected.Raw)
	require.Contains(t, rejected.Get("error.message").String(), "RPM")
}

func TestResponsesWSShutdownDrainsBillingBeforeDatabaseClose(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	t.Cleanup(func() {
		responsesWSConnections.Lock()
		responsesWSConnections.stopping = false
		responsesWSConnections.Unlock()
	})
	entered := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8861, "shutdown", upstream.URL, 100, failoverTestModel)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	sendResponsesWSTest(t, conn, map[string]any{"type": "response.create", "model": failoverTestModel, "input": "hi"})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("未进入上游")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, ShutdownResponsesWebSockets(ctx))
	var ledger model.BillingLedger
	require.NoError(t, db.First(&ledger).Error)
	require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
	_, _, err := conn.ReadMessage()
	require.Error(t, err)
}
