package openai

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResponsesStreamFailureErrorPreservesStructuredError(t *testing.T) {
	response := &dto.ResponsesStreamResponse{
		Type: "response.failed",
		Response: &dto.OpenAIResponsesResponse{Error: map[string]interface{}{
			"type": "invalid_request_error", "code": "context_length_exceeded", "message": "context too long",
		}},
	}
	err := responsesStreamFailureError(response, errors.New("fallback"))
	require.Equal(t, types.ErrorCode("context_length_exceeded"), err.GetErrorCode())
	require.Equal(t, "context too long", err.Error())
	require.Equal(t, "invalid_request_error", err.ToOpenAIError().Type)
}

func TestResponsesStreamFailureErrorFallsBackWhenUnstructured(t *testing.T) {
	err := responsesStreamFailureError(&dto.ResponsesStreamResponse{Type: "response.failed"}, errors.New("fallback"))
	require.Equal(t, types.ErrorCodeBadResponse, err.GetErrorCode())
}

func TestAggregateCompactionStreamPreservesStructuredFailure(t *testing.T) {
	body := `data: {"type":"response.failed","response":{"status":"failed","error":{"type":"server_error","code":"overloaded","message":"try later"}}}` + "\n\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	_, _, err := aggregateCompactionResponsesStreamRaw(resp)
	require.NotNil(t, err)
	require.Equal(t, types.ErrorCode("overloaded"), err.GetErrorCode())
	require.Equal(t, "server_error", err.ToOpenAIError().Type)
}
