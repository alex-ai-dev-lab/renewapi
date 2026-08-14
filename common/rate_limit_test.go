package common

import (
	"testing"
	"time"
)

func TestInMemoryRateLimiterRequestNIsAtomic(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(time.Minute)

	if !limiter.RequestN("weighted", 5, 60, 3) {
		t.Fatal("first weighted request should be allowed")
	}
	if limiter.RequestN("weighted", 5, 60, 3) {
		t.Fatal("request exceeding remaining budget should be rejected")
	}
	if !limiter.RequestN("weighted", 5, 60, 2) {
		t.Fatal("rejected request must not partially consume the budget")
	}
	if limiter.Request("weighted", 5, 60) {
		t.Fatal("budget should be exhausted")
	}
}

func TestInMemoryRateLimiterRequestNRejectsOversizedWeight(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(time.Minute)
	if limiter.RequestN("oversized", 2, 60, 3) {
		t.Fatal("weight larger than the configured budget should be rejected")
	}
	if !limiter.RequestN("oversized", 2, 60, 2) {
		t.Fatal("rejected oversized request must not consume budget")
	}
}

func TestInMemoryRateLimiterReservationCanBeCancelled(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(time.Minute)
	if !limiter.Reserve("reservation", 1, 60, "request-1") {
		t.Fatal("reservation should be allowed")
	}
	if limiter.Reserve("reservation", 1, 60, "request-2") {
		t.Fatal("concurrent reservation should see the occupied slot")
	}
	limiter.Cancel("reservation", "request-1")
	if !limiter.Reserve("reservation", 1, 60, "request-2") {
		t.Fatal("cancelled reservation should release the slot")
	}
}
