package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRequestProtocolMismatchRetriesWithoutPenalizingChannel(t *testing.T) {
	oldDisable := common.AutomaticDisableChannelEnabled
	oldMonitor := *operation_setting.GetMonitorSetting()
	oldCodes := operation_setting.AutomaticDisableStatusCodesToString()
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldDisable
		*operation_setting.GetMonitorSetting() = oldMonitor
		require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString(oldCodes))
	})
	common.AutomaticDisableChannelEnabled = true
	operation_setting.GetMonitorSetting().CountAllRelayErrorsForDisable = true
	operation_setting.GetMonitorSetting().CountSkipRetryErrorsForDisable = true
	operation_setting.GetMonitorSetting().CountModelScopedErrorsForDisable = true
	require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString("100-599"))
	err := types.NewRequestProtocolUnsupportedError(errors.New("request feature is not implemented in target protocol"))
	require.True(t, IsRelayFailoverError(err))
	require.False(t, ShouldDisableChannel(err))
	require.False(t, ShouldCountChannelFailureForDisable(err))
	require.False(t, IsImmediateChannelDisableError(err))
	require.False(t, IsModelScopedChannelFailureError(err))
	require.False(t, IsChannelFailureError(err))
	require.False(t, ShouldRouterCooldownTrackError(err))
}
