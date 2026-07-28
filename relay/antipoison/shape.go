package antipoison

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ShapeCheckEnabled returns whether shape check is enabled for the channel.
func ShapeCheckEnabled(cfg Config) bool {
	return cfg.Enabled && cfg.ShapeCheck
}

// Known finish_reason values for OpenAI
var knownOpenAIFinishReasons = map[string]bool{
	"stop":           true,
	"length":         true,
	"tool_calls":     true,
	"function_call":  true,
	"content_filter": true,
}

// Known stop_reason values for Claude
var knownClaudeStopReasons = map[string]bool{
	"end_turn":       true,
	"max_tokens":     true,
	"stop_sequence":  true,
	"tool_use":       true,
	"content_filter": true,
}

var (
	openAIChatCompletionIDPattern = regexp.MustCompile(`^chatcmpl-[A-Za-z0-9\-]+$`)
	openAIResponsesIDPattern      = regexp.MustCompile(`^resp_[A-Za-z0-9\-]+$`)
	claudeIDPattern               = regexp.MustCompile(`^msg_[A-Za-z0-9]+$`)
)

func isOpenAIChatCompatibleID(id string) bool {
	return openAIChatCompletionIDPattern.MatchString(id) || openAIResponsesIDPattern.MatchString(id)
}

// ValidateClaudeResponseShape validates the structural fingerprint of a Claude response.
func ValidateClaudeResponseShape(resp *dto.ClaudeResponse, requestModel string, cfg Config) error {
	if resp == nil || !ShapeCheckEnabled(cfg) {
		return nil
	}

	// Validate ID format: must match ^msg_[A-Za-z0-9]+$
	if resp.Id != "" {
		if !claudeIDPattern.MatchString(resp.Id) {
			return shapeError("claude id malformed: %q", resp.Id)
		}
	}

	// Validate model field contains request model core tokens
	if resp.Model != "" && requestModel != "" {
		if !modelMatchesClaude(resp.Model, requestModel) {
			return shapeError("claude model mismatch: response=%q request=%q", resp.Model, requestModel)
		}
	}

	// Validate stop_reason is in the known set
	if resp.StopReason != "" && !knownClaudeStopReasons[resp.StopReason] {
		return shapeError("claude stop_reason unknown: %q", resp.StopReason)
	}

	return nil
}

// ValidateOpenAIResponseShape validates the structural fingerprint of an OpenAI chat response.
func ValidateOpenAIResponseShape(resp *dto.OpenAITextResponse, requestModel string, cfg Config) error {
	if resp == nil || !ShapeCheckEnabled(cfg) {
		return nil
	}

	// Validate ID format. NewAPI can route chat.completions through the
	// Responses API, in which case a resp_* ID is still a valid local shape.
	if resp.Id != "" {
		if !isOpenAIChatCompatibleID(resp.Id) {
			return shapeError("openai id malformed: %q", resp.Id)
		}
	}

	// Validate object field
	if resp.Object != "" && resp.Object != "chat.completion" && resp.Object != "chat.completion.chunk" {
		return shapeError("openai object invalid: %q", resp.Object)
	}

	// Validate model field contains request model core tokens
	if resp.Model != "" && requestModel != "" {
		if !modelMatchesOpenAI(resp.Model, requestModel) {
			return shapeError("openai model mismatch: response=%q request=%q", resp.Model, requestModel)
		}
	}

	// Validate finish_reason in choices
	for i, choice := range resp.Choices {
		if choice.FinishReason != "" && !knownOpenAIFinishReasons[choice.FinishReason] {
			return shapeError("openai finish_reason unknown in choice %d: %q", i, choice.FinishReason)
		}
	}

	return nil
}

// ValidateOpenAIStreamChunkShape validates the shape of an OpenAI stream chunk.
func ValidateOpenAIStreamChunkShape(chunk *dto.ChatCompletionsStreamResponse, requestModel string, cfg Config) error {
	if chunk == nil || !ShapeCheckEnabled(cfg) {
		return nil
	}

	// Validate ID format
	if chunk.Id != "" {
		if !isOpenAIChatCompatibleID(chunk.Id) {
			return shapeError("openai stream id malformed: %q", chunk.Id)
		}
	}

	// Validate object field
	if chunk.Object != "" && chunk.Object != "chat.completion.chunk" {
		return shapeError("openai stream object invalid: %q", chunk.Object)
	}

	// Validate model
	if chunk.Model != "" && requestModel != "" {
		if !modelMatchesOpenAI(chunk.Model, requestModel) {
			return shapeError("openai stream model mismatch: response=%q request=%q", chunk.Model, requestModel)
		}
	}

	// Validate finish_reason in choices
	for i, choice := range chunk.Choices {
		if choice.FinishReason != nil && *choice.FinishReason != "" && !knownOpenAIFinishReasons[*choice.FinishReason] {
			return shapeError("openai stream finish_reason unknown in choice %d: %q", i, *choice.FinishReason)
		}
	}

	return nil
}

