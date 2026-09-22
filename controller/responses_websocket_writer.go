package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type responsesWSError struct {
	Type       string            `json:"type"`
	Status     int               `json:"status"`
	EventID    string            `json:"event_id,omitempty"`
	StreamID   string            `json:"stream_id,omitempty"`
	ResponseID string            `json:"response_id,omitempty"`
	Error      types.OpenAIError `json:"error"`
}

type responsesWSWriter struct {
	session          *responsesWSSession
	event            responsesWSCreate
	header           http.Header
	status           int
	pending, body    bytes.Buffer
	terminal         []byte
	terminalSeen     atomic.Bool
	idMu             sync.Mutex
	responseID       string
	text             strings.Builder
	outputItems      []json.RawMessage
	outputBytes      int
	snapshotTooLarge bool
	err              error
}

func newResponsesWSWriter(session *responsesWSSession, event responsesWSCreate) *responsesWSWriter {
	return &responsesWSWriter{session: session, event: event, header: make(http.Header)}
}

func (w *responsesWSWriter) Header() http.Header { return w.header }
func (w *responsesWSWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *responsesWSWriter) Flush() {}

func (w *responsesWSWriter) responseIDValue() string {
	w.idMu.Lock()
	defer w.idMu.Unlock()
	return w.responseID
}

func (w *responsesWSWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	w.WriteHeader(http.StatusOK)
	if !strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream") {
		if w.body.Len()+len(data) > 1<<20 {
			w.err = errResponsesWSProtocol
			return 0, w.err
		}
		return w.body.Write(data)
	}
	if w.pending.Len()+len(data) > helper.DefaultMaxScannerBufferSize {
		w.err = errResponsesWSProtocol
		return 0, w.err
	}
	w.pending.Write(data)
	for {
		pending := w.pending.Bytes()
		index, delim := bytes.Index(pending, []byte("\n\n")), 2
		if crlf := bytes.Index(pending, []byte("\r\n\r\n")); crlf >= 0 && (index < 0 || crlf < index) {
			index, delim = crlf, 4
		}
		if index < 0 {
			break
		}
		frame := string(w.pending.Next(index + delim))
		var values []string
		for _, line := range strings.Split(strings.ReplaceAll(frame, "\r\n", "\n"), "\n") {
			if value, ok := helper.ParseSSEField(line, "data"); ok {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			continue
		}
		payload := []byte(strings.Join(values, "\n"))
		if string(payload) == "[DONE]" {
			continue
		}
		if err := w.eventData(payload); err != nil {
			w.err = err
			return 0, err
		}
	}
	return len(data), nil
}

func (w *responsesWSWriter) eventData(data []byte) error {
	if !gjson.ValidBytes(data) {
		return errResponsesWSProtocol
	}
	value := gjson.ParseBytes(data)
	kind := value.Get("type").String()
	if kind == "" || w.terminalSeen.Load() {
		return errResponsesWSProtocol
	}
	id := value.Get("response.id").String()
	if id == "" {
		id = value.Get("response_id").String()
	}
	if id != "" {
		w.idMu.Lock()
		conflict := w.responseID != "" && w.responseID != id
		if !conflict {
			w.responseID = id
		}
		w.idMu.Unlock()
		if conflict {
			return errResponsesWSProtocol
		}
	}
	if kind == "error" || kind == "response.failed" || kind == "response.error" {
		return errResponsesWSProtocol
	}
	if w.event.streamID != "" {
		var err error
		data, err = sjson.SetBytes(data, "stream_id", w.event.streamID)
		if err != nil {
			return err
		}
	} else {
		data, _ = sjson.DeleteBytes(data, "stream_id")
	}
	if kind == "response.completed" || kind == "response.done" || kind == "response.incomplete" {
		w.terminal = data
		w.terminalSeen.Store(true)
		return nil
	}
	if kind == "response.output_text.delta" && !w.snapshotTooLarge {
		delta := value.Get("delta").String()
		if w.text.Len()+len(delta) <= responsesWSMaxBufferedBytes {
			w.text.WriteString(delta)
		} else {
			w.snapshotTooLarge = true
			w.text.Reset()
		}
	}
	if kind == "response.output_item.done" && !w.snapshotTooLarge {
		if item := value.Get("item"); item.IsObject() {
			w.outputBytes += len(item.Raw)
			if w.outputBytes <= responsesWSMaxBufferedBytes {
				w.outputItems = append(w.outputItems, json.RawMessage(item.Raw))
			} else {
				w.snapshotTooLarge = true
				w.outputItems = nil
				w.text.Reset()
			}
		}
	}
	return w.session.send(data)
}

func (w *responsesWSWriter) warmup() {
	id := "resp_" + common.GetUUID()
	w.idMu.Lock()
	w.responseID = id
	w.idMu.Unlock()
	w.terminal, _ = common.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{
		"id": id, "object": "response", "status": "completed", "output": []any{}, "usage": map[string]int{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}}})
	if w.event.streamID != "" {
		w.terminal, _ = sjson.SetBytes(w.terminal, "stream_id", w.event.streamID)
	}
	w.terminalSeen.Store(true)
}

func (w *responsesWSWriter) output() []json.RawMessage {
	if w.snapshotTooLarge {
		return nil
	}
	if output := gjson.GetBytes(w.terminal, "response.output"); output.IsArray() && len(output.Array()) > 0 {
		var items []json.RawMessage
		if common.UnmarshalJsonStr(output.Raw, &items) == nil {
			return items
		}
	}
	if len(w.outputItems) > 0 {
		return w.outputItems
	}
	if w.text.Len() > 0 {
		item, _ := common.Marshal(map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": w.text.String()}}})
		return []json.RawMessage{item}
	}
	return nil
}

func (w *responsesWSWriter) finish(success, cancelled bool, cancelEventID string) {
	if success && w.err == nil && len(w.terminal) > 0 && len(bytes.TrimSpace(w.pending.Bytes())) == 0 {
		_ = w.session.send(w.terminal)
		return
	}
	if cancelled {
		payload, _ := common.Marshal(map[string]any{"type": "response.cancelled", "response": map[string]string{"id": w.responseIDValue(), "status": "cancelled"}})
		if w.event.streamID != "" {
			payload, _ = sjson.SetBytes(payload, "stream_id", w.event.streamID)
		}
		if cancelEventID != "" {
			payload, _ = sjson.SetBytes(payload, "event_id", cancelEventID)
		}
		_ = w.session.send(payload)
		return
	}
	response := responsesWSError{Type: "error", Status: http.StatusServiceUnavailable, EventID: w.event.eventID, StreamID: w.event.streamID,
		ResponseID: w.responseIDValue(), Error: types.OpenAIError{Type: "server_error", Code: string(types.ErrorCodeModelCapacity), Message: types.PublicModelCapacityMessage}}
	if w.status >= 400 && w.body.Len() > 0 {
		var rejected struct {
			Error *types.OpenAIError `json:"error"`
		}
		if common.Unmarshal(w.body.Bytes(), &rejected) == nil && rejected.Error != nil {
			response.Status, response.Error = w.status, *rejected.Error
		}
	}
	payload, _ := common.Marshal(response)
	_ = w.session.send(payload)
}
