package controller

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const responsesWSMaxStreams = 32
const responsesWSMaxPending = 32
const responsesWSMaxBufferedBytes = 16 << 20

var responsesWSConnections = struct {
	sync.Mutex
	total    int
	tokens   map[int]int
	sessions map[*responsesWSSession]struct{}
	stopping bool
}{tokens: make(map[int]int), sessions: make(map[*responsesWSSession]struct{})}

func reserveResponsesWSConnection(token int) bool {
	responsesWSConnections.Lock()
	defer responsesWSConnections.Unlock()
	if responsesWSConnections.stopping || responsesWSConnections.total >= max(1, common.GetEnvOrDefault("RESPONSES_WS_MAX_CONNECTIONS", 256)) ||
		responsesWSConnections.tokens[token] >= max(1, common.GetEnvOrDefault("RESPONSES_WS_MAX_CONNECTIONS_PER_TOKEN", 4)) {
		return false
	}
	responsesWSConnections.total++
	responsesWSConnections.tokens[token]++
	return true
}

// HTTP Server.Shutdown 不等待 hijacked 连接，必须在关闭数据库前取消并收拢 WS worker。
func ShutdownResponsesWebSockets(ctx context.Context) error {
	responsesWSConnections.Lock()
	responsesWSConnections.stopping = true
	sessions := make([]*responsesWSSession, 0, len(responsesWSConnections.sessions))
	for session := range responsesWSConnections.sessions {
		sessions = append(sessions, session)
	}
	responsesWSConnections.Unlock()
	for _, session := range sessions {
		session.cancel()
	}
	for _, session := range sessions {
		select {
		case <-session.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func releaseResponsesWSConnection(token int) {
	responsesWSConnections.Lock()
	defer responsesWSConnections.Unlock()
	responsesWSConnections.total--
	responsesWSConnections.tokens[token]--
	if responsesWSConnections.tokens[token] == 0 {
		delete(responsesWSConnections.tokens, token)
	}
}

type responsesWSTurn struct {
	event     responsesWSCreate
	writer    *responsesWSWriter
	success   bool
	requestID string
}

type responsesWSTurnKey struct{}

// 每轮使用新的 Gin context，并在发送 terminal 前完成中间件退出和账本处理。
var responsesWSEngine = sync.OnceValue(func() *gin.Engine {
	engine := gin.New()
	engine.ForwardedByClientIP = false
	_ = engine.SetTrustedProxies(nil)
	engine.Use(gin.CustomRecovery(func(c *gin.Context, _ any) { c.AbortWithStatus(http.StatusInternalServerError) }))
	engine.POST("/v1/responses", middleware.BodyStorageCleanup(), middleware.RequestId(), middleware.SystemPerformanceCheck(),
		middleware.TokenAuth(), middleware.TokenRequestLimit(), middleware.ModelRequestRateLimit(), middleware.Distribute(), func(c *gin.Context) {
			turn := c.Request.Context().Value(responsesWSTurnKey{}).(*responsesWSTurn)
			turn.requestID = c.GetString(common.RequestIdKey)
			if turn.event.warmup {
				if _, err := helper.GetAndValidateResponsesRequest(c); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": types.OpenAIError{Type: "invalid_request_error", Code: "invalid_request", Message: "Invalid Responses request."}})
					return
				}
				turn.writer.warmup()
				turn.success = true
				service.SetRelaySemanticSuccess(c, true)
				return
			}
			Relay(c, types.RelayFormatOpenAIResponses)
			turn.success = service.RelaySemanticSuccess(c)
		})
	return engine
})

type responsesWSActive struct {
	ctx           context.Context
	event         responsesWSCreate
	cancel        context.CancelFunc
	writer        *responsesWSWriter
	cancelled     bool
	cancelEventID string
}

type responsesWSLane struct {
	queue     chan *responsesWSActive
	turns     []*responsesWSActive
	transport *relaychannel.ResponsesWSTransport
}

type responsesWSSession struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	client                *websocket.Conn
	headers               http.Header
	remoteAddr            string
	writeMu               sync.Mutex
	mu                    sync.Mutex
	lanes                 map[string]*responsesWSLane
	pending, pendingBytes int
	history               responsesWSHistory
	workers               sync.WaitGroup
	done                  chan struct{}
}

func ResponsesWebSocket(c *gin.Context) {
	tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	if !reserveResponsesWSConnection(tokenID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": types.OpenAIError{Type: "rate_limit_error", Code: "websocket_connection_limit", Message: "WebSocket connection capacity reached."}})
		return
	}
	defer releaseResponsesWSConnection(tokenID)
	upgrader := websocket.Upgrader{HandshakeTimeout: 10 * time.Second}
	client, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Hour)
	defer cancel()
	headers := c.Request.Header.Clone()
	for _, name := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Protocol", "Content-Length", "Content-Encoding"} {
		headers.Del(name)
	}
	headers.Set("Content-Type", "application/json")
	s := &responsesWSSession{ctx: ctx, cancel: cancel, client: client, headers: headers,
		remoteAddr: net.JoinHostPort(c.ClientIP(), "0"), lanes: make(map[string]*responsesWSLane), done: make(chan struct{})}
	responsesWSConnections.Lock()
	if responsesWSConnections.stopping {
		responsesWSConnections.Unlock()
		return
	}
	responsesWSConnections.sessions[s] = struct{}{}
	responsesWSConnections.Unlock()
	client.SetReadLimit(int64(responsesWSBodyLimit()))
	s.workers.Go(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = client.Close()
				return
			case <-ticker.C:
				if client.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)) != nil {
					cancel()
				}
			}
		}
	})
	defer func() {
		cancel()
		s.workers.Wait()
		responsesWSConnections.Lock()
		delete(responsesWSConnections.sessions, s)
		responsesWSConnections.Unlock()
		close(s.done)
	}()
	for {
		kind, data, err := client.ReadMessage()
		if err != nil {
			return
		}
		if kind != websocket.TextMessage {
			s.sendError("", "", "invalid_request", "Only JSON text messages are supported.", http.StatusBadRequest)
			continue
		}
		event, err := parseResponsesWSCreate(data)
		if err != nil {
			s.sendRequestError(event, err)
			continue
		}
		if event.kind == "response.cancel" {
			s.cancelTurn(event)
			continue
		}
		if event.kind != "response.create" {
			s.sendError(event.eventID, event.streamID, "invalid_request", "Unsupported WebSocket event type.", http.StatusBadRequest)
			continue
		}
		s.enqueue(event)
	}
}

