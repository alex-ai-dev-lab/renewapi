// Copyright (C) 2023-2026 QuantumNous and RenewAPI contributors.
// AGPL-3.0-or-later. Adapted from QuantumNous/new-api@6370b2942416,
// relaykit/relayconvert/internal/claude_messages/to_oai_responses_req.go.
// See UPSTREAM_PORTS.md for the adaptation boundary.
package openaicompat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ClaudeRequestToResponsesRequest retains block order and tool-result pairing
// without compressing the request into Chat Completions first.
func ClaudeRequestToResponsesRequest(req *dto.ClaudeRequest) (*dto.OpenAIResponsesRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("Claude request is nil")
	}
	input, err := claudeMessagesToResponsesInput(req.Messages)
	if err != nil {
		return nil, err
	}
	tools, err := claudeToolsToResponsesTools(req.Tools)
	if err != nil {
		return nil, err
	}
	choice, parallel, err := claudeToolChoiceToResponses(req.ToolChoice)
	if err != nil {
		return nil, err
	}
	var instructions []byte
	if req.System != nil {
		var parts []string
		if req.IsStringSystem() {
			parts = append(parts, req.GetStringSystem())
		} else {
			for _, part := range req.ParseSystem() {
				if part.Type != "text" {
					return nil, fmt.Errorf("unsupported system block %q", part.Type)
				}
				parts = append(parts, part.GetText())
			}
		}
		instructions, err = common.Marshal(strings.Join(parts, "\n\n"))
		if err != nil {
			return nil, err
		}
	}
	out := &dto.OpenAIResponsesRequest{Model: req.Model, Input: input, Instructions: instructions,
		Stream: req.Stream, MaxOutputTokens: req.MaxTokens, Temperature: req.Temperature, TopP: req.TopP,
		Tools: tools, ToolChoice: choice, ParallelToolCalls: parallel, Metadata: req.Metadata, ServiceTier: req.ServiceTier}
	if out.MaxOutputTokens == nil {
		out.MaxOutputTokens = req.MaxTokensToSample
	}
	effort, err := ClaudeReasoningEffort(req)
	if err != nil {
		return nil, err
	}
	if effort != "" {
		out.Reasoning = &dto.Reasoning{Effort: effort}
	}
	format, err := ClaudeOutputJSONFormat(req)
	if err != nil {
		return nil, err
	}
	if format != nil {
		format["type"] = "json_schema"
		out.Text, err = common.Marshal(map[string]any{"format": format})
		if err != nil {
			return nil, err
		}
	}

	if len(req.ContextManagement) > 0 || len(req.Container) > 0 || len(req.McpServers) > 0 || len(req.OutputFormat) > 0 || len(req.Speed) > 0 {
		return nil, fmt.Errorf("Claude request contains an extension without a Responses representation")
	}
	if len(req.StopSequences) > 0 || req.TopK != nil {
		return nil, fmt.Errorf("Claude stop_sequences/top_k are not supported by Responses")
	}
	return out, nil
}

// ClaudeReasoningEffort maps intent without increasing the output token budget.
// Budget buckets follow CLIProxyAPI's thinking conversion; explicit disable wins.
func ClaudeReasoningEffort(req *dto.ClaudeRequest) (string, error) {
	var config struct {
		Effort string `json:"effort"`
	}
	if len(req.OutputConfig) > 0 {
		if err := common.Unmarshal(req.OutputConfig, &config); err != nil {
			return "", err
		}
	}
	if req.Thinking == nil {
		return config.Effort, nil
	}
	switch req.Thinking.Type {
	case "disabled":
		return "none", nil
	case "adaptive":
		if config.Effort != "" {
			return config.Effort, nil
		}
		return "medium", nil
	case "enabled":
		if config.Effort != "" {
			return config.Effort, nil
		}
		budget := req.Thinking.GetBudgetTokens()
		switch {
		case budget <= 2048:
			return "low", nil
		case budget <= 8192:
			return "medium", nil
		default:
			return "high", nil
		}
	default:
		return "", fmt.Errorf("unsupported thinking type %q", req.Thinking.Type)
	}
}

func claudeToolsToResponsesTools(value any) (common.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	tools, err := common.Any2Type[[]map[string]any](value)
	if err != nil {
		return nil, err
	}
	converted := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		kind, _ := tool["type"].(string)
		if kind != "" && kind != "custom" {
			return nil, fmt.Errorf("unsupported Claude tool type %q", kind)
		}
		name, _ := tool["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("tool name is required")
		}
		item := map[string]any{"type": "function", "name": name, "parameters": tool["input_schema"]}
		for _, key := range []string{"description", "strict"} {
			if v, ok := tool[key]; ok {
				item[key] = v
			}
		}
		converted = append(converted, item)
	}
	return common.Marshal(converted)
}

