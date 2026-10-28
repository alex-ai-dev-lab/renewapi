package types

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestProtocolErrorPreservesAdminDiagnostic(t *testing.T) {
	err := NewRequestProtocolUnsupportedError(errors.New("tools.private_feature has no supported representation"))
	require.True(t, IsLocalRelayRejection(err))
	require.False(t, IsSkipRetryError(err))
	public := PublicRelayError(err)
	require.Equal(t, http.StatusBadRequest, public.StatusCode)
	require.Equal(t, ErrorCodeRequestProtocolUnsupported, public.GetErrorCode())
	require.Equal(t, PublicRequestProtocolUnsupportedMessage, public.Error())
	require.Contains(t, err.Error(), "private_feature")
	spoof := WithOpenAIError(OpenAIError{Code: string(ErrorCodeRequestProtocolUnsupported), Message: "private provider diagnostic"}, http.StatusBadRequest)
	require.False(t, IsRequestProtocolUnsupportedError(spoof))
	require.Equal(t, ErrorCodeModelCapacity, PublicRelayError(spoof).GetErrorCode())
}
