package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/types"
)

// IsRelayFailoverError 复用现有失败分类，并区分本地拒绝与上游渠道故障。
func IsRelayFailoverError(err *types.NewAPIError) bool {
	if err == nil || errors.Is(err, context.Canceled) || err.StatusCode == 499 {
		return false
	}
	switch err.GetErrorCode() {
	case types.ErrorCodeInvalidRequest, types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeBadRequestBody, types.ErrorCodeAccessDenied,
		types.ErrorCodeInsufficientUserQuota, types.ErrorCodePreConsumeTokenQuotaFailed,
		types.ErrorCodeModelPriceError, types.ErrorCodeRateLimitExceeded,
		types.ErrorCodeRequestGuardBlocked, types.ErrorCodeRequestGuardUnavailable,
		types.ErrorCodeSensitiveWordsDetected:
		return false
	case types.ErrorCodeDoRequestFailed, types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeBadResponse, types.ErrorCodeBadResponseBody, types.ErrorCodeEmptyResponse,
		types.ErrorCodeTruncatedResponse, types.ErrorCodeChannelResponseTimeExceeded:
		return true
	}
	if IsChannelFailureError(err) || IsModelScopedChannelFailureError(err) || IsTLSVerificationError(err) {
		return true
	}
	switch err.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestTimeout,
		http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return err.StatusCode >= 500 && err.StatusCode <= 599
}
