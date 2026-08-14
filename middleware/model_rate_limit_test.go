package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestModelRateLimitFailsClosedWhenRedisClientIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldEnabled := setting.ModelRequestRateLimitEnabled
	oldDuration := setting.ModelRequestRateLimitDurationMinutes
	oldCount := setting.ModelRequestRateLimitCount
	oldSuccessCount := setting.ModelRequestRateLimitSuccessCount
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	t.Cleanup(func() {
		setting.ModelRequestRateLimitEnabled = oldEnabled
		setting.ModelRequestRateLimitDurationMinutes = oldDuration
		setting.ModelRequestRateLimitCount = oldCount
		setting.ModelRequestRateLimitSuccessCount = oldSuccessCount
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})

	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	setting.ModelRequestRateLimitCount = 1
	setting.ModelRequestRateLimitSuccessCount = 1
	common.RedisEnabled = true
	common.RDB = nil

	router := gin.New()
	router.POST("/v1/chat", func(c *gin.Context) {
		c.Set("id", 1)
		c.Next()
	}, ModelRequestRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestModelRateLimitFailuresDoNotConsumeSuccessBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			useRedis := backend == "redis"
			var rdb *redis.Client
			if useRedis {
				server := miniredis.RunT(t)
				rdb = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = rdb.Close() })
			}
			oldRDB := common.RDB
			common.RDB = rdb
			t.Cleanup(func() { common.RDB = oldRDB })

			userID := 9101
			router := gin.New()
			router.POST("/v1/chat", func(c *gin.Context) {
				c.Set("id", userID)
				c.Next()
			}, modelRateLimitHandler(60, 0, 1, useRedis), func(c *gin.Context) {
				if c.Query("success") == "1" {
					service.SetRelaySemanticSuccess(c, true)
					c.Status(http.StatusNoContent)
					return
				}
				c.Status(http.StatusServiceUnavailable)
			})

			for range 20 {
				response := performModelRateLimitRequest(router, "/v1/chat")
				require.Equal(t, http.StatusServiceUnavailable, response.Code)
			}
			response := performModelRateLimitRequest(router, "/v1/chat?success=1")
			require.Equal(t, http.StatusNoContent, response.Code)

			response = performModelRateLimitRequest(router, "/v1/chat?success=1")
			require.Equal(t, http.StatusTooManyRequests, response.Code)
			require.Equal(t, "60", response.Header().Get("Retry-After"))
			require.Equal(t, "1", response.Header().Get("X-RateLimit-Limit-Requests"))
			require.Equal(t, "0", response.Header().Get("X-RateLimit-Remaining-Requests"))
			require.Equal(t, "60s", response.Header().Get("X-RateLimit-Reset-Requests"))
			require.Contains(t, response.Body.String(), `"code":"`+string(types.ErrorCodeRateLimitExceeded)+`"`)
			require.Contains(t, response.Body.String(), `"error"`)
		})
	}
}

func TestModelRateLimitMemoryAndRedisTotalBudgetParity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			useRedis := backend == "redis"
			var rdb *redis.Client
			if useRedis {
				server := miniredis.RunT(t)
				rdb = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = rdb.Close() })
			}
			oldRDB := common.RDB
			common.RDB = rdb
			t.Cleanup(func() { common.RDB = oldRDB })

			router := gin.New()
			router.POST("/v1/chat", func(c *gin.Context) {
				c.Set("id", 9201)
				c.Next()
			}, modelRateLimitHandler(60, 2, 0, useRedis), func(c *gin.Context) {
				if strings.Contains(c.Query("status"), "500") {
					service.SetModelRouteOutcome(c, service.ModelRouteOutcomeServerError)
					c.Status(http.StatusServiceUnavailable)
					return
				}
				c.Status(http.StatusBadRequest)
			})

			// Server failures are refunded, while client failures consume total budget.
			require.Equal(t, http.StatusServiceUnavailable, performModelRateLimitRequest(router, "/v1/chat?status=500").Code)
			require.Equal(t, http.StatusBadRequest, performModelRateLimitRequest(router, "/v1/chat").Code)
			require.Equal(t, http.StatusBadRequest, performModelRateLimitRequest(router, "/v1/chat").Code)
			limited := performModelRateLimitRequest(router, "/v1/chat")
			require.Equal(t, http.StatusTooManyRequests, limited.Code)
			require.Equal(t, "60", limited.Header().Get("Retry-After"))
		})
	}
}

