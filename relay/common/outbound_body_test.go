package common

import (
	"io"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestNewOutboundJSONBodyProvidesIndependentReplayReaders(t *testing.T) {
	original := rootcommon.GetDiskCacheConfig()
	rootcommon.SetDiskCacheConfig(rootcommon.DiskCacheConfig{Enabled: false})
	t.Cleanup(func() { rootcommon.SetDiskCacheConfig(original) })

	payload := []byte(`{"model":"test","input":"hello"}`)
	body, size, getBody, closer, err := NewOutboundJSONBody(payload)
	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), size)
	t.Cleanup(func() { _ = closer.Close() })

	prefix := make([]byte, 7)
	_, err = io.ReadFull(body, prefix)
	require.NoError(t, err)

	for range 2 {
		replay, err := getBody()
		require.NoError(t, err)
		replayed, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		require.Equal(t, payload, replayed)
	}

	remainder, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, payload[7:], remainder)
}
