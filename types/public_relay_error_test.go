package types

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPublicRelayEventDiscardsUpstreamDebugFields(t *testing.T) {
	for _, data := range []string{
		`{"type":"error","event_id":"event_1","stream_id":"main","response_id":"resp_1","error":{"message":"private-upstream","code":"private_plugin"},"debug":{"hostname":"private-upstream","api_key":"private_key"},"message":"private detail"}`,
		`{"type":"response.done","event_id":"event_1","stream_id":"main","response":{"id":"resp_1","object":"realtime.response","status":"failed","status_details":{"type":"failed","reason":"private_detail","error":{"message":"private-upstream"}},"metadata":{"internal":"private_plugin"}}}`,
	} {
		public, err := PublicRelayEvent([]byte(data))
		require.NoError(t, err)
		require.NotContains(t, string(public), "private")
		require.Contains(t, string(public), PublicModelCapacityMessage)
		require.Contains(t, string(public), `"event_id":"event_1"`)
		require.Contains(t, string(public), `"stream_id":"main"`)
		require.Contains(t, string(public), "resp_1")
	}
}

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

func TestPublicRelayEventWithoutResponseStillHasPublicError(t *testing.T) {
	for _, kind := range []string{"response.failed", "response.error"} {
		data, err := PublicRelayEvent([]byte(`{"type":"` + kind + `","response_id":"resp_1","stream_id":"main","event_id":"event_1","message":"private upstream failure"}`))
		require.NoError(t, err)
		require.Equal(t, "error", gjson.GetBytes(data, "type").String())
		require.Equal(t, PublicModelCapacityMessage, gjson.GetBytes(data, "error.message").String())
		require.Equal(t, "resp_1", gjson.GetBytes(data, "response_id").String())
		require.NotContains(t, string(data), "private")
	}
}
