package claude

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func invalidClaudeOpenAIRequest(format string, args ...any) error {
	return types.NewErrorWithStatusCode(
		fmt.Errorf(format, args...),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func validateOpenAIRequestForClaude(request *dto.GeneralOpenAIRequest) error {
	if request == nil {
		return invalidClaudeOpenAIRequest("request is nil")
	}

	for _, tool := range request.Tools {
		params, ok := tool.Function.Parameters.(map[string]any)
		if !ok || params == nil {
			continue
		}
		if schemaType, exists := params["type"]; exists && schemaType != nil {
			if _, ok := schemaType.(string); !ok {
				return invalidClaudeOpenAIRequest("tool %q parameters.type must be a string", tool.Function.Name)
			}
		}
	}

	if stops, ok := request.Stop.([]any); ok {
		for i, stop := range stops {
			if _, ok := stop.(string); !ok {
				return invalidClaudeOpenAIRequest("stop[%d] must be a string", i)
			}
		}
	}
	return nil
}
