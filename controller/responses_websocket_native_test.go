package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func writeResponsesWSNativeSuccess(conn *websocket.Conn, stream, id, text string) error {
	for _, event := range []any{
		map[string]any{"type": "response.created", "stream_id": stream, "response": map[string]any{"id": id, "status": "in_progress"}},
		map[string]any{"type": "response.output_text.delta", "stream_id": stream, "delta": text},
		map[string]any{"type": "response.completed", "stream_id": stream, "response": map[string]any{"id": id, "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": text}}}}, "usage": map[string]int{"input_tokens": 11, "output_tokens": 3, "total_tokens": 14}}},
	} {
		data, _ := common.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
	}
	return nil
}

func TestResponsesWSNativeReuseContinuationIdleReaderAndScope(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	var handshakes atomic.Int32
	var mu sync.Mutex
	var requests [][]byte
	ponged := make(chan struct{}, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handshakes.Add(1)
		conn.SetPongHandler(func(string) error { ponged <- struct{}{}; return nil })
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			requests = append(requests, data)
			n := len(requests)
			mu.Unlock()
			if writeResponsesWSNativeSuccess(conn, gjson.GetBytes(data, "stream_id").String(), fmt.Sprintf("resp_native_%d", n), "hello") != nil {
				return
			}
			_ = conn.WriteControl(websocket.PingMessage, []byte("idle"), time.Now().Add(time.Second))
		}
	}))
	t.Cleanup(upstream.Close)
	channel := recoveryIntegrationChannel(8501, "native", upstream.URL, 100, failoverTestModel)
	channel.SetSetting(dto.ChannelSettings{ResponsesWebSocket: true})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
	for n := 1; n <= 3; n++ {
		if n == 3 {
			require.NoError(t, db.Model(channel).Update("key", "sk-rotated").Error)
			model.InitChannelCache()
		}
		event := map[string]any{"type": "response.create", "stream_id": "main", "event_id": "repeated-client-id", "model": failoverTestModel, "input": "hi", "store": false}
		if n > 1 {
			event["previous_response_id"] = fmt.Sprintf("resp_native_%d", n-1)
		}
		sendResponsesWSTest(t, conn, event)
		_, terminal := readResponsesWSTerminal(t, conn)
		require.Equal(t, "response.completed", terminal.Get("type").String(), terminal.Raw)
		select {
		case <-ponged:
		case <-time.After(2 * time.Second):
			t.Fatal("闲置 reader 未处理上游 ping")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 3)
	require.EqualValues(t, 2, handshakes.Load())
	require.Equal(t, "response.create", gjson.GetBytes(requests[0], "type").String())
	require.False(t, gjson.GetBytes(requests[0], "stream").Exists())
	require.Equal(t, "resp_native_1", gjson.GetBytes(requests[1], "previous_response_id").String())
	require.Len(t, gjson.GetBytes(requests[1], "input").Array(), 1)
	require.NotEqual(t, gjson.GetBytes(requests[0], "event_id").String(), gjson.GetBytes(requests[1], "event_id").String())
	// 密钥变化后必须重建连接并发送完整上下文，不能引用旧连接中的 store=false 状态。
	require.False(t, gjson.GetBytes(requests[2], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(requests[2], "input").Array(), 5)
	var ledgers []model.BillingLedger
	require.NoError(t, db.Find(&ledgers).Error)
	require.Len(t, ledgers, 3)
	for _, ledger := range ledgers {
		require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
	}
}

func TestResponsesWSNativeFailureBeforeAndAfterCommit(t *testing.T) {
	for index, failure := range []string{"handshake", "failed", "wrong_lane", "wrong_event", "truncated", "late_terminal"} {
		t.Run(failure, func(t *testing.T) {
			db := setupEmptyStreamRecoveryDB(t)
			var fallback atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !websocket.IsWebSocketUpgrade(r) {
					fallback.Add(1)
					responsesWSTestSuccess(w, "resp_fallback", "fallback")
					return
				}
				if failure == "handshake" {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"message":"private native quota","type":"private_plugin"}}`)
					return
				}
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				_, request, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if failure == "late_terminal" {
					_ = writeResponsesWSNativeSuccess(conn, "main", "resp_first", "first")
					_, _, err = conn.ReadMessage()
					if err == nil {
						_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","stream_id":"main","response":{"id":"resp_first","status":"completed","output":[]}}`))
					}
					return
				}
				if failure == "wrong_lane" || failure == "wrong_event" {
					payload := map[string]any{"type": "error", "stream_id": "main", "event_id": gjson.GetBytes(request, "event_id").String(), "error": map[string]string{"message": "unrelated private error"}}
					if failure == "wrong_lane" {
						payload["stream_id"] = "other"
					} else {
						payload["event_id"] = "old_event"
					}
					data, _ := common.Marshal(payload)
					_ = conn.WriteMessage(websocket.TextMessage, data)
					_ = writeResponsesWSNativeSuccess(conn, "main", "resp_right", "right")
					_, _, _ = conn.ReadMessage()
					return
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created","stream_id":"main","response":{"id":"resp_partial"}}`))
				if failure == "failed" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.failed","stream_id":"main","response":{"id":"resp_partial","error":{"message":"private native failure","type":"secret_plugin"}}}`))
					return
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_text.delta","stream_id":"main","delta":"partial"}`))
			}))
			t.Cleanup(upstream.Close)
			for i := range 2 {
				channel := recoveryIntegrationChannel(8600+index*10+i, "native-fault", upstream.URL, int64(100-i), failoverTestModel)
				channel.SetSetting(dto.ChannelSettings{ResponsesWebSocket: i == 0})
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			conn := dialResponsesWSTest(t, responsesWSTestGateway(t, db))
			event := map[string]any{"type": "response.create", "stream_id": "main", "event_id": "client_event", "model": failoverTestModel, "input": "hi", "store": false}
			sendResponsesWSTest(t, conn, event)
			events, terminal := readResponsesWSTerminal(t, conn)
			if failure == "late_terminal" {
				require.Equal(t, "resp_first", terminal.Get("response.id").String(), terminal.Raw)
				event["previous_response_id"] = "resp_first"
				sendResponsesWSTest(t, conn, event)
				events, terminal = readResponsesWSTerminal(t, conn)
			}
			for _, data := range events {
				require.NotContains(t, data.Raw, "private")
				require.NotContains(t, data.Raw, "secret_plugin")
			}
			switch failure {
			case "wrong_lane", "wrong_event":
				require.Equal(t, "resp_right", terminal.Get("response.id").String(), terminal.Raw)
				require.Zero(t, fallback.Load())
			case "truncated":
				require.Equal(t, "error", terminal.Get("type").String(), terminal.Raw)
				require.Equal(t, types.PublicModelCapacityMessage, terminal.Get("error.message").String())
				require.Equal(t, "resp_partial", terminal.Get("response_id").String())
				require.Zero(t, fallback.Load())
			default:
				require.Equal(t, "resp_fallback", terminal.Get("response.id").String(), terminal.Raw)
				require.EqualValues(t, 1, fallback.Load())
			}
		})
	}
}
