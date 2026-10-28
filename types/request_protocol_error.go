package types

import "net/http"

const ErrorCodeRequestProtocolUnsupported ErrorCode = "request_protocol_unsupported"

const PublicRequestProtocolUnsupportedMessage = "The request uses features that cannot be represented by the available model protocol. Use a compatible model or disable unsupported client features."

// NewRequestProtocolUnsupportedError describes a local, route-specific failure.
// Another channel can support the original request without changing its meaning.
func NewRequestProtocolUnsupportedError(err error) *NewAPIError {
	return NewErrorWithStatusCode(err, ErrorCodeRequestProtocolUnsupported, http.StatusBadRequest)
}

func IsRequestProtocolUnsupportedError(err *NewAPIError) bool {
	return err != nil && err.GetErrorType() == ErrorTypeNewAPIError && err.GetErrorCode() == ErrorCodeRequestProtocolUnsupported
}
