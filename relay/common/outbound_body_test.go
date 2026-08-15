package common

import (
	"bytes"
	"fmt"
	"io"
	"sync"
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

func TestNewOutboundJSONBodyGetBodyIsConcurrentSafe(t *testing.T) {
	original := rootcommon.GetDiskCacheConfig()
	rootcommon.SetDiskCacheConfig(rootcommon.DiskCacheConfig{Enabled: true, ThresholdMB: 0, MaxSizeMB: 4, Path: t.TempDir()})
	t.Cleanup(func() { rootcommon.SetDiskCacheConfig(original) })

	payload := bytes.Repeat([]byte(`{"model":"test","input":"concurrent"}`), 1024)
	_, _, getBody, closer, err := NewOutboundJSONBody(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })

	const readers = 32
	start := make(chan struct{})
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			<-start
			replay, replayErr := getBody()
			if replayErr != nil {
				errs <- replayErr
				return
			}
			data, readErr := io.ReadAll(replay)
			closeErr := replay.Close()
			if readErr != nil {
				errs <- readErr
				return
			}
			if closeErr != nil {
				errs <- closeErr
				return
			}
			if !bytes.Equal(payload, data) {
				errs <- fmt.Errorf("replayed payload mismatch: got %d bytes", len(data))
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for replayErr := range errs {
		require.NoError(t, replayErr)
	}
}
