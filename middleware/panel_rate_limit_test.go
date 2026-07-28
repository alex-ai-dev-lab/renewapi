package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var panelTestUserID atomic.Int64

func withPanelRateLimitConfig(t *testing.T, readLimit int, writeLimit int) {
	t.Helper()
	oldEnabled := common.PanelRateLimitEnable
	oldRead := common.PanelReadRateLimitNum
	oldWrite := common.PanelWriteRateLimitNum
	oldDuration := common.PanelRateLimitDuration
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.PanelRateLimitEnable = true
	common.PanelReadRateLimitNum = readLimit
	common.PanelWriteRateLimitNum = writeLimit
	common.PanelRateLimitDuration = 60
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.PanelRateLimitEnable = oldEnabled
		common.PanelReadRateLimitNum = oldRead
		common.PanelWriteRateLimitNum = oldWrite
		common.PanelRateLimitDuration = oldDuration
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})
}

func newPanelTestRouter(userID int, method string, path string) *gin.Engine {
	router := gin.New()
	router.Handle(method, path, func(c *gin.Context) {
		if userID > 0 {
			c.Set("id", userID)
		}
		c.Next()
	}, PanelRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func doPanelRequest(router *gin.Engine, method string, path string) int {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(recorder, req)
	return recorder.Code
}

func nextPanelTestUserID() int {
	return int(panelTestUserID.Add(1)) + 100000
}

func TestPanelRateLimitSeparatesReadAndWriteBudgets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 1, 1)
	userID := nextPanelTestUserID()
	router := gin.New()
	auth := func(c *gin.Context) { c.Set("id", userID); c.Next() }
	router.GET("/resource", auth, PanelRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/resource", auth, PanelRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	if got := doPanelRequest(router, http.MethodGet, "/resource"); got != http.StatusNoContent {
		t.Fatalf("first read status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodGet, "/resource"); got != http.StatusTooManyRequests {
		t.Fatalf("second read status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodPost, "/resource"); got != http.StatusNoContent {
		t.Fatalf("write should use a separate budget, status = %d", got)
	}
}

func TestPanelRateLimitSharesBudgetAcrossEndpointsPerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 2, 2)
	userID := nextPanelTestUserID()
	router := gin.New()
	auth := func(c *gin.Context) { c.Set("id", userID); c.Next() }
	for _, path := range []string{"/one", "/two", "/three"} {
		router.GET(path, auth, PanelRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	}

	if got := doPanelRequest(router, http.MethodGet, "/one"); got != http.StatusNoContent {
		t.Fatalf("first endpoint status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodGet, "/two"); got != http.StatusNoContent {
		t.Fatalf("second endpoint status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodGet, "/three"); got != http.StatusTooManyRequests {
		t.Fatalf("shared budget status = %d", got)
	}
}

func TestPanelRateLimitSeparatesUsersAndChargesWeightedStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 5, 5)
	firstUser := nextPanelTestUserID()
	secondUser := nextPanelTestUserID()
	first := newPanelTestRouter(firstUser, http.MethodGet, "/api/stats/overview")
	second := newPanelTestRouter(secondUser, http.MethodGet, "/api/stats/overview")

	if got := doPanelRequest(first, http.MethodGet, "/api/stats/overview"); got != http.StatusNoContent {
		t.Fatalf("first weighted request status = %d", got)
	}
	if got := doPanelRequest(first, http.MethodGet, "/api/stats/overview"); got != http.StatusTooManyRequests {
		t.Fatalf("weighted request should exhaust budget, status = %d", got)
	}
	if got := doPanelRequest(second, http.MethodGet, "/api/stats/overview"); got != http.StatusNoContent {
		t.Fatalf("second user should have an independent budget, status = %d", got)
	}
}

func TestPanelRateLimitRedisFailureFallsBackLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 1, 1)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 20 * time.Millisecond,
		ReadTimeout: 20 * time.Millisecond,
	})
	t.Cleanup(func() { _ = common.RDB.Close() })
	userID := nextPanelTestUserID()
	router := newPanelTestRouter(userID, http.MethodGet, "/fallback")

	if got := doPanelRequest(router, http.MethodGet, "/fallback"); got != http.StatusNoContent {
		t.Fatalf("Redis failure must fall back locally, status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodGet, "/fallback"); got != http.StatusTooManyRequests {
		t.Fatalf("local fallback should still enforce the budget, status = %d", got)
	}
}

func TestPanelRateLimitRejectsMissingAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 10, 10)
	router := newPanelTestRouter(0, http.MethodGet, "/unauthenticated")
	if got := doPanelRequest(router, http.MethodGet, "/unauthenticated"); got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", got, http.StatusUnauthorized)
	}
}

func TestPanelRateLimitIsIdempotentWithinRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 1, 1)
	userID := nextPanelTestUserID()
	router := gin.New()
	router.GET("/nested", func(c *gin.Context) { c.Set("id", userID); c.Next() }, PanelRateLimit(), PanelRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if got := doPanelRequest(router, http.MethodGet, "/nested"); got != http.StatusNoContent {
		t.Fatalf("duplicate middleware must charge once, status = %d", got)
	}
	if got := doPanelRequest(router, http.MethodGet, "/nested"); got != http.StatusTooManyRequests {
		t.Fatalf("second request should exhaust budget, status = %d", got)
	}
}

func TestPanelRateLimitKeyDoesNotIncludeEndpointOrIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withPanelRateLimitConfig(t, 1, 1)
	userID := nextPanelTestUserID()
	key := fmt.Sprintf("rateLimit:panel:read:user:%d", userID)
	if !localPanelRateLimit(key, 1, 1) {
		t.Fatal("first local request should be allowed")
	}
	if localPanelRateLimit(key, 1, 1) {
		t.Fatal("same authenticated user must share a budget across IPs and endpoints")
	}
}
