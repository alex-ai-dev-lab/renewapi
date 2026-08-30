package claude

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/reasoning"
)

func prepareOpenAIRequestForClaudeOpus47(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	if request == nil || !strings.HasPrefix(request.Model, "claude-opus-4-7") {
		return request
	}

	switch request.ReasoningEffort {
	case "low", "medium", "high":
	default:
		return request
	}

	baseModel := request.Model
	if parsedBaseModel, _, ok := reasoning.TrimEffortSuffix(baseModel); ok {
		baseModel = parsedBaseModel
	}

	prepared := *request
	prepared.Model = baseModel + "-" + request.ReasoningEffort
	prepared.ReasoningEffort = ""
	return &prepared
}
