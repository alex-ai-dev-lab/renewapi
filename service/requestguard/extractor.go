package requestguard

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type snapshotBuilder struct {
	limit     int
	runes     int
	truncated bool
	segments  []Segment
}

func newSnapshotBuilder(limit int) *snapshotBuilder {
	return &snapshotBuilder{limit: limit, segments: make([]Segment, 0, 8)}
}

func (b *snapshotBuilder) append(role, text string) bool {
	if b == nil || b.truncated {
		return false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if b.runes >= b.limit {
		b.truncated = true
		return false
	}
	remaining := b.limit - b.runes
	end := len(text)
	count := 0
	for index := range text {
		if count == remaining {
			end = index
			b.truncated = true
			break
		}
		count++
	}
	selected := text[:end]
	if selected != "" {
		b.segments = append(b.segments, Segment{Role: strings.TrimSpace(role), Text: selected})
		b.runes += count
	}
	return !b.truncated
}

func (b *snapshotBuilder) snapshot() Snapshot {
	segments := append([]Segment(nil), b.segments...)
	return Snapshot{
		Segments:  segments,
		RuneCount: b.runes,
		Truncated: b.truncated,
		Digest:    digestSegments(segments),
	}
}

func Extract(request dto.Request, info *relaycommon.RelayInfo, limit int, inputMode string) (Snapshot, error) {
	if inputMode != "full_client_controlled" {
		return Snapshot{}, fmt.Errorf("unsupported input mode %q", inputMode)
	}
	if limit < 1 {
		return Snapshot{}, fmt.Errorf("input limit must be positive")
	}
	builder := newSnapshotBuilder(limit)

	switch typed := request.(type) {
	case *dto.GeneralOpenAIRequest:
		extractOpenAI(builder, typed)
	case *dto.OpenAIResponsesRequest:
		if err := extractResponses(builder, typed.Instructions, typed.Input); err != nil {
			return Snapshot{}, err
		}
	case *dto.OpenAIResponsesCompactionRequest:
		if err := extractResponses(builder, typed.Instructions, typed.Input); err != nil {
			return Snapshot{}, err
		}
	case *dto.ClaudeRequest:
		extractClaude(builder, typed)
	case *dto.GeminiChatRequest:
		extractGemini(builder, typed)
	case *dto.ImageRequest:
		builder.append("user", typed.Prompt)
	case *dto.AudioRequest:
		builder.append("instructions", typed.Instructions)
		builder.append("user", typed.Input)
	case *dto.EmbeddingRequest:
		extractStringLike(builder, "user", typed.Input)
	case *dto.RerankRequest:
		builder.append("query", typed.Query)
		for _, document := range typed.Documents {
			if !extractStringLike(builder, "document", document) {
				break
			}
		}
	default:
		return Snapshot{}, fmt.Errorf("unsupported request type %T", request)
	}
	return builder.snapshot(), nil
}

func extractOpenAI(builder *snapshotBuilder, request *dto.GeneralOpenAIRequest) {
	if request == nil {
		return
	}
	extractStringLike(builder, "prompt", request.Prompt)
	extractStringLike(builder, "input", request.Input)
	if request.Instruction != "" {
		builder.append("instructions", request.Instruction)
	}
	for _, message := range request.Messages {
		if builder.truncated {
			return
		}
		extractStringLike(builder, message.Role, message.Content)
		for _, toolCall := range message.ParseToolCalls() {
			if !builder.append("tool_call", toolCall.Function.Name+"\n"+toolCall.Function.Arguments) {
				return
			}
		}
	}
}

type responsesGuardInput struct {
	Type    string            `json:"type,omitempty"`
	Role    string            `json:"role,omitempty"`
	Text    common.RawMessage `json:"text,omitempty"`
	Content common.RawMessage `json:"content,omitempty"`
	Output  common.RawMessage `json:"output,omitempty"`
}

func extractResponses(builder *snapshotBuilder, instructions, input []byte) error {
	appendJSONString(builder, "instructions", instructions)
	if len(input) == 0 || builder.truncated {
		return nil
	}
	switch common.GetJsonType(input) {
	case "string":
		return appendRequiredJSONString(builder, "user", input)
	case "object":
		return extractResponsesObject(builder, "user", input)
	case "array":
		var walkErr error
		err := common.WalkJsonArray(input, func(raw common.RawMessage) bool {
			walkErr = extractResponsesValue(builder, "user", raw)
			return walkErr == nil && !builder.truncated
		})
		if err != nil {
			return fmt.Errorf("decode Responses input array: %w", err)
		}
		return walkErr
	default:
		return fmt.Errorf("unsupported Responses input JSON type %q", common.GetJsonType(input))
	}
}

func extractResponsesValue(builder *snapshotBuilder, role string, raw common.RawMessage) error {
	if builder == nil || builder.truncated || len(raw) == 0 {
		return nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		return appendRequiredJSONString(builder, role, raw)
	case "object":
		return extractResponsesObject(builder, role, raw)
	case "array":
		var walkErr error
		err := common.WalkJsonArray(raw, func(item common.RawMessage) bool {
			walkErr = extractResponsesValue(builder, role, item)
			return walkErr == nil && !builder.truncated
		})
		if err != nil {
			return err
		}
		return walkErr
	case "null":
		return nil
	default:
		return fmt.Errorf("unsupported Responses text JSON type %q", common.GetJsonType(raw))
	}
}

func extractResponsesObject(builder *snapshotBuilder, inheritedRole string, raw common.RawMessage) error {
	var item responsesGuardInput
	if err := common.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("decode Responses input object: %w", err)
	}
	itemType := strings.ToLower(strings.TrimSpace(item.Type))
	if isBinaryContentType(itemType) {
		return nil
	}
	role := strings.TrimSpace(item.Role)
	if role == "" {
		role = inheritedRole
	}
	switch itemType {
	case "input_text", "output_text", "text":
		return appendRequiredJSONString(builder, role, item.Text)
	case "function_call_output":
		return extractResponsesValue(builder, "tool", item.Output)
	case "tool_result":
		return extractResponsesValue(builder, "tool", item.Content)
	case "message", "":
		if len(item.Text) > 0 {
			if err := appendRequiredJSONString(builder, role, item.Text); err != nil || builder.truncated {
				return err
			}
		}
		if len(item.Content) > 0 {
			return extractResponsesValue(builder, role, item.Content)
		}
		return nil
	default:
		// Unknown typed objects are deliberately ignored. Recursing arbitrary
		// fields risks treating opaque provider or binary payloads as prompts.
		return nil
	}
}

