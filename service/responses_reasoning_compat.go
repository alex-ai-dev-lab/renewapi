package service

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/tidwall/gjson"
)

type ResponsesReasoningContentSanitizeResult struct {
	Changed                 bool
	RemovedReasoningContent int
}

func SanitizeResponsesReasoningContent(req *dto.OpenAIResponsesRequest) ResponsesReasoningContentSanitizeResult {
	if req == nil || len(req.Input) == 0 {
		return ResponsesReasoningContentSanitizeResult{}
	}
	nextInput, removed, changed := sanitizeResponsesReasoningContentJSON(req.Input)
	if !changed {
		return ResponsesReasoningContentSanitizeResult{}
	}
	req.Input = nextInput
	return ResponsesReasoningContentSanitizeResult{
		Changed:                 true,
		RemovedReasoningContent: removed,
	}
}

func sanitizeResponsesReasoningContentJSON(raw json.RawMessage) (json.RawMessage, int, bool) {
	// Without a literal match or an escape there cannot be a reasoning type.
	// Escaped keys/values take the structural path, never a substring decision.
	if !bytes.Contains(raw, []byte("reasoning")) && !bytes.ContainsRune(raw, '\\') {
		return raw, 0, false
	}
	if !common.ValidJSON(raw) {
		return raw, 0, false
	}
	type removal struct{ start, end int }
	var removals []removal
	fallback := !utf8.Valid(raw)
	// The legacy decoder rejects numbers outside float64 even in a subtree
	// that will be removed. Keep that behavior for these exceptional inputs.
	var invalidNumber func(gjson.Result) bool
	invalidNumber = func(value gjson.Result) bool {
		if value.Type == gjson.Number {
			return math.IsInf(value.Num, 0)
		}
		invalid := false
		if value.IsArray() || value.IsObject() {
			value.ForEach(func(_, child gjson.Result) bool {
				invalid = invalidNumber(child)
				return !invalid
			})
		}
		return invalid
	}
	var walk func(gjson.Result, bool)
	walk = func(value gjson.Result, parentIsReasoning bool) {
		if value.IsArray() {
			value.ForEach(func(_, child gjson.Result) bool {
				walk(child, false)
				return true
			})
			return
		}
		if !value.IsObject() {
			fallback = fallback || (value.Type == gjson.Number && math.IsInf(value.Num, 0))
			return
		}
		isReasoning := parentIsReasoning || value.Get("type").Str == "reasoning"
		seen := make(map[string]struct{})
		value.ForEach(func(key, child gjson.Result) bool {
			if _, duplicate := seen[key.Str]; duplicate {
				// The decoder uses the last duplicate member; GJSON uses the first.
				fallback = true
			}
			seen[key.Str] = struct{}{}
			if isReasoning && key.Str == "content" && child.IsArray() && strings.TrimSpace(child.Raw[1:len(child.Raw)-1]) != "" {
				fallback = fallback || invalidNumber(child)
				start, end := key.Index, child.Index+len(child.Raw)
				after := end
				for after < len(raw) && raw[after] <= ' ' {
					after++
				}
				if after < len(raw) && raw[after] == ',' {
					end = after + 1
				} else {
					before := start - 1
					for before >= 0 && raw[before] <= ' ' {
						before--
					}
					if before >= 0 && raw[before] == ',' {
						start = before
					}
				}
				removals = append(removals, removal{start, end})
				return true
			}
			walk(child, isReasoning)
			return true
		})
	}
	walk(gjson.Parse(string(raw)), false)
	if fallback {
		return sanitizeResponsesReasoningContentJSONDecoded(raw)
	}
	if len(removals) == 0 {
		return raw, 0, false
	}
	size := len(raw)
	for _, part := range removals {
		size -= part.end - part.start
	}
	data := make([]byte, 0, size)
	from := 0
	for _, part := range removals {
		data = append(data, raw[from:part.start]...)
		from = part.end
	}
	data = append(data, raw[from:]...)
	return data, len(removals), true
}

func sanitizeResponsesReasoningContentJSONDecoded(raw json.RawMessage) (json.RawMessage, int, bool) {
	var v any
	if err := common.Unmarshal(raw, &v); err != nil {
		return raw, 0, false
	}
	next, removed, changed := sanitizeResponsesReasoningContentValue(v, false)
	if !changed {
		return raw, 0, false
	}
	data, err := common.Marshal(next)
	if err != nil {
		return raw, 0, false
	}
	return data, removed, true
}

func sanitizeResponsesReasoningContentValue(v any, parentIsReasoning bool) (any, int, bool) {
	switch t := v.(type) {
	case []any:
		next := make([]any, 0, len(t))
		removed := 0
		changed := false
		for _, item := range t {
			cleaned, itemRemoved, itemChanged := sanitizeResponsesReasoningContentValue(item, false)
			removed += itemRemoved
			changed = changed || itemChanged
			next = append(next, cleaned)
		}
		return next, removed, changed
	case map[string]any:
		isReasoning := parentIsReasoning
		if typ, _ := t["type"].(string); typ == "reasoning" {
			isReasoning = true
		}
		next := make(map[string]any, len(t))
		removed := 0
		changed := false
		for key, value := range t {
			if isReasoning && key == "content" {
				if items, ok := value.([]any); ok && len(items) > 0 {
					removed++
					changed = true
					continue
				}
			}
			cleaned, itemRemoved, itemChanged := sanitizeResponsesReasoningContentValue(value, isReasoning)
			removed += itemRemoved
			changed = changed || itemChanged
			next[key] = cleaned
		}
		return next, removed, changed
	default:
		return v, 0, false
	}
}
