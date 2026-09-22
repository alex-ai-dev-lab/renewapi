package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

type responsesWSCreate struct {
	kind, eventID, streamID, responseID string
	body                                []byte
	warmup                              bool
}

type responsesWSRequestError struct {
	code, message string
	status        int
}

func (e *responsesWSRequestError) Error() string { return e.message }

func parseResponsesWSCreate(data []byte) (responsesWSCreate, error) {
	var fields map[string]json.RawMessage
	var event responsesWSCreate
	if common.Unmarshal(data, &fields) != nil || fields == nil {
		return event, errors.New("Invalid WebSocket event JSON.")
	}
	_ = common.Unmarshal(fields["type"], &event.kind)
	if raw, ok := fields["event_id"]; ok {
		if common.Unmarshal(raw, &event.eventID) != nil || string(raw) == "null" || len(event.eventID) > 256 {
			event.eventID = ""
			return event, errors.New("Invalid event_id.")
		}
	}
	if raw, ok := fields["response_id"]; ok && (common.Unmarshal(raw, &event.responseID) != nil || string(raw) == "null" || len(event.responseID) > 256) {
		return event, errors.New("Invalid response_id.")
	}
	if raw := fields["response"]; len(raw) > 0 && event.kind == "response.create" {
		var nested map[string]json.RawMessage
		if common.Unmarshal(raw, &nested) != nil || nested == nil {
			return event, errors.New("Invalid response.create payload.")
		}
		for _, key := range []string{"stream_id", "generate"} {
			if value, ok := fields[key]; ok {
				nested[key] = value
			}
		}
		fields = nested
	}
	if raw, ok := fields["stream_id"]; ok {
		if common.Unmarshal(raw, &event.streamID) != nil || len(event.streamID) < 1 || len(event.streamID) > 256 {
			event.streamID = ""
			return event, &responsesWSRequestError{"invalid_stream_id", "stream_id must contain 1-256 ASCII letters, digits, underscores, hyphens, or periods.", http.StatusBadRequest}
		}
		for _, ch := range event.streamID {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-' || ch == '.') {
				event.streamID = ""
				return event, &responsesWSRequestError{"invalid_stream_id", "Invalid stream_id.", http.StatusBadRequest}
			}
		}
	}
	if raw, ok := fields["generate"]; ok {
		var generate bool
		if string(raw) == "null" || common.Unmarshal(raw, &generate) != nil {
			return event, errors.New("generate must be a boolean.")
		}
		event.warmup = !generate
	}
	if raw, ok := fields["background"]; ok && string(raw) != "false" && string(raw) != "null" {
		return event, errors.New("background is not supported in WebSocket mode.")
	}
	for _, key := range []string{"type", "event_id", "stream_id", "response_id", "generate", "background"} {
		delete(fields, key)
	}
	fields["stream"] = json.RawMessage("true")
	if _, ok := fields["input"]; !ok {
		fields["input"] = json.RawMessage("[]")
	}
	event.body, _ = common.Marshal(fields)
	return event, nil
}

type responsesWSSnapshot struct {
	input []json.RawMessage
	size  int
}

// 续写上下文仅保存在当前客户端连接内，不能跨用户、Token 或连接共享。
type responsesWSHistory struct {
	mu    sync.Mutex
	items map[string]responsesWSSnapshot
	order []string
	bytes int
}

func responsesWSInput(raw json.RawMessage) ([]json.RawMessage, error) {
	if string(raw) == "null" {
		return nil, errors.New("input must be a string or an array.")
	}
	if gjson.ParseBytes(raw).Type == gjson.String {
		item, _ := common.Marshal(map[string]string{"role": "user", "content": gjson.ParseBytes(raw).String()})
		return []json.RawMessage{item}, nil
	}
	var input []json.RawMessage
	if common.Unmarshal(raw, &input) != nil {
		return nil, errors.New("input must be a string or an array.")
	}
	return input, nil
}

func (h *responsesWSHistory) prepare(body []byte) ([]byte, []json.RawMessage, string, bool, error) {
	var fields map[string]json.RawMessage
	if common.Unmarshal(body, &fields) != nil {
		return nil, nil, "", false, errors.New("Invalid Responses request.")
	}
	input, err := responsesWSInput(fields["input"])
	if err != nil {
		return nil, nil, "", false, err
	}
	var previous string
	if raw, ok := fields["previous_response_id"]; ok && string(raw) != "null" {
		if common.Unmarshal(raw, &previous) != nil {
			return nil, nil, "", false, errors.New("Invalid previous_response_id.")
		}
	}
	replayable := true
	if previous != "" {
		h.mu.Lock()
		parent, found := h.items[previous]
		h.mu.Unlock()
		if found {
			input = append(append([]json.RawMessage(nil), parent.input...), input...)
			delete(fields, "previous_response_id")
		} else {
			if string(fields["store"]) == "false" {
				return nil, nil, "", false, &responsesWSRequestError{"previous_response_not_found", "Previous response is unavailable on this connection. Send the full input with previous_response_id omitted.", http.StatusBadRequest}
			}
			replayable = false
		}
	}
	fields["input"], _ = common.Marshal(input)
	prepared, err := common.Marshal(fields)
	if len(prepared) > responsesWSBodyLimit() {
		return nil, nil, "", false, &responsesWSRequestError{"request_too_large", "The combined Responses input exceeds the request body limit.", http.StatusRequestEntityTooLarge}
	}
	return prepared, input, previous, replayable, err
}

func (h *responsesWSHistory) remember(id string, input, output []json.RawMessage, warmup bool) {
	if id == "" || output == nil && !warmup {
		return
	}
	items := append(append([]json.RawMessage(nil), input...), output...)
	size := 0
	for _, item := range items {
		size += len(item)
	}
	if size > responsesWSMaxBufferedBytes {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.items == nil {
		h.items = make(map[string]responsesWSSnapshot)
	}
	if _, exists := h.items[id]; exists {
		return
	}
	for len(h.order) >= 64 || h.bytes+size > responsesWSMaxBufferedBytes {
		first := h.order[0]
		h.order = h.order[1:]
		h.bytes -= h.items[first].size
		delete(h.items, first)
	}
	h.items[id] = responsesWSSnapshot{input: items, size: size}
	h.order = append(h.order, id)
	h.bytes += size
}
