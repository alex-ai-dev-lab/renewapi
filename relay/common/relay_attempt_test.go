package common

import (
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayAttemptSnapshotRestoresSevenAttempts(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAIResponses,
		RelayMode:               relayconstant.RelayModeResponses,
		RequestURLPath:          "/v1/responses",
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAIResponses},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}
	for attempt := 0; attempt < 7; attempt++ {
		snapshot := info.SnapshotAttempt()
		info.RelayMode = relayconstant.RelayModeChatCompletions
		info.RequestURLPath = "/v1/chat/completions"
		info.RequestConversionChain = append(info.RequestConversionChain, types.RelayFormatOpenAI)
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
		info.RestoreAttempt(snapshot)
		require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.RelayFormat)
		require.Equal(t, relayconstant.RelayModeResponses, info.RelayMode)
		require.Equal(t, "/v1/responses", info.RequestURLPath)
		require.Equal(t, []types.RelayFormat{types.RelayFormatOpenAIResponses}, info.RequestConversionChain)
		require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)
	}
}