func claudeMessagesToResponsesInput(messages []dto.ClaudeMessage) (common.RawMessage, error) {
	input := make([]map[string]any, 0, len(messages))
	for messageIndex := range messages {
		message := messages[messageIndex]
		role := strings.TrimSpace(message.Role)
		if role == "" {
			continue
		}
		if message.IsStringContent() {
			input = append(input, map[string]any{
				"role":    role,
				"content": message.GetStringContent(),
			})
			continue
		}

		blocks, err := message.ParseContent()
		if err != nil {
			return nil, fmt.Errorf("messages[%d].content: %w", messageIndex, err)
		}
		contentParts := make([]map[string]any, 0, len(blocks))
		flushContent := func() {
			if len(contentParts) == 0 {
				return
			}
			input = append(input, map[string]any{
				"role":    role,
				"content": contentParts,
			})
			contentParts = nil
		}

		for blockIndex := range blocks {
			block := blocks[blockIndex]
			switch block.Type {
			case "thinking":
				// Responses summaries returned through Messages are not Anthropic
				// signed thinking. Preserve their visible text on a stateless turn;
				// never reinterpret a foreign signature as Responses ciphertext.
				if block.Signature != "" {
					return nil, fmt.Errorf("signed thinking history cannot cross provider protocols")
				}
				if block.Thinking != nil {
					contentParts = append(contentParts, map[string]any{"type": "output_text", "text": *block.Thinking})
				}
			case "text", "input_text":
				partType := "input_text"
				if role == "assistant" {
					partType = "output_text"
				}
				contentParts = append(contentParts, map[string]any{
					"type": partType,
					"text": block.GetText(),
				})
			case "image":
				if source := ClaudeSourceURL(block.Source); source != "" {
					contentParts = append(contentParts, map[string]any{
						"type":      "input_image",
						"image_url": source,
					})
				}
			case "document":
				if source := ClaudeSourceURL(block.Source); source != "" {
					contentParts = append(contentParts, map[string]any{
						"type":      "input_file",
						"file_data": source,
					})
				}
			case "tool_use":
				flushContent()
				arguments, err := common.Marshal(block.Input)
				if err != nil {
					return nil, fmt.Errorf("messages[%d].content[%d].input: %w", messageIndex, blockIndex, err)
				}
				if block.Input == nil {
					arguments = []byte("{}")
				}
				input = append(input, map[string]any{
					"type":      "function_call",
					"call_id":   block.Id,
					"name":      block.Name,
					"arguments": string(arguments),
				})
			case "tool_result":
				flushContent()
				output, err := claudeToolResultToResponsesOutput(block.Content)
				if err != nil {
					return nil, fmt.Errorf("messages[%d].content[%d].content: %w", messageIndex, blockIndex, err)
				}
				input = append(input, map[string]any{
					"type":    "function_call_output",
					"call_id": block.ToolUseId,
					"output":  output,
				})
			default:
				return nil, fmt.Errorf("unsupported Claude history block %q", block.Type)
			}
		}
		flushContent()
	}
	return common.Marshal(input)
}

func claudeToolChoiceToResponses(value any) (common.RawMessage, common.RawMessage, error) {
	if value == nil {
		return nil, nil, nil
	}
	choice, err := common.Any2Type[dto.ClaudeToolChoice](value)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid Claude tool_choice: %w", err)
	}

	var converted any
	switch choice.Type {
	case "", "auto":
		converted = "auto"
	case "any":
		converted = "required"
	case "none":
		converted = "none"
	case "tool":
		converted = map[string]any{"type": "function", "name": choice.Name}
	default:
		return nil, nil, fmt.Errorf("unsupported Claude tool_choice type %q", choice.Type)
	}
	toolChoice, err := common.Marshal(converted)
	if err != nil {
		return nil, nil, err
	}

	var parallelToolCalls common.RawMessage
	if choice.DisableParallelToolUse && choice.Type != "none" {
		parallelToolCalls, err = common.Marshal(false)
		if err != nil {
			return nil, nil, err
		}
	}
	return toolChoice, parallelToolCalls, nil
}

func claudeToolResultToResponsesOutput(content any) (any, error) {
	if content == nil {
		return "", nil
	}
	if text, ok := content.(string); ok {
		return text, nil
	}
	blocks, err := common.Any2Type[[]dto.ClaudeMediaMessage](content)
	if err != nil {
		return content, nil
	}
	parts := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text", "input_text":
			parts = append(parts, map[string]any{"type": "input_text", "text": block.GetText()})
		case "image":
			if source := ClaudeSourceURL(block.Source); source != "" {
				parts = append(parts, map[string]any{"type": "input_image", "image_url": source})
			}
		case "document":
			if source := ClaudeSourceURL(block.Source); source != "" {
				parts = append(parts, map[string]any{"type": "input_file", "file_data": source})
			}
		}
	}
	if len(parts) == 0 {
		return content, nil
	}
	return parts, nil
}

func ClaudeSourceURL(source *dto.ClaudeMessageSource) string {
	if source == nil {
		return ""
	}
	if strings.TrimSpace(source.Url) != "" {
		return source.Url
	}
	data := common.Interface2String(source.Data)
	if data == "" {
		return ""
	}
	if strings.HasPrefix(data, "data:") {
		return data
	}
	return fmt.Sprintf("data:%s;base64,%s", source.MediaType, data)
}

// ClaudeOutputJSONFormat normalizes the shared JSON-schema representation.
func ClaudeOutputJSONFormat(req *dto.ClaudeRequest) (map[string]any, error) {
	if len(req.OutputConfig) == 0 {
		return nil, nil
	}
	var config struct {
		Format map[string]any `json:"format"`
	}
	if err := common.Unmarshal(req.OutputConfig, &config); err != nil {
		return nil, err
	}
	if config.Format == nil {
		return nil, nil
	}
	if config.Format["type"] != "json_schema" {
		return nil, fmt.Errorf("unsupported output format %v", config.Format["type"])
	}
	if config.Format["schema"] == nil {
		return nil, fmt.Errorf("output format schema is required")
	}
	out := map[string]any{"name": "response", "schema": config.Format["schema"]}
	for _, key := range []string{"name", "description", "strict"} {
		if v, ok := config.Format[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}
