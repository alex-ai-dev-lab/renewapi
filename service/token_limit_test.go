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
	slot, allowed := AcquireTokenConcurrency(context.Background(), 7, 1)
	require.True(t, allowed)
	_, allowed = AcquireTokenConcurrency(context.Background(), 7, 1)
	require.False(t, allowed)
	slot.Release()
	slot.Release()
	slot, allowed = AcquireTokenConcurrency(context.Background(), 7, 1)
	require.True(t, allowed)
	slot.Release()
	slot, allowed = AcquireTokenConcurrency(context.Background(), 7, 0)
	require.True(t, allowed)
	slot.Release()
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

	slot, allowed := AcquireTokenConcurrency(context.Background(), 23, 1)
	require.True(t, allowed)
	t.Cleanup(slot.Release)
	counter.reset()
	_, allowed = AcquireTokenConcurrency(context.Background(), 23, 1)
	require.False(t, allowed)
	require.Equal(t, 1, counter.value())
	value, err := server.ZMembers(tokenConcurrencyKey(23))
	require.NoError(t, err)
	require.Equal(t, []string{slot.id}, value)

	require.NoError(t, tokenConcurrencyReleaseScript.Load(context.Background(), common.RDB).Err())
	counter.reset()
	slot.Release()
	require.Equal(t, 1, counter.value())
	slot.Release()
	require.Equal(t, 1, counter.value(), "repeated release performed another Redis write")
	require.False(t, server.Exists(tokenConcurrencyKey(23)))
}

func TestTokenConcurrencyFallbackDoesNotReleaseRedisRequest(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	redisSlot, allowed := AcquireTokenConcurrency(context.Background(), 31, 2)
	require.True(t, allowed)
	t.Cleanup(redisSlot.Release)
	server.SetError("temporary Redis failure")
	fallbackSlot, allowed := AcquireTokenConcurrency(context.Background(), 31, 2)
	require.True(t, allowed)
	t.Cleanup(fallbackSlot.Release)
	server.SetError("")
	fallbackSlot.Release() // The fallback request completes first.
	value, err := server.ZMembers(tokenConcurrencyKey(31))
	require.NoError(t, err)
	require.Equal(t, []string{redisSlot.id}, value, "fallback release removed another request's Redis slot")
	require.Empty(t, tokenConcurrencyCount)
}

func TestTokenConcurrencyLongRequestKeepsSlot(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	now := time.Now()
	server.SetTime(now)
	slot, allowed := AcquireTokenConcurrency(context.Background(), 32, 1)
	require.True(t, allowed)
	t.Cleanup(slot.Release)
	server.FastForward(time.Minute)
	server.SetTime(now.Add(time.Minute))
	renewed, err := slot.renew(context.Background())
	require.NoError(t, err)
	require.True(t, renewed)
	server.FastForward(61 * time.Second)
	server.SetTime(now.Add(121 * time.Second))
	_, allowed = AcquireTokenConcurrency(context.Background(), 32, 1)
	require.False(t, allowed, "active long request lost its concurrency budget")
}

func TestTokenConcurrencyExpiredReleaseCannotRemoveReplacement(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	now := time.Now()
	server.SetTime(now)
	old, allowed := AcquireTokenConcurrency(context.Background(), 33, 1)
	require.True(t, allowed)
	t.Cleanup(old.Release)
	server.FastForward(121 * time.Second)
	server.SetTime(now.Add(121 * time.Second))
	replacement, allowed := AcquireTokenConcurrency(context.Background(), 33, 1)
	require.True(t, allowed)
	t.Cleanup(replacement.Release)
	renewed, err := old.renew(context.Background())
	require.NoError(t, err)
	require.False(t, renewed, "expired lease recreated its slot after replacement")
	old.Release()
	members, err := server.ZMembers(tokenConcurrencyKey(33))
	require.NoError(t, err)
	require.Equal(t, []string{replacement.id}, members)
}

func TestTokenConcurrencyReleaseUsesOriginalRedisClient(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	slot, allowed := AcquireTokenConcurrency(context.Background(), 34, 1)
	require.True(t, allowed)
	t.Cleanup(slot.Release)
	client := common.RDB
	common.RDB = nil
	common.RedisEnabled = false
	slot.Release()
	common.RDB = client
	common.RedisEnabled = true
	require.False(t, server.Exists(tokenConcurrencyKey(34)))
	require.Empty(t, tokenConcurrencyCount)
}

func TestTokenConcurrencyWorkerRenewsAndCancelsLostLease(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	now := time.Now()
	server.SetTime(now)
	slot := &TokenConcurrencySlot{client: common.RDB, key: tokenConcurrencyKey(35), id: "worker"}
	allowed, err := tokenConcurrencyAcquireScript.Run(context.Background(), slot.client, []string{slot.key}, 1, tokenConcurrencyLeaseDuration.Milliseconds(), slot.id).Int()
	require.NoError(t, err)
	require.Equal(t, 1, allowed)
	initial, err := server.ZScore(slot.key, slot.id)
	require.NoError(t, err)
	server.SetTime(now.Add(time.Minute))
	slot.startRenewal(context.Background(), 5*time.Millisecond)
	t.Cleanup(slot.Release)
	require.Eventually(t, func() bool {
		score, err := server.ZScore(slot.key, slot.id)
		return err == nil && score > initial
	}, time.Second, time.Millisecond, "worker did not renew the active slot")
	server.Del(slot.key)
	require.Eventually(t, func() bool { return slot.Context.Err() != nil }, time.Second, time.Millisecond,
		"lost lease did not cancel the request")
	slot.Release()
	require.False(t, server.Exists(slot.key), "worker recreated its released lease")
}

func TestTokenConcurrencyCancelledRequestStillReleasesRedisSlot(t *testing.T) {
	server, _ := setupTokenLimitRedisTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slot, allowed := AcquireTokenConcurrency(ctx, 36, 1)
	require.True(t, allowed)
	t.Cleanup(slot.Release)
	cancel()
	slot.Release()
	require.ErrorIs(t, slot.Context.Err(), context.Canceled)
	require.False(t, server.Exists(tokenConcurrencyKey(36)))
}
