package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestNotImplementedIsModelScopedChannelFailure(t *testing.T) {
	oldAutomaticDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldAutomaticDisable
	})

	err := types.NewErrorWithStatusCode(
		errors.New("not implemented"),
		types.ErrorCodeConvertRequestFailed,
		http.StatusInternalServerError,
	)

	if !IsChannelFailureError(err) {
		t.Fatal("not implemented 500 should be treated as a channel failure")
	}
	if !IsModelScopedChannelFailureError(err) {
		t.Fatal("not implemented 500 should be treated as a model-scoped channel failure")
	}
	if ShouldDisableChannel(err) {
		t.Fatal("not implemented 500 should disable the channel-model, not the whole channel")
	}
}

func TestInvalidEncryptedHistoryNeverCountsForChannelDisable(t *testing.T) {
	oldDisable := common.AutomaticDisableChannelEnabled
	oldMonitor := *operation_setting.GetMonitorSetting()
	oldCodes := operation_setting.AutomaticDisableStatusCodesToString()
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldDisable
		*operation_setting.GetMonitorSetting() = oldMonitor
		require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString(oldCodes))
	})
	common.AutomaticDisableChannelEnabled = true
	require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString("100-599"))
	monitor := operation_setting.GetMonitorSetting()
	monitor.CountAllRelayErrorsForDisable = true
	monitor.CountSkipRetryErrorsForDisable = true
	monitor.CountModelScopedErrorsForDisable = true
	for _, status := range []int{400, 500, 502} {
		err := types.NewOpenAIError(errors.New("The encrypted content for item rs_private could not be verified. Reason: Encrypted content item_id did not match the target item id."), types.ErrorCodeBadResponseBody, status)
		require.False(t, ShouldDisableChannel(err))
		require.False(t, ShouldCountChannelFailureForDisable(err))
		require.False(t, IsModelScopedChannelFailureError(err))
		require.False(t, IsImmediateChannelDisableError(err))
		require.False(t, IsChannelFailureError(err))
		require.False(t, ShouldRouterCooldownTrackError(err))
	}
	// Broad policy still applies to actual upstream failures.
	require.True(t, ShouldCountChannelFailureForDisable(types.NewOpenAIError(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, 503)))
}

func TestTLSVerificationErrorIsNotChannelFailure(t *testing.T) {
	err := types.NewError(
		errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"),
		types.ErrorCodeDoRequestFailed,
	)

	if IsChannelFailureError(err) {
		t.Fatal("TLS verification errors should not be treated as channel failures")
	}
}

func TestUnsupportedUserParameterIsNotModelScopedChannelFailure(t *testing.T) {
	err := types.NewErrorWithStatusCode(
		errors.New("unsupported parameter: temperature"),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
	)

	if IsModelScopedChannelFailureError(err) {
		t.Fatal("4xx user request errors should not disable a channel-model")
	}
}

func TestModelNotFound404IsModelScopedChannelFailure(t *testing.T) {
	err := types.NewErrorWithStatusCode(
		errors.New("model not found: claude-opus-4-6"),
		types.ErrorCodeInvalidRequest,
		http.StatusNotFound,
	)

	if !IsModelScopedChannelFailureError(err) {
		t.Fatal("404 model not found should disable the channel-model")
	}
}

func TestNotImplemented501IsModelScopedChannelFailure(t *testing.T) {
	err := types.NewErrorWithStatusCode(
		errors.New("not implemented"),
		types.ErrorCodeChannelModelMappedError,
		http.StatusNotImplemented,
	)

	if !IsModelScopedChannelFailureError(err) {
		t.Fatal("501 not implemented should disable the channel-model")
	}
}
