package gemini

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func geminiStructuralFinishReason(reason string) bool {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "FINISH_REASON_UNSPECIFIED",
		"MALFORMED_FUNCTION_CALL",
		"UNEXPECTED_TOOL_CALL",
		"TOO_MANY_TOOL_CALLS",
		"MISSING_THOUGHT_SIGNATURE",
		"MALFORMED_RESPONSE",
		"NO_IMAGE",
		"IMAGE_OTHER":
		return true
	default:
		return false
	}
}

func geminiCompatibilityFinishReasonError(response *dto.GeminiChatResponse) *types.NewAPIError {
	if response == nil {
		return nil
	}
	for _, candidate := range response.Candidates {
		if candidate.FinishReason == nil || !geminiStructuralFinishReason(*candidate.FinishReason) {
			continue
		}
		return types.NewOpenAIError(
			fmt.Errorf("Gemini generation failed for candidate %d: %s", candidate.Index, strings.TrimSpace(*candidate.FinishReason)),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	return nil
}
