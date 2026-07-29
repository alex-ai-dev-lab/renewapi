package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestLegacyDisabledOnlyControlsPreCommitRetry(t *testing.T) {
	setting := StreamRecoverySetting{
		SessionRouteRepairEnabled: true, PostCommitRouteRepairEnabled: true,
		MaxCrossRequestRouteChanges: 2,
	}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	require.NoError(t, manager.LoadFromDB(map[string]string{"stream_recovery_setting": `{"enabled":false}`}))
	require.False(t, setting.PreCommitRetryOn())
	require.True(t, setting.SessionRouteRepairEnabled)
	require.True(t, setting.PostCommitRouteRepairEnabled)
	require.Equal(t, 2, setting.MaxCrossRequestRouteChanges)
}

func TestLegacyEnabledEnablesPreCommitRetryWithoutDisablingNewDefaults(t *testing.T) {
	setting := StreamRecoverySetting{SessionRouteRepairEnabled: true, PostCommitRouteRepairEnabled: true}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	require.NoError(t, manager.LoadFromDB(map[string]string{"stream_recovery_setting": `{"enabled":true}`}))
	require.True(t, setting.PreCommitRetryOn())
	require.True(t, setting.SessionRouteRepairEnabled)
}

func TestExplicitNewPreCommitFalseOverridesLegacyTrue(t *testing.T) {
	setting := StreamRecoverySetting{}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	require.NoError(t, manager.LoadFromDB(map[string]string{
		"stream_recovery_setting": `{"enabled":true,"pre_commit_retry_enabled":false}`,
	}))
	require.True(t, setting.Enabled)
	require.False(t, setting.PreCommitRetryOn())
}

func TestDotExpandedPreCommitFieldOverridesLegacyJSON(t *testing.T) {
	setting := StreamRecoverySetting{}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	require.NoError(t, manager.LoadFromDB(map[string]string{
		"stream_recovery_setting":                          `{"enabled":true}`,
		"stream_recovery_setting.pre_commit_retry_enabled": "false",
	}))
	require.False(t, setting.PreCommitRetryOn())
}

func TestExplicitNewPreCommitTrue(t *testing.T) {
	setting := StreamRecoverySetting{Enabled: false}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	require.NoError(t, manager.LoadFromDB(map[string]string{
		"stream_recovery_setting.pre_commit_retry_enabled": "true",
	}))
	require.True(t, setting.PreCommitRetryOn())
}

func TestStreamRecoveryConfigSavePreservesNewFields(t *testing.T) {
	setting := StreamRecoverySetting{
		Enabled: false, PreCommitRetryEnabled: false,
		SessionRouteRepairEnabled: true, PostCommitRouteRepairEnabled: true,
		MaxCrossRequestRouteChanges: 2,
	}
	manager := config.NewConfigManager()
	manager.Register("stream_recovery_setting", &setting)
	saved := map[string]string{}
	require.NoError(t, manager.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	require.Equal(t, "false", saved["stream_recovery_setting.pre_commit_retry_enabled"])
	require.Equal(t, "true", saved["stream_recovery_setting.session_route_repair_enabled"])
	require.Equal(t, "true", saved["stream_recovery_setting.post_commit_route_repair_enabled"])
	require.Equal(t, "2", saved["stream_recovery_setting.max_cross_request_route_changes"])
}
