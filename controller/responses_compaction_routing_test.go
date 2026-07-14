package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestResponsesRequirementForRelayKeepsCompactionClientModel(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeResponses,
		ResponsesRequestKind: dto.ResponsesCompactionTrigger,
		OriginModelName:      "gpt-5.6-sol",
		ClientModelName:      "gpt-5.5",
		IsStream:             true,
	}

	requirement := responsesRequirementForRelay(info)
	require.NotNil(t, requirement)
	require.Equal(t, dto.ResponsesCompactionTrigger, requirement.Kind)
	require.True(t, requirement.ClientStream)
	require.Equal(t, "gpt-5.5", requirement.RequiredContinuationModel)
}

func TestResponsesRequirementForRelayDoesNotConstrainNormalRequests(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeResponses,
		ResponsesRequestKind: dto.ResponsesNormal,
		OriginModelName:      "gpt-5.5",
		ClientModelName:      "gpt-5.5",
	}

	requirement := responsesRequirementForRelay(info)
	require.NotNil(t, requirement)
	require.Empty(t, requirement.RequiredContinuationModel)
}
