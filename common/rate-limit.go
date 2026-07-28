package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	if l.store == nil {
		l.mutex.Lock()
		if l.store == nil {
			l.store = make(map[string]*[]int64)
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
			if size == 0 || now-(*queue)[size-1] > int64(l.expirationDuration.Seconds()) {
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
		for firstValid < len(*queue) && now-(*queue)[firstValid] >= duration {
			firstValid++
		}
		if firstValid > 0 {
			*queue = (*queue)[firstValid:]
		}
		if len(*queue)+weight > maxRequestNum {
			return false
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
	}
	for range weight {
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}
