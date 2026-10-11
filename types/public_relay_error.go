package types

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

const PublicModelCapacityMessage = "Selected model is at capacity. Please try a different model."
const ErrorCodeModelCapacity ErrorCode = "model_capacity"

// Only specific structured model codes are classified. Generic HTTP 404,
// billing failures and free-form provider messages remain private.
func PublicUpstreamModelError(err *NewAPIError) *NewAPIError {
	if err == nil || err.GetErrorType() == ErrorTypeNewAPIError {
		return nil
	}
	return publicUpstreamModelCode(string(err.GetErrorCode()))
}

func publicUpstreamModelCode(code string) *NewAPIError {
	switch code {
	case "model_not_found", "unknown_model", "invalid_model":
		return WithOpenAIError(OpenAIError{Message: "Requested model does not exist or is not accessible. Check the model name and access permissions.", Type: "invalid_request_error", Code: "model_not_found"}, http.StatusNotFound)
	case "model_unavailable", "model_not_available":
		return WithOpenAIError(OpenAIError{Message: "Selected model is currently unavailable. Please try another model or retry later.", Type: "server_error", Code: "model_unavailable"}, http.StatusServiceUnavailable)
	}
	return nil
}

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
	case ErrorCodeModelNotFound, ErrorCodeModelUnavailable, ErrorCodeResponsesCompactionNoEligibleChannel:
		return true
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
		// Route diagnostics may contain internal group/channel details. Expose
		// the actionable category without echoing those details.
		switch err.GetErrorCode() {
		case ErrorCodeModelNotFound:
			if err.StatusCode >= 500 {
				return NewErrorWithStatusCode(fmt.Errorf("Selected model is unavailable. No usable channel is currently available."), ErrorCodeModelUnavailable, http.StatusServiceUnavailable, ErrOptionWithSkipRetry())
			}
		case ErrorCodeModelUnavailable:
			copy.SetMessage("Selected model is unavailable. No usable channel is currently available.")
		case ErrorCodeResponsesCompactionNoEligibleChannel:
			copy.SetMessage("No available channel supports this model's requested Responses compaction operation.")
		}
		return &copy
	}
	if modelError := PublicUpstreamModelError(err); modelError != nil {
		return modelError
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
	status := http.StatusServiceUnavailable
	for _, path := range []string{"error.code", "response.error.code", "response.status_details.error.code"} {
		if modelError := publicUpstreamModelCode(value.Get(path).String()); modelError != nil {
			public = modelError.ToOpenAIError()
			status = modelError.StatusCode
			break
		}
	}
	event := map[string]any{"type": kind}
	for _, key := range []string{"stream_id", "event_id", "response_id"} {
		if field := value.Get(key); field.Type == gjson.String {
			event[key] = field.String()
		}
	}
	response := value.Get("response")
	if kind == "error" || kind == "response.error" || kind == "" || value.Get("error").Exists() || !response.IsObject() {
		event["type"] = "error"
		event["error"], event["status"] = public, status
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
