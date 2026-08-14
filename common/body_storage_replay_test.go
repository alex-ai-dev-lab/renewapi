package common

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenBodyStorageReaderDoesNotMoveSharedCursor(t *testing.T) {
	original := GetDiskCacheConfig()
	SetDiskCacheConfig(DiskCacheConfig{Enabled: false})
	t.Cleanup(func() { SetDiskCacheConfig(original) })

	payload := []byte("0123456789")
	storage, err := CreateBodyStorage(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	_, err = storage.Seek(4, io.SeekStart)
	require.NoError(t, err)

	replay, err := OpenBodyStorageReader(storage)
	require.NoError(t, err)
	replayed, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.NoError(t, replay.Close())
	require.Equal(t, payload, replayed)

	shared := make([]byte, 2)
	_, err = io.ReadFull(storage, shared)
	require.NoError(t, err)
	require.Equal(t, []byte("45"), shared)
}

func TestOpenBodyStorageReaderUsesIndependentDiskFile(t *testing.T) {
	original := GetDiskCacheConfig()
	SetDiskCacheConfig(DiskCacheConfig{Enabled: true, ThresholdMB: 0, MaxSizeMB: 1, Path: t.TempDir()})
	t.Cleanup(func() { SetDiskCacheConfig(original) })

	payload := []byte("disk-backed-payload")
	storage, err := CreateBodyStorage(payload)
	require.NoError(t, err)
	require.True(t, storage.IsDisk())
	t.Cleanup(func() { _ = storage.Close() })
	_, err = storage.Seek(5, io.SeekStart)
	require.NoError(t, err)

	for range 2 {
		replay, err := OpenBodyStorageReader(storage)
		require.NoError(t, err)
		_, isFile := replay.(*os.File)
		require.True(t, isFile)
		replayed, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		require.Equal(t, payload, replayed)
	}

	shared := make([]byte, 2)
	_, err = io.ReadFull(storage, shared)
	require.NoError(t, err)
	require.Equal(t, []byte("ba"), shared)
}

func TestOpenBodyStorageReaderRejectsClosedStorage(t *testing.T) {
	storage := newMemoryStorage([]byte("closed"))
	require.NoError(t, storage.Close())
	_, err := OpenBodyStorageReader(storage)
	require.True(t, errors.Is(err, ErrStorageClosed))
}
