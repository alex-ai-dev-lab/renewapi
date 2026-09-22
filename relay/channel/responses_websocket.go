package channel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

const responsesWSInputLimit = 16 << 20

var errResponsesWSUpstream = errors.New("invalid upstream Responses WebSocket sequence")

// 一个实例只属于一个客户端连接的一个 lane，绝不跨用户借用带会话状态的连接。
type ResponsesWSTransport struct {
	ctx    context.Context
	mu     sync.Mutex
	conn   *responsesWSUpstream
	closed bool
}

type responsesWSUpstream struct {
	conn        *websocket.Conn
	scope       [32]byte
	headers     http.Header
	mu          sync.Mutex
	active      *responsesWSUpstreamTurn
	closed      bool
	done        chan struct{}
	closeOnce   sync.Once
	lastID      string
	lastEventID string
	lastCount   int
	lastHash    [32]byte
}

type responsesWSUpstreamTurn struct {
	upstream                      *responsesWSUpstream
	ctx                           context.Context
	reader                        *io.PipeReader
	writer                        *io.PipeWriter
	stop                          func() bool
	closeOnce                     sync.Once
	streamID, eventID, responseID string
	input                         []json.RawMessage
	terminal, reusable            bool
	prefixCount                   int
	prefixHash                    [32]byte
}

func NewResponsesWSTransport(ctx context.Context) *ResponsesWSTransport {
	return &ResponsesWSTransport{ctx: ctx}
}

func (t *ResponsesWSTransport) Close() {
	t.mu.Lock()
	t.closed = true
	conn := t.conn
	t.conn = nil
	t.mu.Unlock()
	if conn != nil {
		conn.close(context.Canceled)
		<-conn.done
	}
}

func (t *ResponsesWSTransport) RoundTrip(req *http.Request, info *relaycommon.RelayInfo, execution *relaycommon.ResponsesWSExecution) (*http.Response, error) {
	defer req.Body.Close()
	body, err := io.ReadAll(io.LimitReader(req.Body, responsesWSInputLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > responsesWSInputLimit {
		return nil, errors.New("Responses WebSocket request exceeds the input limit")
	}
	var fields map[string]json.RawMessage
	if common.Unmarshal(body, &fields) != nil || fields == nil {
		return nil, errResponsesWSUpstream
	}
	var input []json.RawMessage
	_ = common.Unmarshal(fields["input"], &input)
	u := *req.URL
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return nil, errors.New("unsupported Responses WebSocket upstream scheme")
	}
	headers := req.Header.Clone()
	for _, name := range []string{"Content-Length", "Content-Type", "Connection", "Upgrade", "Transfer-Encoding", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Protocol"} {
		headers.Del(name)
	}
	if req.Host != "" {
		headers.Set("Host", req.Host)
	}
	// URL、渠道、认证、代理及 TLS 设置的任一变化都必须重新握手。
	scopeData, err := common.Marshal([]any{info.ChannelId, u.String(), headers, service.HTTPClientOptionsFromChannelSettings(info.ChannelSetting)})
	if err != nil {
		return nil, err
	}
	scope := sha256.Sum256(scopeData)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.ctx.Err() != nil {
		return nil, context.Canceled
	}
	conn := t.conn
	if conn != nil {
		conn.mu.Lock()
		usable := !conn.closed && conn.scope == scope
		conn.mu.Unlock()
		if !usable {
			conn.close(errResponsesWSUpstream)
			<-conn.done
			conn, t.conn = nil, nil
		}
	}
	if conn == nil {
		dialer, dialErr := service.NewWebSocketDialerWithChannelSettings(info.ChannelSetting)
		if dialErr != nil {
			return nil, dialErr
		}
		target, response, dialErr := dialer.DialContext(req.Context(), u.String(), headers)
		if dialErr != nil {
			if response != nil {
				// 将握手 HTTP 错误交给现有分类器和管理员错误链。
				return response, nil
			}
			return nil, dialErr
		}
		target.SetReadLimit(int64(helper.DefaultMaxScannerBufferSize))
		conn = &responsesWSUpstream{conn: target, scope: scope, headers: response.Header.Clone(), done: make(chan struct{})}
		t.conn = conn
		go conn.readLoop()
	}
	conn.mu.Lock()
	if conn.closed || conn.active != nil {
		conn.mu.Unlock()
		return nil, errors.New("Responses WebSocket upstream is unavailable or busy")
	}
	// 本地始终先验证完整上下文；只有实际发出的前缀完全相同，才缩成上游增量请求。
	if execution.PreviousResponseID != "" && execution.PreviousResponseID == conn.lastID && conn.lastCount > 0 && len(input) >= conn.lastCount &&
		responsesWSPrefixHash(input[:conn.lastCount]) == conn.lastHash && len(fields["previous_response_id"]) == 0 {
		fields["previous_response_id"], _ = common.Marshal(conn.lastID)
		fields["input"], _ = common.Marshal(input[conn.lastCount:])
	}
	for _, name := range []string{"stream", "background", "generate", "stream_id", "event_id"} {
		delete(fields, name)
	}
	fields["type"] = json.RawMessage(`"response.create"`)
	if execution.StreamID != "" {
		fields["stream_id"], _ = common.Marshal(execution.StreamID)
	}
	// 不使用客户端可重复的 event_id 区分上游轮次。
	eventID := "event_" + common.GetUUID()
	fields["event_id"], _ = common.Marshal(eventID)
	payload, marshalErr := common.Marshal(fields)
	if marshalErr != nil {
		conn.mu.Unlock()
		return nil, marshalErr
	}
	reader, writer := io.Pipe()
	turn := &responsesWSUpstreamTurn{upstream: conn, ctx: req.Context(), reader: reader, writer: writer,
		streamID: execution.StreamID, eventID: eventID, input: input}
	conn.active = turn
	conn.mu.Unlock()
	turn.stop = context.AfterFunc(req.Context(), func() { conn.close(req.Context().Err()) })
	_ = conn.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := conn.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		_ = turn.Close()
		return nil, err
	}
	responseHeaders := conn.headers.Clone()
	responseHeaders.Set("Content-Type", "text/event-stream")
	return &http.Response{StatusCode: http.StatusOK, Header: responseHeaders, Body: turn, Request: req}, nil
}

