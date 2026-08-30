package types

import (
	"net/http"
	"testing"
)

func TestSensitiveWordsDetectedDefaultsToClientError(t *testing.T) {
	err := NewError(nil, ErrorCodeSensitiveWordsDetected)

	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadRequest)
	}
	if !IsSkipRetryError(err) {
		t.Fatal("sensitive words rejection must not be retryable")
	}
	if err.GetErrorCode() != ErrorCodeSensitiveWordsDetected {
		t.Fatalf("error code = %q, want %q", err.GetErrorCode(), ErrorCodeSensitiveWordsDetected)
	}
	if err.Error() != string(ErrorCodeSensitiveWordsDetected) {
		t.Fatalf("message = %q, want %q", err.Error(), ErrorCodeSensitiveWordsDetected)
	}
}
