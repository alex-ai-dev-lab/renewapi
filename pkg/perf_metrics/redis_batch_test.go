package perfmetrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

type perfMetricRedisCommandCounter struct {
	mu       sync.Mutex
	pipeline int
}

func (h *perfMetricRedisCommandCounter) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *perfMetricRedisCommandCounter) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *perfMetricRedisCommandCounter) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	h.pipeline++
	h.mu.Unlock()
	return ctx, nil
}

func (h *perfMetricRedisCommandCounter) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *perfMetricRedisCommandCounter) pipelineCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pipeline
}

func clearMetricBuckets(buckets *sync.Map) {
	buckets.Range(func(key, _ any) bool {
		buckets.Delete(key)
		return true
	})
}

func TestRedisMetricsBatchMultipleSamplesIntoOnePipeline(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	counter := &perfMetricRedisCommandCounter{}
	client.AddHook(counter)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	oldTimeout := common.RedisOperationTimeout
	common.RedisEnabled = true
	common.RDB = client
	common.RedisOperationTimeout = time.Second
	clearMetricBuckets(&hotBuckets)
	clearMetricBuckets(&redisPendingBuckets)
	t.Cleanup(func() {
		clearMetricBuckets(&hotBuckets)
		clearMetricBuckets(&redisPendingBuckets)
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		common.RedisOperationTimeout = oldTimeout
		require.NoError(t, client.Close())
	})

	for i := 0; i < 100; i++ {
		Record(Sample{
			Model:        "gpt-batch",
			Group:        "default",
			LatencyMs:    20,
			TtftMs:       5,
			HasTtft:      true,
			Success:      i%4 != 0,
			OutputTokens: 10,
			GenerationMs: 15,
		})
	}
	key := bucketKey{model: "gpt-batch", group: "default", bucketTs: bucketStart(time.Now().Unix())}
	require.False(t, server.Exists(redisBucketKey(key)))

	flushRedisPendingBuckets()
	require.Equal(t, 1, counter.pipelineCount())
	require.Equal(t, "100", server.HGet(redisBucketKey(key), "req"))
	require.Equal(t, "75", server.HGet(redisBucketKey(key), "ok"))
	require.Equal(t, "2000", server.HGet(redisBucketKey(key), "lat"))
	require.Equal(t, "500", server.HGet(redisBucketKey(key), "ttft"))
	require.Equal(t, "100", server.HGet(redisBucketKey(key), "ttft_n"))
	require.Equal(t, "1000", server.HGet(redisBucketKey(key), "out"))
	require.Equal(t, "1500", server.HGet(redisBucketKey(key), "gen_ms"))
	require.Equal(t, time.Hour, server.TTL(redisBucketKey(key)))
}

func TestRedisMetricsRestorePendingCountersAfterFlushFailure(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	oldTimeout := common.RedisOperationTimeout
	common.RedisEnabled = true
	common.RDB = client
	common.RedisOperationTimeout = time.Second
	clearMetricBuckets(&redisPendingBuckets)
	t.Cleanup(func() {
		clearMetricBuckets(&redisPendingBuckets)
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		common.RedisOperationTimeout = oldTimeout
	})

	Record(Sample{Model: "gpt-failure", Group: "default", Success: true})
	require.NoError(t, client.Close())
	flushRedisPendingBuckets()

	key := bucketKey{model: "gpt-failure", group: "default", bucketTs: bucketStart(time.Now().Unix())}
	value, ok := redisPendingBuckets.Load(key)
	require.True(t, ok)
	require.EqualValues(t, 1, value.(*atomicBucket).snapshot().requestCount)
}
