package common

import (
	"container/list"
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*rateLimitBucket
	lru                list.List
	mutex              sync.Mutex
	expirationDuration time.Duration
}

type rateLimitEntry struct {
	timestamp     int64
	reservationID string
}

type rateLimitBucket struct {
	entries    []rateLimitEntry
	head       int
	lastActive time.Time
	window     int64
	element    *list.Element
}

func (b *rateLimitBucket) size() int { return len(b.entries) - b.head }

func (b *rateLimitBucket) expire(now, duration int64) {
	for b.head < len(b.entries) && now-b.entries[b.head].timestamp >= duration {
		b.entries[b.head] = rateLimitEntry{}
		b.head++
	}
	if b.head == len(b.entries) {
		b.entries = b.entries[:0]
		b.head = 0
	}
}

func (b *rateLimitBucket) append(now int64, weight int, reservationID string) {
	// Reuse expired slots before growing; capacity follows actual traffic.
	if b.head > 0 && (len(b.entries)+weight > cap(b.entries) || b.head >= len(b.entries)/2) {
		size := copy(b.entries, b.entries[b.head:])
		clear(b.entries[size:])
		b.entries = b.entries[:size]
		b.head = 0
	}
	for range weight {
		b.entries = append(b.entries, rateLimitEntry{timestamp: now, reservationID: reservationID})
	}
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store != nil {
		return
	}
	l.store = make(map[string]*rateLimitBucket)
	l.expirationDuration = expirationDuration
	if expirationDuration > 0 {
		go l.clearExpiredItems()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	ticker := time.NewTicker(l.expirationDuration)
	defer ticker.Stop()
	for now := range ticker.C {
		l.deleteExpiredEntries(now)
	}
}

func (l *InMemoryRateLimiter) deleteExpiredEntries(now time.Time) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.expirationDuration <= 0 {
		return
	}
	for oldest := l.lru.Back(); oldest != nil; {
		previous := oldest.Prev()
		key := oldest.Value.(string)
		bucket := l.store[key]
		if now.Sub(bucket.lastActive) <= l.expirationDuration {
			return
		}
		// The shared limiter serves different windows. An idle bucket can
		// still contain a live budget longer than the cleanup interval.
		if bucket.size() == 0 || now.Unix()-bucket.entries[len(bucket.entries)-1].timestamp >= bucket.window {
			l.removeBucket(key, bucket)
		}
		oldest = previous
	}
}

// removeBucket requires mutex ownership.
func (l *InMemoryRateLimiter) removeBucket(key string, bucket *rateLimitBucket) {
	delete(l.store, key)
	l.lru.Remove(bucket.element)
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	return l.RequestN(key, maxRequestNum, duration, 1)
}

// RequestN atomically consumes weight entries from a sliding-window budget.
func (l *InMemoryRateLimiter) RequestN(key string, maxRequestNum int, duration int64, weight int) bool {
	return l.reserveN(key, maxRequestNum, duration, weight, "")
}

// Reserve atomically reserves one entry identified by reservationID. The
// reservation can be cancelled when a request fails before semantic success.
func (l *InMemoryRateLimiter) Reserve(key string, maxRequestNum int, duration int64, reservationID string) bool {
	return l.reserveN(key, maxRequestNum, duration, 1, reservationID)
}

func (l *InMemoryRateLimiter) reserveN(key string, maxRequestNum int, duration int64, weight int, reservationID string) bool {
	if maxRequestNum <= 0 {
		return true
	}
	if weight <= 0 {
		weight = 1
	}
	if weight > maxRequestNum {
		return false
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	now := time.Now()
	bucket, ok := l.store[key]
	if !ok {
		bucket = &rateLimitBucket{element: l.lru.PushFront(key)}
		l.store[key] = bucket
	} else {
		bucket.expire(now.Unix(), duration)
		l.lru.MoveToFront(bucket.element)
	}
	bucket.lastActive = now
	bucket.window = duration
	if weight > maxRequestNum-bucket.size() {
		return false
	}
	bucket.append(now.Unix(), weight, reservationID)
	return true
}

// Cancel removes one reservation. A blank ID is intentionally ignored because
// legacy Request/RequestN entries are permanent window observations.
func (l *InMemoryRateLimiter) Cancel(key, reservationID string) {
	if reservationID == "" {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	bucket, ok := l.store[key]
	if !ok {
		return
	}
	for i := bucket.head; i < len(bucket.entries); i++ {
		if bucket.entries[i].reservationID == reservationID {
			copy(bucket.entries[i:], bucket.entries[i+1:])
			bucket.entries[len(bucket.entries)-1] = rateLimitEntry{}
			bucket.entries = bucket.entries[:len(bucket.entries)-1]
			if bucket.size() == 0 {
				l.removeBucket(key, bucket)
			}
			return
		}
	}
}

func (l *InMemoryRateLimiter) RetryAfter(key string, duration int64) int64 {
	if duration <= 0 {
		return 1
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	bucket, ok := l.store[key]
	if !ok {
		return 1
	}
	now := time.Now().Unix()
	bucket.expire(now, duration)
	if bucket.size() == 0 {
		l.removeBucket(key, bucket)
		return 1
	}
	return max(1, duration-(now-bucket.entries[bucket.head].timestamp))
}
