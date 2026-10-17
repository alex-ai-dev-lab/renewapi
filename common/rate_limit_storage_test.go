package common

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllocatesForTrafficAndCompacts(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	if !limiter.Request("cold", 100000, 60) {
		t.Fatal("cold request rejected")
	}
	bucket := limiter.store["cold"]
	if cap(bucket.entries) > 16 {
		t.Fatalf("cold key allocated %d entries", cap(bucket.entries))
	}
	now := time.Now().Unix()
	for i := range 10 {
		bucket.entries = append(bucket.entries, rateLimitEntry{timestamp: now, reservationID: fmt.Sprint(i)})
	}
	for i := range 6 {
		bucket.entries[i].timestamp = now - 60
	}
	bucket.expire(now, 60)
	if bucket.size() != 5 {
		t.Fatalf("size=%d", bucket.size())
	}
	bucket.append(now, 1, "new")
	if bucket.head != 0 || bucket.size() != 6 || bucket.entries[0].reservationID != "5" {
		t.Fatalf("compaction lost order: %+v", bucket)
	}
	limiter.Cancel("cold", "7")
	limiter.Cancel("cold", "7")
	if bucket.size() != 5 {
		t.Fatal("cancellation not idempotent")
	}
	if limiter.RetryAfter("cold", 60) < 59 {
		t.Fatal("retry wait lost oldest entry")
	}
	for i := range bucket.entries {
		bucket.entries[i].timestamp = now - 60
	}
	if limiter.RetryAfter("cold", 60) != 1 || len(limiter.store) != 0 || limiter.lru.Len() != 0 {
		t.Fatal("expired bucket not fully removed")
	}
}

func TestRateLimiterCleanupIdleOrderAndRejectedActivity(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0) // deterministic cleanup, no ticker
	limiter.expirationDuration = time.Minute
	for _, key := range []string{"old", "hot", "idle"} {
		limiter.Request(key, 1, 60)
	}
	if limiter.Request("hot", 1, 60) {
		t.Fatal("full budget accepted")
	}
	now := time.Now()
	limiter.store["old"].lastActive = now.Add(-2 * time.Minute)
	limiter.store["idle"].lastActive = now.Add(-90 * time.Second)
	limiter.store["old"].entries[0].timestamp = now.Unix() - 120
	limiter.store["idle"].entries[0].timestamp = now.Unix() - 90
	limiter.deleteExpiredEntries(now)
	if len(limiter.store) != 1 || limiter.lru.Len() != 1 || limiter.store["hot"] == nil {
		t.Fatal("idle cleanup removed active/rejected bucket or leaked list entries")
	}
	limiter.deleteExpiredEntries(now.Add(2 * time.Minute))
	if len(limiter.store) != 0 || limiter.lru.Len() != 0 {
		t.Fatal("idle key retained")
	}
}

func TestRateLimiterConcurrentReservationsAndInit(t *testing.T) {
	var limiter InMemoryRateLimiter
	var wg sync.WaitGroup
	for worker := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Init(0)
			id := fmt.Sprint(worker)
			for range 100 {
				if !limiter.Reserve("shared", 32, 60, id) {
					t.Error("reservation rejected despite room")
				}
				limiter.RetryAfter("shared", 60)
				limiter.Cancel("shared", id)
			}
		}()
	}
	wg.Wait()
	if len(limiter.store) != 0 || limiter.lru.Len() != 0 {
		t.Fatal("cancelled concurrent reservations leaked")
	}
	if !limiter.RequestN("unlimited", 0, 60, 100) || limiter.RequestN("overweight", 1, 60, 2) {
		t.Fatal("budget boundary changed")
	}
	if !limiter.RequestN("default-weight", 1, 60, 0) || limiter.Request("default-weight", 1, 60) {
		t.Fatal("default weight changed")
	}
}

func BenchmarkRateLimiterColdKeyHighLimit(b *testing.B) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	b.ReportAllocs()
	for b.Loop() {
		if !limiter.Reserve("cold", 10000, 60, "id") {
			b.Fatal("reserve")
		}
		limiter.Cancel("cold", "id")
	}
}

func TestRateLimiterCleanupRespectsEachRequestWindow(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	limiter.expirationDuration = 10 * time.Second
	limiter.Request("long", 1, 60)
	limiter.Request("short", 1, 5)
	limiter.Request("active", 1, 60)
	now := time.Now()
	for _, key := range []string{"long", "short"} {
		limiter.store[key].lastActive = now.Add(-20 * time.Second)
		limiter.store[key].entries[0].timestamp = now.Unix() - 20
	}
	limiter.deleteExpiredEntries(now)
	if limiter.store["long"] == nil {
		t.Fatal("cleanup discarded a live budget whose request window exceeds the cleanup interval")
	}
	if limiter.store["short"] != nil {
		t.Fatal("long idle bucket prevented cleanup of a later expired bucket")
	}
	if limiter.Request("long", 1, 60) {
		t.Fatal("cleanup allowed a request before its window expired")
	}
	limiter.deleteExpiredEntries(now.Add(61 * time.Second))
	if len(limiter.store) != 0 || limiter.lru.Len() != 0 {
		t.Fatal("expired budgets leaked")
	}
}

func BenchmarkRateLimiterHotKey(b *testing.B) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	limiter.Request("hot", 10000, 60)
	b.ReportAllocs()
	for b.Loop() {
		limiter.Request("hot", 10000, 60)
	}
}
