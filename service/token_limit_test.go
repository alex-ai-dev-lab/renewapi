package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

type tokenLimitRedisCommandCounter struct {
	mu    sync.Mutex
	count int
}

func (h *tokenLimitRedisCommandCounter) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	h.count++
	h.mu.Unlock()
	return ctx, nil
}

func (h *tokenLimitRedisCommandCounter) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *tokenLimitRedisCommandCounter) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	h.count++
	h.mu.Unlock()
	return ctx, nil
}

func (h *tokenLimitRedisCommandCounter) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *tokenLimitRedisCommandCounter) reset() {
	h.mu.Lock()
	h.count = 0
	h.mu.Unlock()
}

func (h *tokenLimitRedisCommandCounter) value() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func setupTokenLimitRedisTest(t *testing.T) (*miniredis.Miniredis, *tokenLimitRedisCommandCounter) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	counter := &tokenLimitRedisCommandCounter{}
	client.AddHook(counter)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	oldTimeout := common.RedisOperationTimeout
	common.RedisEnabled = true
	common.RDB = client
	common.RedisOperationTimeout = time.Second
	ResetTokenLimitForTest()
	t.Cleanup(func() {
		ResetTokenLimitForTest()
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		common.RedisOperationTimeout = oldTimeout
		require.NoError(t, client.Close())
	})
	return server, counter
}

func TestTokenRPMMemoryLimit(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	defer func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		ResetTokenLimitForTest()
	}()

	ResetTokenLimitForTest()
	require.True(t, CheckAndRecordTokenRPM(7, 2))
	require.True(t, CheckAndRecordTokenRPM(7, 2))
	require.False(t, CheckAndRecordTokenRPM(7, 2))
	require.True(t, CheckAndRecordTokenRPM(8, 2))
	require.True(t, CheckAndRecordTokenRPM(7, 0))
}

func TestTokenTPMMemoryLimitDoesNotConsumeRejectedAttempt(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	defer func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		ResetTokenLimitForTest()
	}()

	ResetTokenLimitForTest()
	require.True(t, CheckAndRecordTokenTPM(7, 10, 6))
	require.False(t, CheckAndRecordTokenTPM(7, 10, 5))
	require.True(t, CheckAndRecordTokenTPM(7, 10, 4))
	require.False(t, CheckAndRecordTokenTPM(7, 10, 11))
}

func TestTokenConcurrencyMemoryLimit(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	defer func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		ResetTokenLimitForTest()
	}()

	ResetTokenLimitForTest()
	require.True(t, TryAcquireTokenConcurrency(7, 1))
	require.False(t, TryAcquireTokenConcurrency(7, 1))
	ReleaseTokenConcurrency(7, 1)
	require.True(t, TryAcquireTokenConcurrency(7, 1))
	ReleaseTokenConcurrency(7, 1)
	require.True(t, TryAcquireTokenConcurrency(7, 0))
}

func TestTokenRateLimitRedisUsesOneAtomicRoundTrip(t *testing.T) {
	server, counter := setupTokenLimitRedisTest(t)

	require.True(t, CheckAndRecordTokenTPM(17, 10, 6))
	counter.reset()
	require.False(t, CheckAndRecordTokenTPM(17, 10, 5))
	require.Equal(t, 1, counter.value())
	require.True(t, CheckAndRecordTokenTPM(17, 10, 4))

	key := fmt.Sprintf("token_limit:tpm:17:%d", time.Now().Unix()/60)
	value, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "10", value)
	require.Greater(t, server.TTL(key), time.Minute)
}

func TestTokenConcurrencyRedisAcquireAndReleaseUseOneAtomicRoundTrip(t *testing.T) {
	server, counter := setupTokenLimitRedisTest(t)

	require.True(t, TryAcquireTokenConcurrency(23, 1))
	counter.reset()
	require.False(t, TryAcquireTokenConcurrency(23, 1))
	require.Equal(t, 1, counter.value())
	value, err := server.Get(tokenConcurrencyKey(23))
	require.NoError(t, err)
	require.Equal(t, "1", value)

	require.NoError(t, tokenConcurrencyReleaseScript.Load(context.Background(), common.RDB).Err())
	counter.reset()
	ReleaseTokenConcurrency(23, 1)
	require.Equal(t, 1, counter.value())
	require.False(t, server.Exists(tokenConcurrencyKey(23)))
}
