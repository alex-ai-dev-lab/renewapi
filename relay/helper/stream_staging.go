package helper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var errFirstSemanticTimeout = errors.New("first_semantic_timeout")
var errStreamStaging = errors.New("upstream_stream_staging")

const maxStreamStagingBytes = 1 << 20

// streamStagingWriter 在真正语义输出出现之前隔离头部和 SSE，失败时可以完整丢弃。
type streamStagingWriter struct {
	gin.ResponseWriter
	mu       sync.Mutex
	header   http.Header
	status   int
	pending  []byte
	staged   bytes.Buffer
	state    atomic.Int32 // 0 等待语义，1 已观察语义，2 超时
	err      error
	terminal bool
	info     *relaycommon.RelayInfo
}

func (w *streamStagingWriter) Header() http.Header { return w.header }
func (w *streamStagingWriter) Status() int         { return w.status }
func (w *streamStagingWriter) WriteHeader(code int) {
	if !w.Written() && code > 0 {
		w.status = code
	}
}
func (w *streamStagingWriter) WriteHeaderNow() {}
func (w *streamStagingWriter) Flush() {
	if w.Written() {
		w.ResponseWriter.Flush()
	}
}
func (w *streamStagingWriter) Unwrap() http.ResponseWriter       { return w.ResponseWriter }
func (w *streamStagingWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *streamStagingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	if w.state.Load() == 2 {
		return 0, errFirstSemanticTimeout
	}
	if !strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream") {
		if w.staged.Len()+len(p) > getScannerBufferSize() {
			w.err = fmt.Errorf("%w: response exceeds staging limit", errStreamStaging)
			return 0, w.err
		}
		_, _ = w.staged.Write(p)
		return len(p), nil
	}
	w.pending = append(w.pending, p...)
	if len(w.pending) > getScannerBufferSize() {
		w.err = fmt.Errorf("%w: frame exceeds staging limit", errStreamStaging)
		return 0, w.err
	}
	for {
		index, delimiter := bytes.Index(w.pending, []byte("\n\n")), 2
		if crlf := bytes.Index(w.pending, []byte("\r\n\r\n")); crlf >= 0 && (index < 0 || crlf < index) {
			index, delimiter = crlf, 4
		}
		if index < 0 {
			break
		}
		frame := append([]byte(nil), w.pending[:index+delimiter]...)
		w.pending = w.pending[index+delimiter:]
		var data []string
		for _, line := range strings.Split(strings.ReplaceAll(string(frame), "\r\n", "\n"), "\n") {
			if value, ok := ParseSSEField(line, "data"); ok {
				data = append(data, value)
			}
		}
		if len(data) == 0 {
			continue
		}
		payload := strings.Join(data, "\n")
		meaning := relaycommon.ClassifyStreamPayload(payload)
		if !meaning.Valid || meaning.Failed {
			w.err = fmt.Errorf("%w: malformed or failed SSE: %s", errStreamStaging, common.LocalLogPreview(payload))
			return 0, w.err
		}
		w.terminal = w.terminal || meaning.Terminal
		if !w.Written() {
			_, _ = w.staged.Write(frame)
			if meaning.Semantic {
				if !w.state.CompareAndSwap(0, 1) && w.state.Load() == 2 {
					return 0, errFirstSemanticTimeout
				}
				if err := w.commit(); err != nil {
					return 0, err
				}
			}
		} else if _, err := w.ResponseWriter.Write(frame); err != nil {
			w.err = err
			return 0, err
		}
	}
	if !w.Written() && w.staged.Len()+len(w.pending) > maxStreamStagingBytes {
		w.err = fmt.Errorf("%w: SSE prelude exceeds staging limit", errStreamStaging)
		return 0, w.err
	}
	return len(p), nil
}

func (w *streamStagingWriter) commit() error {
	for key, values := range w.header {
		w.ResponseWriter.Header()[key] = append([]string(nil), values...)
	}
	w.ResponseWriter.WriteHeader(w.status)
	written, err := w.ResponseWriter.Write(w.staged.Bytes())
	w.err = err
	if w.err == nil && written != w.staged.Len() {
		w.err = io.ErrShortWrite
	}
	w.staged.Reset()
	if w.Written() && w.info.StreamStatus != nil {
		w.info.StreamStatus.MarkClientCommitted()
	}
	w.info.SetFirstResponseTime()
	return w.err
}

func (w *streamStagingWriter) validate() *types.NewAPIError {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state.Load() == 2 {
		return types.NewOpenAIError(errFirstSemanticTimeout, types.ErrorCodeChannelResponseTimeExceeded, http.StatusGatewayTimeout)
	}
	if w.err != nil {
		return types.NewOpenAIError(w.err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if !strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream") {
		return nil
	}
	if !w.Written() {
		return types.NewOpenAIError(errors.New("upstream stream ended without semantic output"), types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}
	if len(bytes.TrimSpace(w.pending)) > 0 || !w.terminal {
		return types.NewOpenAIError(errors.New("upstream stream ended before a protocol terminal"), types.ErrorCodeTruncatedResponse, http.StatusBadGateway)
	}
	return nil
}

func ValidateStreamStaging(c *gin.Context) *types.NewAPIError {
	if writer, ok := c.Writer.(*streamStagingWriter); ok {
		return writer.validate()
	}
	return nil
}

func WithStreamStaging(c *gin.Context, info *relaycommon.RelayInfo, call func() *types.NewAPIError) *types.NewAPIError {
	if !info.IsStream || info.RelayFormat == types.RelayFormatOpenAIRealtime {
		return call()
	}
	originalWriter, originalRequest := c.Writer, c.Request
	writer := &streamStagingWriter{ResponseWriter: originalWriter, header: originalWriter.Header().Clone(), status: http.StatusOK, info: info}
	c.Writer = writer
	c.Set("event_stream_headers_set", false)
	ctx, cancel := context.WithCancelCause(originalRequest.Context())
	c.Request = originalRequest.WithContext(ctx)
	timeout := time.Duration(common.RelayFirstByteTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	timer := time.AfterFunc(timeout, func() {
		if writer.state.CompareAndSwap(0, 2) {
			cancel(errFirstSemanticTimeout)
		}
	})
	defer func() {
		timer.Stop()
		cancel(nil)
		c.Writer, c.Request = originalWriter, originalRequest
		c.Set("event_stream_headers_set", false)
	}()
	err := call()
	if writer.state.Load() == 2 {
		if info.StreamStatus == nil {
			info.StreamStatus = relaycommon.NewStreamStatus()
		}
		info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonFirstByteTimeout, errFirstSemanticTimeout)
		err = writer.validate()
	}
	if err == nil {
		err = writer.validate()
	}
	if err == nil && !writer.Written() && writer.staged.Len() > 0 {
		if commitErr := writer.commit(); commitErr != nil {
			err = types.NewError(commitErr, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
	}
	return err
}
