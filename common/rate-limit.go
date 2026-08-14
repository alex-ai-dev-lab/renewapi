package common

import (
	"math"
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]rateLimitEntry
	mutex              sync.Mutex
	expirationDuration time.Duration
}

type rateLimitEntry struct {
	timestamp     int64
	reservationID string
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	if l.store == nil {
		l.mutex.Lock()
		if l.store == nil {
			l.store = make(map[string]*[]rateLimitEntry)
			l.expirationDuration = expirationDuration
			if expirationDuration > 0 {
				go l.clearExpiredItems()
			}
		}
		l.mutex.Unlock()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1].timestamp > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
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
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		firstValid := 0
		for firstValid < len(*queue) && now-(*queue)[firstValid].timestamp >= duration {
			firstValid++
		}
		if firstValid > 0 {
			*queue = (*queue)[firstValid:]
		}
		if len(*queue)+weight > maxRequestNum {
			return false
		}
	} else {
		s := make([]rateLimitEntry, 0, maxRequestNum)
		l.store[key] = &s
	}
	for range weight {
		*(l.store[key]) = append(*(l.store[key]), rateLimitEntry{timestamp: now, reservationID: reservationID})
	}
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
	queue, ok := l.store[key]
	if !ok {
		return
	}
	for i, entry := range *queue {
		if entry.reservationID == reservationID {
			*queue = append((*queue)[:i], (*queue)[i+1:]...)
			if len(*queue) == 0 {
				delete(l.store, key)
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
	queue, ok := l.store[key]
	if !ok || len(*queue) == 0 {
		return 1
	}
	now := time.Now().Unix()
	firstValid := 0
	for firstValid < len(*queue) && now-(*queue)[firstValid].timestamp >= duration {
		firstValid++
	}
	if firstValid > 0 {
		*queue = (*queue)[firstValid:]
	}
	if len(*queue) == 0 {
		delete(l.store, key)
		return 1
	}
	retryAfter := duration - (now - (*queue)[0].timestamp)
	if retryAfter <= 0 {
		return 1
	}
	return int64(math.Ceil(float64(retryAfter)))
}