func TestModelRateLimitUpstream429RefundsTotalBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			useRedis := backend == "redis"
			var rdb *redis.Client
			if useRedis {
				server := miniredis.RunT(t)
				rdb = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = rdb.Close() })
			}
			oldRDB := common.RDB
			common.RDB = rdb
			t.Cleanup(func() { common.RDB = oldRDB })

			router := gin.New()
			router.POST("/v1/chat", func(c *gin.Context) {
				c.Set("id", 9251)
				c.Next()
			}, modelRateLimitHandler(60, 1, 0, useRedis), func(c *gin.Context) {
				service.SetModelRouteOutcome(c, service.ModelRouteOutcomeUpstreamRetryable)
				c.Status(http.StatusTooManyRequests)
			})

			for range 3 {
				response := performModelRateLimitRequest(router, "/v1/chat")
				require.Equal(t, http.StatusTooManyRequests, response.Code)
			}
		})
	}
}

func TestModelRateLimitBare429ConsumesTotalBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			useRedis := backend == "redis"
			var rdb *redis.Client
			if useRedis {
				server := miniredis.RunT(t)
				rdb = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = rdb.Close() })
			}
			oldRDB := common.RDB
			common.RDB = rdb
			t.Cleanup(func() { common.RDB = oldRDB })

			router := gin.New()
			router.POST("/v1/chat", func(c *gin.Context) {
				c.Set("id", 9261)
				c.Next()
			}, modelRateLimitHandler(60, 1, 0, useRedis), func(c *gin.Context) {
				c.Status(http.StatusTooManyRequests)
			})

			first := performModelRateLimitRequest(router, "/v1/chat")
			require.Equal(t, http.StatusTooManyRequests, first.Code)
			second := performModelRateLimitRequest(router, "/v1/chat")
			require.Equal(t, http.StatusTooManyRequests, second.Code)
			require.Contains(t, second.Body.String(), `"code":"`+string(types.ErrorCodeRateLimitExceeded)+`"`)
		})
	}
}

func TestRedisSlidingWindowReservationCanBeCancelled(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()

	allowed, err := redisSlidingWindowReserve(ctx, rdb, "reservation", 1, 60, time.Minute, "request-1")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = redisSlidingWindowReserve(ctx, rdb, "reservation", 1, 60, time.Minute, "request-2")
	require.NoError(t, err)
	require.False(t, allowed)
	require.NoError(t, redisSlidingWindowCancel(ctx, rdb, "reservation", "request-1"))
	allowed, err = redisSlidingWindowReserve(ctx, rdb, "reservation", 1, 60, time.Minute, "request-2")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestModelRateLimitSuccessReservationIsConcurrencySafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			useRedis := backend == "redis"
			var rdb *redis.Client
			if useRedis {
				server := miniredis.RunT(t)
				rdb = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = rdb.Close() })
			}
			oldRDB := common.RDB
			common.RDB = rdb
			t.Cleanup(func() { common.RDB = oldRDB })

			started := make(chan struct{})
			release := make(chan struct{})
			router := gin.New()
			router.POST("/v1/chat", func(c *gin.Context) {
				c.Set("id", 9301)
				c.Next()
			}, modelRateLimitHandler(60, 0, 1, useRedis), func(c *gin.Context) {
				close(started)
				<-release
				service.SetRelaySemanticSuccess(c, true)
				c.Status(http.StatusNoContent)
			})

			firstDone := make(chan *httptest.ResponseRecorder, 1)
			go func() { firstDone <- performModelRateLimitRequest(router, "/v1/chat") }()
			<-started
			second := performModelRateLimitRequest(router, "/v1/chat")
			require.Equal(t, http.StatusTooManyRequests, second.Code)
			close(release)
			first := <-firstDone
			require.Equal(t, http.StatusNoContent, first.Code)
		})
	}
}

func performModelRateLimitRequest(router http.Handler, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, target, nil))
	return response
}
