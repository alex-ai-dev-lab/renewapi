package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func setupRedisTestClient(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	oldRDB := RDB
	oldRedisEnabled := RedisEnabled
	oldTimeout := RedisOperationTimeout
	oldSyncFrequency := SyncFrequency
	RDB = client
	RedisEnabled = true
	RedisOperationTimeout = time.Second
	SyncFrequency = 60
	t.Cleanup(func() {
		_ = client.Close()
		RDB = oldRDB
		RedisEnabled = oldRedisEnabled
		RedisOperationTimeout = oldTimeout
		SyncFrequency = oldSyncFrequency
	})
	return server
}

func TestRedisContextOperationsAndScripts(t *testing.T) {
	server := setupRedisTestClient(t)

	require.NoError(t, RedisSetContext(context.Background(), "plain", "value", time.Minute))
	value, err := RedisGetContext(context.Background(), "plain")
	require.NoError(t, err)
	require.Equal(t, "value", value)

	count, err := RedisIncrWithTTLContext(context.Background(), "counter", 2, 60)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	count, err = RedisIncrWithTTLContext(context.Background(), "counter", 3, 60)
	require.NoError(t, err)
	require.EqualValues(t, 5, count)
	require.Equal(t, 60*time.Second, server.TTL("counter"))
}

func TestRedisHashScripts(t *testing.T) {
	server := setupRedisTestClient(t)

	require.NoError(t, RedisHIncrBy("hash:counter", "count", 2))
	require.Equal(t, "2", server.HGet("hash:counter", "count"))
	require.Equal(t, 60*time.Second, server.TTL("hash:counter"))

	require.NoError(t, RedisHIncrByExisting("hash:missing", "count", 1, "required"))
	require.False(t, server.Exists("hash:missing"))

	server.HSet("hash:existing", "required", "yes", "count", "3")
	require.NoError(t, RedisHIncrByExisting("hash:existing", "count", 4, "required"))
	require.Equal(t, "7", server.HGet("hash:existing", "count"))
	require.Equal(t, 60*time.Second, server.TTL("hash:existing"))
	require.NoError(t, RedisHIncrByExisting("hash:existing", "count", 4, "absent"))
	require.Equal(t, "7", server.HGet("hash:existing", "count"))

	require.NoError(t, RedisHSetField("hash:set", "value", "first"))
	require.Equal(t, "first", server.HGet("hash:set", "value"))
	require.Equal(t, 60*time.Second, server.TTL("hash:set"))

	require.NoError(t, RedisHSetFieldExisting("hash:set-missing", "value", "ignored", "required"))
	require.False(t, server.Exists("hash:set-missing"))

	server.HSet("hash:set-existing", "required", "yes", "value", "before")
	require.NoError(t, RedisHSetFieldExisting("hash:set-existing", "value", "after", "required"))
	require.Equal(t, "after", server.HGet("hash:set-existing", "value"))
	require.Equal(t, 60*time.Second, server.TTL("hash:set-existing"))
	require.NoError(t, RedisHSetFieldExisting("hash:set-existing", "value", "ignored", "absent"))
	require.Equal(t, "after", server.HGet("hash:set-existing", "value"))
}

func TestRedisContextHonorsCancellation(t *testing.T) {
	setupRedisTestClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RedisGetContext(ctx, "cancelled")
	require.True(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)
}
