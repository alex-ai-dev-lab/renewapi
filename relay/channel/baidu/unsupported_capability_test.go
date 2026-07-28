package baidu

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/stretchr/testify/require"
)

func TestConvertClaudeRequestReturnsUnsupportedCapability(t *testing.T) {
	_, err := (&Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{})
	require.Error(t, err)
	require.True(t, errors.Is(err, channel.ErrUnsupportedCapability))
}