func appendRequiredJSONString(builder *snapshotBuilder, role string, raw common.RawMessage) error {
	if len(raw) == 0 || common.GetJsonType(raw) == "null" {
		return nil
	}
	if common.GetJsonType(raw) != "string" {
		return fmt.Errorf("expected JSON string, got %s", common.GetJsonType(raw))
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return err
	}
	builder.append(role, value)
	return nil
}

func extractClaude(builder *snapshotBuilder, request *dto.ClaudeRequest) {
	if request == nil {
		return
	}
	extractStringLike(builder, "system", request.System)
	builder.append("prompt", request.Prompt)
	for _, message := range request.Messages {
		if !extractStringLike(builder, message.Role, message.Content) {
			return
		}
	}
}

func extractGemini(builder *snapshotBuilder, request *dto.GeminiChatRequest) {
	if request == nil {
		return
	}
	if request.SystemInstructions != nil {
		extractGeminiContent(builder, "system", *request.SystemInstructions)
	}
	for _, content := range request.Contents {
		if !extractGeminiContent(builder, content.Role, content) {
			return
		}
	}
	for i := range request.Requests {
		extractGemini(builder, &request.Requests[i])
		if builder.truncated {
			return
		}
	}
}

func extractGeminiContent(builder *snapshotBuilder, role string, content dto.GeminiChatContent) bool {
	for _, part := range content.Parts {
		if part.Text != "" && !builder.append(role, part.Text) {
			return false
		}
		if part.ExecutableCode != nil && !builder.append("code", part.ExecutableCode.Code) {
			return false
		}
		if part.CodeExecutionResult != nil && !builder.append("tool", part.CodeExecutionResult.Output) {
			return false
		}
	}
	return !builder.truncated
}

func extractStringLike(builder *snapshotBuilder, role string, value any) bool {
	if builder == nil || builder.truncated || value == nil {
		return builder != nil && !builder.truncated
	}
	switch typed := value.(type) {
	case string:
		return builder.append(role, typed)
	case []string:
		for _, item := range typed {
			if !builder.append(role, item) {
				return false
			}
		}
	case []dto.MediaContent:
		for _, item := range typed {
			if item.Type == dto.ContentTypeText && !builder.append(role, item.Text) {
				return false
			}
		}
	case []dto.ClaudeMediaMessage:
		for i := range typed {
			if !extractClaudeMediaMessage(builder, role, &typed[i]) {
				return false
			}
		}
	case []any:
		for _, item := range typed {
			if !extractStringLike(builder, role, item) {
				return false
			}
		}
	case map[string]any:
		itemType, _ := typed["type"].(string)
		itemType = strings.ToLower(strings.TrimSpace(itemType))
		if isBinaryContentType(itemType) {
			return true
		}
		switch itemType {
		case "tool_result":
			return extractStringLike(builder, "tool", typed["content"])
		case "function_call_output":
			return extractStringLike(builder, "tool", typed["output"])
		case "text", "input_text", "output_text":
			return extractStringLike(builder, role, typed["text"])
		case "message":
			return extractStringLike(builder, role, typed["content"])
		case "":
			for _, key := range []string{"text", "content", "output"} {
				if value, ok := typed[key]; ok && !extractStringLike(builder, role, value) {
					return false
				}
			}
		}
	case dto.MediaContent:
		if typed.Type == dto.ContentTypeText {
			return builder.append(role, typed.Text)
		}
	case dto.ClaudeMediaMessage:
		return extractClaudeMediaMessage(builder, role, &typed)
	case *dto.ClaudeMediaMessage:
		return extractClaudeMediaMessage(builder, role, typed)
	case dto.RerankDocument:
		return extractStringLike(builder, role, typed.Text)
	case *dto.RerankDocument:
		if typed != nil {
			return extractStringLike(builder, role, typed.Text)
		}
	default:
		_ = common.Interface2String(typed)
	}
	return !builder.truncated
}

func extractClaudeMediaMessage(builder *snapshotBuilder, role string, item *dto.ClaudeMediaMessage) bool {
	if item == nil || isBinaryContentType(item.Type) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(item.Type)) {
	case "tool_result":
		return extractStringLike(builder, "tool", item.Content)
	case "text", "input_text", "output_text", "":
		return builder.append(role, item.GetText())
	default:
		return true
	}
}

func isBinaryContentType(itemType string) bool {
	itemType = strings.ToLower(strings.TrimSpace(itemType))
	for _, marker := range []string{"image", "audio", "video", "file", "binary", "blob", "screenshot"} {
		if strings.Contains(itemType, marker) {
			return true
		}
	}
	return false
}

func appendJSONString(builder *snapshotBuilder, role string, raw []byte) {
	if len(raw) == 0 || common.GetJsonType(raw) != "string" {
		return
	}
	var value string
	if err := common.Unmarshal(raw, &value); err == nil {
		builder.append(role, value)
	}
}
