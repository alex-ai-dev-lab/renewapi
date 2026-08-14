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
		extractResponses(builder, typed.Instructions, typed.Input)
	case *dto.OpenAIResponsesCompactionRequest:
		extractResponses(builder, typed.Instructions, typed.Input)
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

func extractResponses(builder *snapshotBuilder, instructions, input []byte) {
	appendJSONString(builder, "instructions", instructions)
	request := &dto.OpenAIResponsesRequest{Input: input}
	for _, item := range request.ParseInput() {
		if item.Type == "input_text" && !builder.append("user", item.Text) {
			return
		}
	}
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
	case []any:
		for _, item := range typed {
			if !extractStringLike(builder, role, item) {
				return false
			}
		}
	case map[string]any:
		for _, key := range []string{"text", "content", "output"} {
			if text, ok := typed[key].(string); ok && text != "" {
				return builder.append(role, text)
			}
		}
	case dto.MediaContent:
		if typed.Type == dto.ContentTypeText {
			return builder.append(role, typed.Text)
		}
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

func appendJSONString(builder *snapshotBuilder, role string, raw []byte) {
	if len(raw) == 0 || common.GetJsonType(raw) != "string" {
		return
	}
	var value string
	if err := common.Unmarshal(raw, &value); err == nil {
		builder.append(role, value)
	}
}