func (s *responsesWSSession) send(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	_ = s.client.SetWriteDeadline(time.Now().Add(30 * time.Second))
	err := s.client.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		s.cancel()
	}
	return err
}

func (s *responsesWSSession) sendError(eventID, streamID, code, message string, status int) {
	errorType := "invalid_request_error"
	if status >= 500 {
		errorType = "server_error"
	} else if status == http.StatusTooManyRequests {
		errorType = "rate_limit_error"
	}
	payload, _ := common.Marshal(responsesWSError{Type: "error", EventID: eventID, StreamID: streamID, Status: status,
		Error: types.OpenAIError{Type: errorType, Code: code, Message: message}})
	_ = s.send(payload)
}

func (s *responsesWSSession) sendRequestError(event responsesWSCreate, err error) {
	var requestError *responsesWSRequestError
	if errors.As(err, &requestError) {
		s.sendError(event.eventID, event.streamID, requestError.code, requestError.message, requestError.status)
		return
	}
	s.sendError(event.eventID, event.streamID, "invalid_request", err.Error(), http.StatusBadRequest)
}

func (s *responsesWSSession) enqueue(event responsesWSCreate) {
	s.mu.Lock()
	lane := s.lanes[event.streamID]
	if lane == nil && len(s.lanes) >= responsesWSMaxStreams {
		s.mu.Unlock()
		s.sendError(event.eventID, event.streamID, "websocket_stream_limit_reached", "This connection has reached its stream limit (32).", 400)
		return
	}
	if s.pending >= responsesWSMaxPending || s.pendingBytes+len(event.body) > responsesWSMaxBufferedBytes {
		s.mu.Unlock()
		s.sendError(event.eventID, event.streamID, "websocket_queue_full", "The WebSocket request queue is full.", 429)
		return
	}
	if lane == nil {
		lane = &responsesWSLane{queue: make(chan *responsesWSActive, responsesWSMaxPending), transport: relaychannel.NewResponsesWSTransport(s.ctx)}
		s.lanes[event.streamID] = lane
		s.workers.Go(func() { s.runLane(lane) })
	}
	s.pending++
	s.pendingBytes += len(event.body)
	ctx, cancel := context.WithCancel(s.ctx)
	active := &responsesWSActive{ctx: ctx, event: event, cancel: cancel, writer: newResponsesWSWriter(s, event)}
	lane.turns = append(lane.turns, active)
	lane.queue <- active
	s.mu.Unlock()
}

