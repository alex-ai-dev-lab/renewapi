package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestRejectsNonStringToolSchemaType(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name: "lookup",
				Parameters: map[string]any{
					"type":       123,
					"properties": map[string]any{},
				},
			},
		}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.Nil(t, converted)
	require.EqualError(t, err, `tool "lookup" parameters.type must be a string`)
}

func TestConvertOpenAIRequestRejectsNonStringStopEntry(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Stop: []any{"valid", float64(123)},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, req)
	require.Nil(t, converted)
	require.EqualError(t, err, "stop[1] must be a string")
}
