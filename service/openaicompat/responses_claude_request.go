// Copyright (C) 2023-2026 QuantumNous and RenewAPI contributors.
// AGPL-3.0-or-later. Adapted from QuantumNous/new-api@6370b2942416,
// relaykit/relayconvert/internal/oai_responses/to_claude_messages_req.go.
package openaicompat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ResponsesRequestToClaudeRequest(req *dto.OpenAIResponsesRequest) (*dto.ClaudeRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("Responses request is nil")
	}
	out := &dto.ClaudeRequest{Model: req.Model, Stream: req.Stream, MaxTokens: req.MaxOutputTokens,
		Temperature: req.Temperature, TopP: req.TopP, Metadata: req.Metadata, ServiceTier: req.ServiceTier}
	if len(req.Instructions) > 0 {
		var text string
		if err := common.Unmarshal(req.Instructions, &text); err != nil {
			return nil, fmt.Errorf("instructions must be a string: %w", err)
		}
		out.System = text
	}
	tools := responsesToolsToChatTools(req.Tools)
	declarations := make([]any, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, map[string]any{"name": tool.Function.Name, "description": tool.Function.Description, "input_schema": tool.Function.Parameters})
	}
	if len(declarations) > 0 {
		out.Tools = declarations
	}
	choice := &dto.ClaudeToolChoice{Type: "auto"}
	if len(req.ToolChoice) > 0 {
		if common.GetJsonType(req.ToolChoice) == "string" {
			var value string
			if err := common.Unmarshal(req.ToolChoice, &value); err != nil {
				return nil, err
			}
			switch value {
			case "auto", "none":
				choice.Type = value
			case "required":
				choice.Type = "any"
			default:
				return nil, fmt.Errorf("unsupported tool_choice %q", value)
			}
		} else {
			var value struct {
				Type string `json:"type"`
				Name string `json:"name"`
			}
			if err := common.Unmarshal(req.ToolChoice, &value); err != nil {
				return nil, err
			}
			if value.Type != "function" || value.Name == "" {
				return nil, fmt.Errorf("unsupported Responses tool choice")
			}
			choice.Type = "tool"
			choice.Name = value.Name
		}
	}
	if len(req.ParallelToolCalls) > 0 {
		var parallel bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallel); err != nil {
			return nil, err
		}
		choice.DisableParallelToolUse = !parallel
	}
	if len(declarations) > 0 || len(req.ToolChoice) > 0 {
		out.ToolChoice = choice
	}
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		if req.Reasoning.Effort == "none" {
			out.Thinking = &dto.Thinking{Type: "disabled"}
		} else {
			out.Thinking = &dto.Thinking{Type: "adaptive"}
			out.OutputConfig, _ = common.Marshal(map[string]any{"effort": req.Reasoning.Effort})
		}
	}
	if len(req.Text) > 0 {
		var text struct {
			Format common.RawMessage `json:"format"`
		}
		if err := common.Unmarshal(req.Text, &text); err != nil {
			return nil, err
		}
		if len(text.Format) > 0 {
			config := map[string]any{}
			if len(out.OutputConfig) > 0 {
				if err := common.Unmarshal(out.OutputConfig, &config); err != nil {
					return nil, err
				}
			}
			var format map[string]any
			if err := common.Unmarshal(text.Format, &format); err != nil {
				return nil, err
			}
			switch format["type"] {
			case "text":
			case "json_schema":
				config["format"] = map[string]any{"type": "json_schema", "schema": format["schema"]}
			default:
				return nil, fmt.Errorf("unsupported Responses text format %v", format["type"])
			}
			out.OutputConfig, _ = common.Marshal(config)
		}
	}
	appendBlock := func(role string, parts []dto.ClaudeMediaMessage) {
		if len(parts) == 0 {
			return
		}
		if len(out.Messages) > 0 && out.Messages[len(out.Messages)-1].Role == role {
			last := &out.Messages[len(out.Messages)-1]
			previous, _ := last.ParseContent()
			last.Content = append(previous, parts...)
		} else {
			out.Messages = append(out.Messages, dto.ClaudeMessage{Role: role, Content: parts})
		}
	}
	if common.GetJsonType(req.Input) == "string" {
		var text string
		if err := common.Unmarshal(req.Input, &text); err != nil {
			return nil, err
		}
		out.Messages = []dto.ClaudeMessage{{Role: "user", Content: text}}
		return out, nil
	}
	var items []map[string]any
	if err := common.Unmarshal(req.Input, &items); err != nil {
		return nil, fmt.Errorf("invalid Responses input: %w", err)
	}
	for _, item := range items {
		kind, _ := item["type"].(string)
		switch kind {
		case "function_call":
			id, _ := item["call_id"].(string)
			name, _ := item["name"].(string)
			if id == "" || name == "" {
				return nil, fmt.Errorf("function_call requires call_id and name")
			}
			var input map[string]any
			if raw, ok := item["arguments"].(string); ok {
				if err := common.UnmarshalJsonStr(raw, &input); err != nil {
					return nil, fmt.Errorf("invalid function arguments: %w", err)
				}
			} else {
				input, _ = item["arguments"].(map[string]any)
				if input == nil && item["arguments"] != nil {
					return nil, fmt.Errorf("function arguments must be a JSON object or JSON object string")
				}
			}
			if input == nil {
				input = map[string]any{}
			}
			appendBlock("assistant", []dto.ClaudeMediaMessage{{Type: "tool_use", Id: id, Name: name, Input: input}})
		case "function_call_output":
			id, _ := item["call_id"].(string)
			if id == "" {
				return nil, fmt.Errorf("function_call_output requires call_id")
			}
			content, err := responsesContentToClaude(item["output"])
			if err != nil {
				return nil, err
			}
			appendBlock("user", []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: id, Content: content}})
		case "", "message":
			role, _ := item["role"].(string)
			parts, err := responsesContentToClaude(item["content"])
			if err != nil {
				return nil, err
			}
			if role == "system" || role == "developer" {
				existing := out.ParseSystem()
				if out.IsStringSystem() {
					existing = []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer(out.GetStringSystem())}}
				}
				out.System = append(existing, parts...)
			} else {
				if role != "user" && role != "assistant" {
					return nil, fmt.Errorf("invalid message role %q", role)
				}
				appendBlock(role, parts)
			}
		default:
			return nil, fmt.Errorf("Responses input %q cannot be converted to Messages", kind)
		}
	}
	return out, nil
}