func (s *responsesWSSession) cancelTurn(event responsesWSCreate) {
	s.mu.Lock()
	lane := s.lanes[event.streamID]
	var active *responsesWSActive
	if lane != nil && len(lane.turns) > 0 {
		active = lane.turns[0]
	}
	if active == nil || active.writer.terminalSeen.Load() ||
		(event.responseID != "" && event.responseID != active.writer.responseIDValue()) {
		s.mu.Unlock()
		s.sendError(event.eventID, event.streamID, "invalid_response_state", "No matching response is running.", 400)
		return
	}
	active.cancelled = true
	active.cancelEventID = event.eventID
	active.cancel()
	s.mu.Unlock()
}

func (s *responsesWSSession) runLane(lane *responsesWSLane) {
	defer lane.transport.Close()
	for {
		select {
		case <-s.ctx.Done():
			return
		case active := <-lane.queue:
			if s.ctx.Err() != nil {
				return
			}
			s.runTurn(lane, active)
		}
	}
}

func (s *responsesWSSession) runTurn(lane *responsesWSLane, active *responsesWSActive) {
	event, writer := active.event, active.writer
	defer active.cancel()
	accounted := len(event.body)
	released := false
	release := func() {
		if released {
			return
		}
		s.mu.Lock()
		s.pending--
		s.pendingBytes -= accounted
		lane.turns[0] = nil
		lane.turns = lane.turns[1:]
		s.mu.Unlock()
		released = true
	}
	defer release()
	if active.ctx.Err() != nil {
		s.mu.Lock()
		cancelEventID := active.cancelEventID
		s.mu.Unlock()
		release()
		writer.finish(false, true, cancelEventID)
		return
	}
	body, input, previous, replayable, err := s.history.prepare(event.body)
	if err != nil {
		release()
		s.sendRequestError(event, err)
		return
	}
	extra := max(0, len(body)-len(event.body))
	s.mu.Lock()
	if s.pendingBytes+extra > responsesWSMaxBufferedBytes {
		s.mu.Unlock()
		release()
		s.sendError(event.eventID, event.streamID, "websocket_queue_full", "The combined WebSocket inputs exceed the connection buffer limit.", 429)
		return
	}
	s.pendingBytes += extra
	accounted += extra
	s.mu.Unlock()
	turn := &responsesWSTurn{event: event, writer: writer}
	ctx := context.WithValue(active.ctx, responsesWSTurnKey{}, turn)
	execution := &relaycommon.ResponsesWSExecution{StreamID: event.streamID, EventID: event.eventID, PreviousResponseID: previous, Transport: lane.transport}
	ctx = relaycommon.WithResponsesWSExecution(ctx, execution)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/responses", bytes.NewReader(body))
	req.RequestURI = "/v1/responses"
	req.Header = s.headers.Clone()
	req.RemoteAddr = s.remoteAddr
	responsesWSEngine().ServeHTTP(writer, req)
	if execution.SettlementError != nil {
		turn.success = false
	}
	s.mu.Lock()
	cancelled, cancelEventID := active.cancelled, active.cancelEventID
	s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	if turn.success && writer.err == nil && replayable {
		s.history.remember(writer.responseIDValue(), input, writer.output(), event.warmup)
	}
	release()
	writer.finish(turn.success, cancelled, cancelEventID)
}

func responsesWSBodyLimit() int {
	if constant.MaxRequestBodyMB > 0 {
		return min(constant.MaxRequestBodyMB<<20, responsesWSMaxBufferedBytes)
	}
	return responsesWSMaxBufferedBytes
}

var errResponsesWSProtocol = errors.New("invalid Responses WebSocket stream")
