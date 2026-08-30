package claude

import (
	"fmt"

	"github.com/QuantumNous/new-api/dto"
)

func validateOpenAIRequestForClaude(request *dto.GeneralOpenAIRequest) error {
	if request == nil {
		return fmt.Errorf("request is nil")
	}

	for _, tool := range request.Tools {
		params, ok := tool.Function.Parameters.(map[string]any)
		if !ok || params == nil {
			continue
		}
		if schemaType, exists := params["type"]; exists && schemaType != nil {
			if _, ok := schemaType.(string); !ok {
				return fmt.Errorf("tool %q parameters.type must be a string", tool.Function.Name)
			}
		}
	}

	if stops, ok := request.Stop.([]any); ok {
		for i, stop := range stops {
			if _, ok := stop.(string); !ok {
				return fmt.Errorf("stop[%d] must be a string", i)
			}
		}
	}
	return nil
}