func responsesContentToClaude(value any) ([]dto.ClaudeMediaMessage, error) {
	if text, ok := value.(string); ok {
		return []dto.ClaudeMediaMessage{{Type: "text", Text: common.GetPointer(text)}}, nil
	}
	if value == nil {
		return nil, nil
	}
	blocks, err := common.Any2Type[[]map[string]any](value)
	if err != nil {
		return nil, err
	}
	parts := make([]dto.ClaudeMediaMessage, 0, len(blocks))
	for _, block := range blocks {
		kind, _ := block["type"].(string)
		switch kind {
		case "input_text", "output_text", "text":
			text, _ := block["text"].(string)
			parts = append(parts, dto.ClaudeMediaMessage{Type: "text", Text: &text})
		case "input_image":
			url, _ := block["image_url"].(string)
			source := &dto.ClaudeMessageSource{Type: "url", Url: url}
			if strings.HasPrefix(url, "data:") {
				header, data, ok := strings.Cut(strings.TrimPrefix(url, "data:"), ",")
				if !ok {
					return nil, fmt.Errorf("invalid image data URL")
				}
				source = &dto.ClaudeMessageSource{Type: "base64", MediaType: strings.TrimSuffix(header, ";base64"), Data: data}
			}
			parts = append(parts, dto.ClaudeMediaMessage{Type: "image", Source: source})
		default:
			return nil, fmt.Errorf("Responses content %q cannot be converted to Messages", kind)
		}
	}
	return parts, nil
}
