package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("model is required")
	}

	out := &dto.GeneralOpenAIRequest{
		Model:         req.Model,
		Stream:        req.Stream,
		StreamOptions: req.StreamOptions,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		User:          req.User,
		Metadata:      req.Metadata,
		Store:         req.Store,
		Reasoning:     reasoningToRaw(req.Reasoning),
	}
	if req.MaxOutputTokens != nil {
		out.MaxTokens = req.MaxOutputTokens
	}
	if req.ParallelToolCalls != nil {
		if parallel, ok := rawBool(req.ParallelToolCalls); ok {
			out.ParallelTooCalls = &parallel
		}
	}
	if req.ServiceTier != "" {
		out.ServiceTier, _ = common.Marshal(req.ServiceTier)
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	out.Tools = responsesToolsToChatTools(req.Tools)
	out.ToolChoice = responsesToolChoiceToChat(req.ToolChoice)
	out.ResponseFormat = responsesTextToChatResponseFormat(req.Text)

	if instructions := rawText(req.Instructions); strings.TrimSpace(instructions) != "" {
		out.Messages = append(out.Messages, dto.Message{Role: "system", Content: instructions})
	}
	out.Messages = append(out.Messages, responsesInputToChatMessages(req.Input)...)
	if len(out.Messages) == 0 {
		out.Messages = append(out.Messages, dto.Message{Role: "user", Content: ""})
	}
	return out, nil
}

func responsesInputToChatMessages(raw json.RawMessage) []dto.Message {
	if len(raw) == 0 {
		return nil
	}
	if common.GetJsonType(raw) == "string" {
		return []dto.Message{{Role: "user", Content: rawText(raw)}}
	}
	if common.GetJsonType(raw) != "array" {
		return []dto.Message{{Role: "user", Content: rawText(raw)}}
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return []dto.Message{{Role: "user", Content: rawText(raw)}}
	}
	messages := make([]dto.Message, 0, len(items))
	for _, item := range items {
		msgs := responsesInputItemToChatMessages(item)
		messages = append(messages, msgs...)
	}
	return messages
}

func responsesInputItemToChatMessages(item map[string]any) []dto.Message {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	role := strings.TrimSpace(common.Interface2String(item["role"]))
	if role == "" {
		role = "user"
	}

	switch itemType {
	case "function_call_output":
		msg := dto.Message{
			Role:       "tool",
			ToolCallId: common.Interface2String(item["call_id"]),
		}
		msg.SetStringContent(common.Interface2String(item["output"]))
		return []dto.Message{msg}
	case "function_call":
		call := dto.ToolCallRequest{
			ID:   common.Interface2String(item["call_id"]),
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      common.Interface2String(item["name"]),
				Arguments: common.Interface2String(item["arguments"]),
			},
		}
		msg := dto.Message{Role: "assistant"}
		msg.SetNullContent()
		msg.SetToolCalls([]dto.ToolCallRequest{call})
		return []dto.Message{msg}
	case "input_text", "output_text":
		return []dto.Message{{
			Role:    role,
			Content: common.Interface2String(item["text"]),
		}}
	case "input_image", "input_file":
		return []dto.Message{{
			Role:    role,
			Content: []dto.MediaContent{responsesContentPartToChatContent(item, role)},
		}}
	}

	content, ok := item["content"]
	if !ok {
		if text := common.Interface2String(item["text"]); text != "" {
			return []dto.Message{{Role: role, Content: text}}
		}
		return nil
	}
	return []dto.Message{{Role: role, Content: responsesContentToChatContent(content, role)}}
}

func responsesContentToChatContent(content any, role string) any {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]dto.MediaContent, 0, len(v))
		for _, itemAny := range v {
			item, ok := itemAny.(map[string]any)
			if !ok {
				continue
			}
			parts = append(parts, responsesContentPartToChatContent(item, role))
		}
		if len(parts) == 1 && parts[0].Type == dto.ContentTypeText {
			return parts[0].Text
		}
		return parts
	default:
		if b, err := common.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

func responsesContentPartToChatContent(item map[string]any, role string) dto.MediaContent {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	switch itemType {
	case "input_text", "output_text", "text":
		return dto.MediaContent{Type: dto.ContentTypeText, Text: common.Interface2String(item["text"])}
	case "input_image", "image":
		imageURL := item["image_url"]
		if imageURL == nil {
			imageURL = item["url"]
		}
		return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: imageURL}
	case "input_file", "file":
		file := item["file"]
		if file == nil {
			file = item
		}
		return dto.MediaContent{Type: dto.ContentTypeFile, File: file}
	default:
		text := common.Interface2String(item["text"])
		if text == "" {
			if b, err := common.Marshal(item); err == nil {
				text = string(b)
			}
		}
		return dto.MediaContent{Type: dto.ContentTypeText, Text: text}
	}
}