// ValidateResponsesResponseShape validates the structural fingerprint of an OpenAI Responses API response.
func ValidateResponsesResponseShape(resp *dto.OpenAIResponsesResponse, requestModel string, cfg Config) error {
	if resp == nil || !ShapeCheckEnabled(cfg) {
		return nil
	}

	// Validate ID format: must match ^resp_[A-Za-z0-9\-]+$
	if resp.ID != "" {
		if !openAIResponsesIDPattern.MatchString(resp.ID) {
			return shapeError("responses id malformed: %q", resp.ID)
		}
	}

	// Validate object field
	if resp.Object != "" && resp.Object != "response" && resp.Object != "response.compaction" {
		return shapeError("responses object invalid: %q", resp.Object)
	}

	// Validate model field
	if resp.Model != "" && requestModel != "" {
		if !modelMatchesOpenAI(resp.Model, requestModel) {
			return shapeError("responses model mismatch: response=%q request=%q", resp.Model, requestModel)
		}
	}

	return nil
}

// modelMatchesClaude checks if the response model matches the request model for Claude.
func modelMatchesClaude(responseModel, requestModel string) bool {
	// Both must contain "claude"
	if !strings.Contains(strings.ToLower(responseModel), "claude") {
		return false
	}
	if !strings.Contains(strings.ToLower(requestModel), "claude") {
		return false
	}

	// Extract the tier (opus, sonnet, haiku)
	requestTier := extractClaudeTier(requestModel)
	responseTier := extractClaudeTier(responseModel)

	// If both have a tier, they must match
	if requestTier != "" && responseTier != "" && requestTier != responseTier {
		return false
	}

	return true
}

// extractClaudeTier extracts the model tier from a Claude model name.
func extractClaudeTier(model string) string {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "opus") {
		return "opus"
	}
	if strings.Contains(lower, "sonnet") {
		return "sonnet"
	}
	if strings.Contains(lower, "haiku") {
		return "haiku"
	}
	return ""
}

// modelMatchesOpenAI checks if the response model matches the request model for OpenAI.
func modelMatchesOpenAI(responseModel, requestModel string) bool {
	// Normalize both to lowercase
	respLower := strings.ToLower(responseModel)
	reqLower := strings.ToLower(requestModel)

	// Extract the core model family
	respFamily := extractOpenAIFamily(respLower)
	reqFamily := extractOpenAIFamily(reqLower)

	// If both have a family, they must match
	if respFamily != "" && reqFamily != "" && respFamily != reqFamily {
		return false
	}

	// Allow exact match or response contains request
	if respLower == reqLower || strings.Contains(respLower, reqLower) {
		return true
	}

	// Allow request contains response (e.g., request="gpt-4o-2024-05-13", response="gpt-4o")
	if strings.Contains(reqLower, respLower) {
		return true
	}

	return false
}

// extractOpenAIFamily extracts the model family from an OpenAI model name.
func extractOpenAIFamily(model string) string {
	// Common families: gpt-3.5, gpt-4, gpt-4o, gpt-5, o1, o3
	if strings.Contains(model, "gpt-5") {
		return "gpt-5"
	}
	if strings.Contains(model, "gpt-4o") {
		return "gpt-4o"
	}
	if strings.Contains(model, "gpt-4") {
		return "gpt-4"
	}
	if strings.Contains(model, "gpt-3.5") {
		return "gpt-3.5"
	}
	if strings.Contains(model, "o3-mini") {
		return "o3-mini"
	}
	if strings.Contains(model, "o3") {
		return "o3"
	}
	if strings.Contains(model, "o1-mini") {
		return "o1-mini"
	}
	if strings.Contains(model, "o1") {
		return "o1"
	}
	return ""
}

// shapeError constructs a shape validation error message.
func shapeError(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("shape check failed: %s", msg)
}

// ShapeCheckFailureError returns a user-facing error for shape check failures.
func ShapeCheckFailureError(cfg Config) error {
	if cfg.FailureMode == FailureModeWarn {
		common.SysLog("shape check failed but allowing (warn mode)")
		return nil
	}
	return fmt.Errorf("response shape validation failed")
}
