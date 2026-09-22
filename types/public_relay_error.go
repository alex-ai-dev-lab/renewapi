package types

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
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

// PublicRelayEvent 只保留协议和关联字段，防止上游把真实错误放进额外的 debug/metadata。
func PublicRelayEvent(data []byte) ([]byte, error) {
	value := gjson.ParseBytes(data)
	kind := value.Get("type").String()
	failed := kind == "error" || kind == "response.failed" || kind == "response.error" || value.Get("response.status").String() == "failed"
	for _, path := range []string{"error", "response.error", "response.status_details.error"} {
		field := value.Get(path)
		failed = failed || field.Exists() && field.Type != gjson.Null
	}
	if !failed {
		return data, nil
	}
	public := OpenAIError{Message: PublicModelCapacityMessage, Type: "server_error", Code: string(ErrorCodeModelCapacity)}
	event := map[string]any{"type": kind}
	for _, key := range []string{"stream_id", "event_id", "response_id"} {
		if field := value.Get(key); field.Type == gjson.String {
			event[key] = field.String()
		}
	}
	response := value.Get("response")
	if kind == "error" || kind == "response.error" || kind == "" || value.Get("error").Exists() || !response.IsObject() {
		event["type"] = "error"
		event["error"], event["status"] = public, http.StatusServiceUnavailable
	}
	if response.IsObject() {
		sanitized := map[string]any{"id": response.Get("id").String(), "status": "failed"}
		if object := response.Get("object").String(); object == "response" || object == "realtime.response" {
			sanitized["object"] = object
		}
		if response.Get("status_details").Exists() {
			sanitized["status_details"] = map[string]any{"type": "failed", "error": public}
		} else {
			sanitized["error"] = public
		}
		event["response"] = sanitized
	}
	return common.Marshal(event)
}