func responsesToolsToChatTools(raw json.RawMessage) []dto.ToolCallRequest {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType == "" {
			toolType = "function"
		}
		if toolType != "function" {
			continue
		}
		out = append(out, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        common.Interface2String(tool["name"]),
				Description: common.Interface2String(tool["description"]),
				Parameters:  tool["parameters"],
			},
		})
	}
	return out
}

func responsesToolChoiceToChat(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	if common.GetJsonType(raw) == "string" {
		return rawText(raw)
	}
	var choice map[string]any
	if err := common.Unmarshal(raw, &choice); err != nil {
		return raw
	}
	if strings.TrimSpace(common.Interface2String(choice["type"])) == "function" {
		if name := strings.TrimSpace(common.Interface2String(choice["name"])); name != "" {
			return map[string]any{"type": "function", "function": map[string]any{"name": name}}
		}
	}
	return choice
}

func responsesTextToChatResponseFormat(raw json.RawMessage) *dto.ResponseFormat {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	format, ok := payload["format"].(map[string]any)
	if !ok {
		return nil
	}
	formatType := strings.TrimSpace(common.Interface2String(format["type"]))
	if formatType == "" {
		return nil
	}
	out := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		out.JsonSchema, _ = common.Marshal(format)
	}
	return out
}

func reasoningToRaw(reasoning *dto.Reasoning) json.RawMessage {
	if reasoning == nil {
		return nil
	}
	raw, _ := common.Marshal(reasoning)
	return raw
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if common.GetJsonType(raw) == "string" {
		var s string
		_ = common.Unmarshal(raw, &s)
		return s
	}
	return string(raw)
}

func rawBool(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 || common.GetJsonType(raw) != "bool" {
		return false, false
	}
	var b bool
	if err := common.Unmarshal(raw, &b); err != nil {
		return false, false
	}
	return b, true
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	text := ExtractOutputTextFromResponses(resp)

	usage := &dto.Usage{}
	if resp.Usage != nil {
		if resp.Usage.InputTokens != 0 {
			usage.PromptTokens = resp.Usage.InputTokens
			usage.InputTokens = resp.Usage.InputTokens
		}
		if resp.Usage.OutputTokens != 0 {
			usage.CompletionTokens = resp.Usage.OutputTokens
			usage.OutputTokens = resp.Usage.OutputTokens
		}
		if resp.Usage.TotalTokens != 0 {
			usage.TotalTokens = resp.Usage.TotalTokens
		} else {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = resp.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.ImageTokens = resp.Usage.InputTokensDetails.ImageTokens
			usage.PromptTokensDetails.AudioTokens = resp.Usage.InputTokensDetails.AudioTokens
		}
		if resp.Usage.CompletionTokenDetails.ReasoningTokens != 0 {
			usage.CompletionTokenDetails.ReasoningTokens = resp.Usage.CompletionTokenDetails.ReasoningTokens
		}
	}

	created := resp.CreatedAt

	var toolCalls []dto.ToolCallResponse
	if text == "" && len(resp.Output) > 0 {
		for _, out := range resp.Output {
			if out.Type != "function_call" {
				continue
			}
			name := strings.TrimSpace(out.Name)
			if name == "" {
				continue
			}
			callId := strings.TrimSpace(out.CallId)
			if callId == "" {
				callId = strings.TrimSpace(out.ID)
			}
			toolCalls = append(toolCalls, dto.ToolCallResponse{
				ID:   callId,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      name,
					Arguments: out.ArgumentsString(),
				},
			})
		}
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	msg := dto.Message{
		Role:    "assistant",
		Content: text,
	}
	if len(toolCalls) > 0 {
		msg.SetToolCalls(toolCalls)
		msg.Content = ""
	}

	out := &dto.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   resp.Model,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			},
		},
		Usage: *usage,
	}

	return out, usage, nil
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Output) == 0 {
		return ""
	}

	var sb strings.Builder

	// Prefer assistant message outputs.
	for _, out := range resp.Output {
		if out.Type != "message" {
			continue
		}
		if out.Role != "" && out.Role != "assistant" {
			continue
		}
		for _, c := range out.Content {
			if c.Type == "output_text" && c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	for _, out := range resp.Output {
		for _, c := range out.Content {
			if c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	return sb.String()
}
