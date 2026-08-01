package system_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPasskeySettingsSnapshotDerivesOriginWithoutMutation(t *testing.T) {
	previousSettings := defaultPasskeySettings
	previousServerAddress := ServerAddress
	t.Cleanup(func() {
		defaultPasskeySettings = previousSettings
		ServerAddress = previousServerAddress
	})

	defaultPasskeySettings = PasskeySettings{}
	ServerAddress = "https://Example.com:8443/console/"

	snapshot := GetPasskeySettingsSnapshot()
	require.Equal(t, "example.com", snapshot.RPID)
	require.Equal(t, "https://Example.com:8443", snapshot.Origins)
	require.Empty(t, defaultPasskeySettings.RPID)
	require.Empty(t, defaultPasskeySettings.Origins)
}
