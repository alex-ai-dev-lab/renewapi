package claude

import "github.com/QuantumNous/new-api/dto"

func prepareOpenAIRequestForClaude(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	prepared := prepareOpenAIRequestForClaudeOpus47(request)
	if prepared == nil {
		return nil
	}

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
