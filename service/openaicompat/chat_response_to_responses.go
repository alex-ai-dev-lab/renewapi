package openaicompat

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func ChatResponseToResponses(resp *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("chat response is nil")
	}
	responseID := strings.TrimSpace(resp.Id)
	if responseID == "" {
		responseID = "resp_unknown"
	} else if !strings.HasPrefix(responseID, "resp_") {
		responseID = "resp_" + strings.TrimPrefix(responseID, "chatcmpl-")
	}
	createdAt := intFromAny(resp.Created)
	status := "completed"
	finishReason := ""
	if len(resp.Choices) > 0 {
		finishReason = resp.Choices[0].FinishReason
	}
	incompleteReason := ""
	if finishReason == constant.FinishReasonLength {
		status = "incomplete"
		incompleteReason = "max_output_tokens"
	} else if finishReason == constant.FinishReasonContentFilter {
		status = "incomplete"
		incompleteReason = "content_filter"
	}
	statusRaw, _ := common.Marshal(status)

	output := make([]dto.ResponsesOutput, 0)
	for choiceIndex, choice := range resp.Choices {
		text := choice.Message.StringContent()
		if text != "" {
			output = append(output, dto.ResponsesOutput{
				Type:   "message",
				ID:     fmt.Sprintf("msg_%s_%d", strings.TrimPrefix(responseID, "resp_"), choiceIndex),
				Status: status,
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{{
					Type: "output_text",
					Text: text,
				}},
			})
		}
		for toolIndex, tool := range choice.Message.ParseToolCalls() {
			callID := strings.TrimSpace(tool.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%d_%d", choiceIndex, toolIndex)
			}
			arguments, err := common.Marshal(tool.Function.Arguments)
			if err != nil {
				return nil, err
			}
			output = append(output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        fmt.Sprintf("fc_%s", callID),
				Status:    status,
				CallId:    callID,
				Name:      tool.Function.Name,
				Arguments: arguments,
			})
		}
	}

	usage := responseUsageToResponses(resp.Usage)
	result := &dto.OpenAIResponsesResponse{
		ID:        responseID,
		Object:    "response",
		CreatedAt: createdAt,
		Status:    statusRaw,
		Model:     resp.Model,
		Output:    output,
		Usage:     usage,
	}
	if status == "incomplete" {
		result.IncompleteDetails = &dto.IncompleteDetails{Reasoning: incompleteReason}
	}
	return result, nil
}

func responseUsageToResponses(usage dto.Usage) *dto.Usage {
	converted := usage
	if converted.InputTokens == 0 {
		converted.InputTokens = converted.PromptTokens
	}
	if converted.OutputTokens == 0 {
		converted.OutputTokens = converted.CompletionTokens
	}
	if converted.TotalTokens == 0 {
		converted.TotalTokens = converted.InputTokens + converted.OutputTokens
	}
	if converted.InputTokensDetails == nil {
		details := converted.PromptTokensDetails
		converted.InputTokensDetails = &details
	}
	return &converted
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}
