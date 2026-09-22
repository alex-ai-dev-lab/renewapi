package types

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const PublicModelCapacityMessage = "Selected model is at capacity. Please try a different model."
const ErrorCodeModelCapacity ErrorCode = "model_capacity"

// IsLocalRelayRejection 只信任本地生成的拒绝类型，不能让上游同名 code 绕过脱敏。
func IsLocalRelayRejection(err *NewAPIError) bool {
	if err == nil || err.GetErrorType() != ErrorTypeNewAPIError {
		return false
	}
	switch err.GetErrorCode() {
	case ErrorCodeInvalidRequest, ErrorCodeReadRequestBodyFailed, ErrorCodeBadRequestBody,
		ErrorCodeAccessDenied, ErrorCodeInsufficientUserQuota, ErrorCodePreConsumeTokenQuotaFailed,
		ErrorCodeModelPriceError, ErrorCodeRateLimitExceeded, ErrorCodeConvertRequestFailed,
		ErrorCodeRequestGuardBlocked, ErrorCodeRequestGuardUnavailable,
		ErrorCodeSensitiveWordsDetected, ErrorCodeViolationFeeGrokCSAM:
		return true
	case ErrorCodeModelNotFound:
		return err.StatusCode < 500
	}
	return false
}

// PublicRelayError 是客户端错误的唯一脱敏入口，不修改管理员仍需读取的原始错误。
func PublicRelayError(err *NewAPIError) *NewAPIError {
	if err == nil {
		return nil
	}
	if IsLocalRelayRejection(err) {
		copy := *err
		return &copy
	}
	return WithOpenAIError(OpenAIError{
		Message: PublicModelCapacityMessage, Type: "server_error", Code: string(ErrorCodeModelCapacity),
	}, http.StatusServiceUnavailable, ErrOptionWithSkipRetry())
}

// PublicRelayEvent 保留 stream_id、event_id 等关联字段，只替换错误对象。
func PublicRelayEvent(data []byte) ([]byte, error) {
	for _, path := range []string{"error", "response.error", "response.status_details.error"} {
		value := gjson.GetBytes(data, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		public, err := common.Marshal(OpenAIError{Message: PublicModelCapacityMessage, Type: "server_error", Code: string(ErrorCodeModelCapacity)})
		if err != nil {
			return nil, err
		}
		data, err = sjson.SetRawBytes(data, path, public)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}
