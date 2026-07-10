package common

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSRFProtectionRejectsLiteralPrivateAndReservedIPs(t *testing.T) {
	protection := &SSRFProtection{AllowPrivateIp: false}
	for _, host := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "fc00::1", "::ffff:127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			require.Error(t, protection.ValidateNetworkTarget(host, 80))
		})
	}
}

func TestSSRFProtectionValidatesResolvedIPAndConfiguredPorts(t *testing.T) {
	protection, err := NewSSRFProtectionFromFetchSetting(false, false, false, nil, nil, []string{"80", "8000-8001"}, true)
	require.NoError(t, err)
	require.NoError(t, protection.ValidateNetworkTarget("example.com", 8001))
	require.Error(t, protection.ValidateNetworkTarget("example.com", 9000))
	require.Error(t, protection.ValidateResolvedIP("example.com", net.ParseIP("169.254.169.254")))
	require.NoError(t, protection.ValidateResolvedIP("example.com", net.ParseIP("8.8.8.8")))
}

func TestSSRFProtectionAllowsPrivateIPOnlyWhenExplicitlyEnabled(t *testing.T) {
	protection := &SSRFProtection{AllowPrivateIp: true}
	require.NoError(t, protection.ValidateNetworkTarget("10.0.0.1", 80))
}
