package claude

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func prepareOpenAIRequestForClaude(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	prepared := prepareOpenAIReasoningEffortForClaude(request)
	prepared = prepareOpenAIRequestForClaudeOpus47(prepared)
	if prepared == nil {
		return nil
	}
	prepared = prepareOpenAIToolsForClaude(prepared)

	hasBlankRole := false
	for _, message := range prepared.Messages {
		if message.Role == "" {
			hasBlankRole = true
			break
		}
	}
	if !hasBlankRole {
		return prepared
	}

	copyRequest := *prepared
	copyRequest.Messages = append([]dto.Message(nil), prepared.Messages...)
	for i := range copyRequest.Messages {
		if copyRequest.Messages[i].Role == "" {
			copyRequest.Messages[i].Role = "user"
		}
	}
	return &copyRequest
}

func prepareOpenAIReasoningEffortForClaude(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	if request == nil || strings.TrimSpace(request.ReasoningEffort) != "" || len(request.Reasoning) == 0 {
		return request
	}

	var reasoning struct {
		Enabled *bool  `json:"enabled"`
		Effort  string `json:"effort"`
	}
	if err := common.Unmarshal(request.Reasoning, &reasoning); err != nil {
		return request
	}
	if reasoning.Enabled != nil && !*reasoning.Enabled {
		return request
	}
	effort := strings.ToLower(strings.TrimSpace(reasoning.Effort))
	switch effort {
	case "low", "medium", "high":
		prepared := *request
		prepared.ReasoningEffort = effort
		return &prepared
	default:
		return request
	}
}

func prepareOpenAIToolsForClaude(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	if request == nil || len(request.Tools) == 0 {
		return request
	}

	needsCopy := false
	for _, tool := range request.Tools {
		params := tool.Function.Parameters
		if params == nil {
			needsCopy = true
			break
		}
		if schema, ok := params.(map[string]any); ok {
			if schema["properties"] == nil || schema["required"] == nil {
				needsCopy = true
				break
			}
		}
	}
	if !needsCopy {
		return request
	}

	prepared := *request
	prepared.Tools = append([]dto.ToolCallRequest(nil), request.Tools...)
	for i := range prepared.Tools {
		params := prepared.Tools[i].Function.Parameters
		if params == nil {
			prepared.Tools[i].Function.Parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			}
			continue
		}
		schema, ok := params.(map[string]any)
		if !ok || (schema["properties"] != nil && schema["required"] != nil) {
			continue
		}
		copySchema := make(map[string]any, len(schema)+2)
		for key, value := range schema {
			copySchema[key] = value
		}
		if copySchema["properties"] == nil {
			copySchema["properties"] = map[string]any{}
		}
		if copySchema["required"] == nil {
			copySchema["required"] = []string{}
		}
		prepared.Tools[i].Function.Parameters = copySchema
	}
	return &prepared
}
