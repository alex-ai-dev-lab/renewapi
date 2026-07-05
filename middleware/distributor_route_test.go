package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestValidateResolvedChannelRouteRejectsCodexThroughOtherAdaptor(t *testing.T) {
	err := validateResolvedChannelRoute(
		&model.Channel{Id: 151, Type: constant.ChannelTypeCodex},
		constant.ChannelTypeAnthropic,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "codex channel #151")
}

func TestValidateResolvedChannelRouteAllowsNonCodexProtocolOverride(t *testing.T) {
	err := validateResolvedChannelRoute(
		&model.Channel{Id: 71, Type: constant.ChannelTypeOpenAI},
		constant.ChannelTypeAnthropic,
	)

	require.NoError(t, err)
}
