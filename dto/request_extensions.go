package dto

import (
	"reflect"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// RequestExtensions preserves provider extensions on same-protocol relays.
// Cross-protocol converters must explicitly map or reject these fields.
type RequestExtensions struct {
	ExtraFields map[string]common.RawMessage `json:"-"`
}

var requestFieldNames sync.Map

func knownRequestFields(value any) map[string]bool {
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if cached, ok := requestFieldNames.Load(typ); ok {
		return cached.(map[string]bool)
	}
	names := make(map[string]bool)
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			names[strings.ToLower(name)] = true
		}
	}
	requestFieldNames.Store(typ, names)
	return names
}

func unmarshalRequestExtensions(data []byte, value any, extensions *RequestExtensions) error {
	if err := common.Unmarshal(data, value); err != nil {
		return err
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	known := knownRequestFields(value)
	for name := range fields {
		if known[strings.ToLower(name)] {
			delete(fields, name)
		}
	}
	extensions.ExtraFields = fields
	return nil
}

func marshalRequestExtensions(value any, extensions RequestExtensions) ([]byte, error) {
	data, err := common.Marshal(value)
	if err != nil || len(extensions.ExtraFields) == 0 {
		return data, err
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	known := knownRequestFields(value)
	for name, raw := range extensions.ExtraFields {
		// Never resurrect declared fields intentionally removed by an adapter.
		if !known[strings.ToLower(name)] {
			fields[name] = raw
		}
	}
	return common.Marshal(fields)
}

type generalOpenAIRequestFields GeneralOpenAIRequest

func (r *GeneralOpenAIRequest) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*generalOpenAIRequestFields)(r), &r.RequestExtensions)
}
func (r GeneralOpenAIRequest) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(generalOpenAIRequestFields(r), r.RequestExtensions)
}

type responsesRequestFields OpenAIResponsesRequest

func (r *OpenAIResponsesRequest) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*responsesRequestFields)(r), &r.RequestExtensions)
}
func (r OpenAIResponsesRequest) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(responsesRequestFields(r), r.RequestExtensions)
}

type claudeRequestFields ClaudeRequest

func (r *ClaudeRequest) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*claudeRequestFields)(r), &r.RequestExtensions)
}
func (r ClaudeRequest) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(claudeRequestFields(r), r.RequestExtensions)
}

type claudeMediaFields ClaudeMediaMessage

func (r *ClaudeMediaMessage) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*claudeMediaFields)(r), &r.RequestExtensions)
}
func (r ClaudeMediaMessage) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(claudeMediaFields(r), r.RequestExtensions)
}

type claudeMessageSourceFields ClaudeMessageSource

func (r *ClaudeMessageSource) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*claudeMessageSourceFields)(r), &r.RequestExtensions)
}
func (r ClaudeMessageSource) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(claudeMessageSourceFields(r), r.RequestExtensions)
}

type claudeMessageFields ClaudeMessage

func (r *ClaudeMessage) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*claudeMessageFields)(r), &r.RequestExtensions)
}
func (r ClaudeMessage) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(claudeMessageFields(r), r.RequestExtensions)
}

type thinkingFields Thinking

func (r *Thinking) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*thinkingFields)(r), &r.RequestExtensions)
}
func (r Thinking) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(thinkingFields(r), r.RequestExtensions)
}

type messageFields Message

func (r *Message) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*messageFields)(r), &r.RequestExtensions)
}
func (r Message) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(messageFields(r), r.RequestExtensions)
}

type reasoningFields Reasoning

func (r *Reasoning) UnmarshalJSON(data []byte) error {
	return unmarshalRequestExtensions(data, (*reasoningFields)(r), &r.RequestExtensions)
}
func (r Reasoning) MarshalJSON() ([]byte, error) {
	return marshalRequestExtensions(reasoningFields(r), r.RequestExtensions)
}
