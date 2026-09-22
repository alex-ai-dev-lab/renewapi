package types

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPublicRelayErrorPreservesInternalErrorAndLocalRejections(t *testing.T) {
	internal := WithOpenAIError(OpenAIError{Message: "private.example: invalid API key", Type: "provider_plugin", Code: "invalid_request"}, http.StatusUnauthorized)
	public := PublicRelayError(internal)
	require.Equal(t, PublicModelCapacityMessage, public.Error())
	require.Equal(t, ErrorCodeModelCapacity, public.GetErrorCode())
	require.Equal(t, "server_error", public.ToOpenAIError().Type)
	require.Equal(t, "api_error", public.ToClaudeError().Type)
	require.Contains(t, internal.Error(), "private.example")
	local := NewErrorWithStatusCode(errors.New("missing model"), ErrorCodeInvalidRequest, http.StatusBadRequest)
	require.Equal(t, local.Error(), PublicRelayError(local).Error())
}

func TestPublicRelayEventPreservesErrorCorrelation(t *testing.T) {
	data, err := PublicRelayEvent([]byte(`{"type":"error","stream_id":"s1","event_id":"e1","error":{"message":"private.example","metadata":{"key":"secret"}},"response":{"id":"r1","status_details":{"error":{"message":"provider-key"}}}}`))
	require.NoError(t, err)
	require.NotContains(t, string(data), "private.example")
	require.NotContains(t, string(data), "secret")
	require.NotContains(t, string(data), "provider-key")
	require.Equal(t, "s1", gjson.GetBytes(data, "stream_id").String())
	require.Equal(t, "e1", gjson.GetBytes(data, "event_id").String())
	require.Equal(t, "r1", gjson.GetBytes(data, "response.id").String())
}
