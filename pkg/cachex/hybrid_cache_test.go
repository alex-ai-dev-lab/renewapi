package cachex

import (
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/samber/hot"
	"github.com/stretchr/testify/require"
)

func TestHybridCacheCompareAndDeleteMemory(t *testing.T) {
	cache := NewHybridCache[int](HybridCacheConfig[int]{
		Namespace: "compare-delete-test",
		Memory: func() *hot.HotCache[string, int] {
			return hot.NewHotCache[string, int](hot.LRU, 8).Build()
		},
	})

	require.NoError(t, cache.SetWithTTL("session", 41, time.Minute))
	deleted, err := cache.CompareAndDelete("session", 42, func(a, b int) bool { return a == b })
	require.NoError(t, err)
	require.False(t, deleted)
	value, found, err := cache.Get("session")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 41, value)

	deleted, err = cache.CompareAndDelete("session", 41, func(a, b int) bool { return a == b })
	require.NoError(t, err)
	require.True(t, deleted)
	_, found, err = cache.Get("session")
	require.NoError(t, err)
	require.False(t, found)
}

func TestHybridCacheCompareAndDeleteRedis(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewHybridCache[int](HybridCacheConfig[int]{
		Namespace:  "compare-delete-redis-test",
		Redis:      client,
		RedisCodec: IntCodec{},
	})

	require.NoError(t, cache.SetWithTTL("session", 41, time.Minute))
	deleted, err := cache.CompareAndDelete("session", 42, func(a, b int) bool { return a == b })
	require.NoError(t, err)
	require.False(t, deleted)
	value, found, err := cache.Get("session")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 41, value)

	deleted, err = cache.CompareAndDelete("session", 41, func(a, b int) bool { return a == b })
	require.NoError(t, err)
	require.True(t, deleted)
	_, found, err = cache.Get("session")
	require.NoError(t, err)
	require.False(t, found)
}

func TestHybridCacheDeleteManySerializesWithCompareAndDelete(t *testing.T) {
	cache := NewHybridCache[int](HybridCacheConfig[int]{
		Namespace: "compare-delete-race-test",
		Memory: func() *hot.HotCache[string, int] {
			return hot.NewHotCache[string, int](hot.LRU, 8).Build()
		},
	})
	require.NoError(t, cache.SetWithTTL("session", 41, time.Minute))

	compareEntered := make(chan struct{})
	releaseCompare := make(chan struct{})
	var compareDeleted bool
	var compareErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		compareDeleted, compareErr = cache.CompareAndDelete("session", 41, func(a, b int) bool {
			close(compareEntered)
			<-releaseCompare
			return a == b
		})
	}()
	<-compareEntered

	bulkDone := make(chan map[string]bool, 1)
	go func() {
		deleted, _ := cache.DeleteMany([]string{"session"})
		bulkDone <- deleted
	}()
	select {
	case <-bulkDone:
		t.Fatal("DeleteMany bypassed the compare-and-delete key lock")
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseCompare)
	wg.Wait()
	require.NoError(t, compareErr)
	require.True(t, compareDeleted)
	require.False(t, (<-bulkDone)[cache.FullKey("session")])
}
