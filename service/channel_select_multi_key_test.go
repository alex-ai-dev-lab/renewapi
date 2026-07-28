package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSelectUntriedEnabledMultiKeyOnlyOverridesRetries(t *testing.T) {
	channel := &model.Channel{Id: 42, Key: "key-zero\nkey-one"}
	channel.ChannelInfo.IsMultiKey = true
	param := &RetryParam{}

	_, _, found, err := SelectUntriedEnabledMultiKey(param, channel)
	require.Nil(t, err)
	require.False(t, found, "the first attempt must preserve the key selected by channel affinity or polling")

	RecordTriedMultiKeyIndex(param, channel.Id, 0)
	key, index, found, err := SelectUntriedEnabledMultiKey(param, channel)
	require.Nil(t, err)
	require.True(t, found)
	require.Equal(t, 1, index)
	require.Equal(t, "key-one", key)
}