func responsesWSPrefixHash(items []json.RawMessage) [32]byte {
	// 保留数字精度及工具参数原文；表示方式变化时宁可发送完整上下文。
	data, _ := common.Marshal(items)
	return sha256.Sum256(data)
}

func (u *responsesWSUpstream) close(err error) {
	u.closeOnce.Do(func() {
		u.mu.Lock()
		u.closed = true
		active := u.active
		u.mu.Unlock()
		if active != nil {
			_ = active.writer.CloseWithError(err)
		}
		_ = u.conn.Close()
	})
}

func (u *responsesWSUpstream) readLoop() {
	defer close(u.done)
	defer u.close(io.ErrUnexpectedEOF)
	for {
		kind, payload, err := u.conn.ReadMessage()
		if err != nil {
			u.close(err)
			return
		}
		if kind != websocket.TextMessage || !gjson.ValidBytes(payload) {
			u.close(errResponsesWSUpstream)
			return
		}
		value := gjson.ParseBytes(payload)
		u.mu.Lock()
		turn := u.active
		if turn == nil || turn.terminal {
			u.mu.Unlock()
			u.close(errResponsesWSUpstream)
			return
		}
		event := value.Get("type").String()
		id := value.Get("response.id").String()
		if id == "" {
			id = value.Get("response_id").String()
		}
		stream := value.Get("stream_id")
		errorEvent := event == "error" || event == "response.error"
		errorID := value.Get("error.event_id").String()
		// 顶层 event_id 可能由服务端生成；只有已知旧请求 ID 或明确的 error.event_id 才能排除当前轮次。
		previousError := errorID == "" && value.Get("event_id").String() != "" && value.Get("event_id").String() == u.lastEventID
		if stream.Exists() && stream.String() != turn.streamID || errorEvent &&
			(previousError || errorID != "" && errorID != turn.eventID || id != "" && (id == u.lastID || turn.responseID != "" && id != turn.responseID)) {
			u.mu.Unlock()
			continue
		}
		if event == "" || id != "" && (id == u.lastID || turn.responseID != "" && id != turn.responseID) ||
			turn.responseID == "" && id == "" && !errorEvent {
			u.mu.Unlock()
			u.close(errResponsesWSUpstream)
			return
		}
		if id != "" {
			turn.responseID = id
		}
		terminal := event == "response.completed" || event == "response.incomplete" || event == "response.done" || event == "response.failed" || errorEvent
		if terminal {
			turn.terminal = true
			turn.reusable = event == "response.completed" || event == "response.incomplete" || event == "response.done"
			output := value.Get("response.output")
			if turn.reusable && output.IsArray() && len(output.Raw) <= responsesWSInputLimit {
				var items []json.RawMessage
				if common.UnmarshalJsonStr(output.Raw, &items) == nil {
					prefix := append(append([]json.RawMessage(nil), turn.input...), items...)
					turn.prefixCount, turn.prefixHash = len(prefix), responsesWSPrefixHash(prefix)
				}
			}
		}
		u.mu.Unlock()
		if _, err := fmt.Fprintf(turn.writer, "data: %s\n\n", payload); err != nil {
			u.close(err)
			return
		}
		if terminal {
			_ = turn.writer.Close()
		}
	}
}

func (t *responsesWSUpstreamTurn) Read(p []byte) (int, error) { return t.reader.Read(p) }

func (t *responsesWSUpstreamTurn) Close() error {
	t.closeOnce.Do(func() {
		if t.stop != nil {
			t.stop()
		}
		u := t.upstream
		u.mu.Lock()
		keep := !u.closed && t.terminal && t.reusable && t.ctx.Err() == nil
		if u.active == t {
			u.active = nil
			if keep {
				u.lastID, u.lastCount, u.lastHash = t.responseID, t.prefixCount, t.prefixHash
				u.lastEventID = t.eventID
			}
		}
		u.mu.Unlock()
		_ = t.reader.Close()
		if !keep {
			u.close(io.ErrUnexpectedEOF)
		}
	})
	return nil
}
