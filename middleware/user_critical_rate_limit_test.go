package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

var userCriticalRateLimitTestID atomic.Int64

func withUserCriticalRateLimitConfig(t *testing.T, useRedis bool) {
	t.Helper()
	oldEnabled := common.CriticalRateLimitEnable
	oldLimit := common.CriticalRateLimitNum
	oldDuration := common.CriticalRateLimitDuration
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60
	common.RedisEnabled = useRedis
	common.RDB = nil
	if useRedis {
		server := miniredis.RunT(t)
		common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	}
	t.Cleanup(func() {
		if useRedis && common.RDB != nil {
			_ = common.RDB.Close()
		}
		common.CriticalRateLimitEnable = oldEnabled
		common.CriticalRateLimitNum = oldLimit
		common.CriticalRateLimitDuration = oldDuration
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})
}

func newUserCriticalRateLimitRouter(scope string) *gin.Engine {
	router := gin.New()
	router.GET("/critical", func(c *gin.Context) {
		userID, _ := strconv.Atoi(c.GetHeader("X-Test-User"))
		c.Set("id", userID)
		c.Next()
	}, UserCriticalRateLimit(scope), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func performUserCriticalRequest(router *gin.Engine, userID int) int {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/critical", nil)
	request.Header.Set("X-Test-User", strconv.Itoa(userID))
	router.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestUserCriticalRateLimitIsolatesUsersAndScopesAcrossBackends(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			withUserCriticalRateLimitConfig(t, backend == "redis")
			sequence := userCriticalRateLimitTestID.Add(1)
			userOne := int(sequence*10 + 1)
			userTwo := int(sequence*10 + 2)
			tokenScope := fmt.Sprintf("token-%d", sequence)
			transferScope := fmt.Sprintf("aff-transfer-%d", sequence)
			tokenRouter := newUserCriticalRateLimitRouter(tokenScope)
			transferRouter := newUserCriticalRateLimitRouter(transferScope)

			require.Equal(t, http.StatusNoContent, performUserCriticalRequest(tokenRouter, userOne))
			require.Equal(t, http.StatusTooManyRequests, performUserCriticalRequest(tokenRouter, userOne))
			require.Equal(t, http.StatusNoContent, performUserCriticalRequest(tokenRouter, userTwo))
			require.Equal(t, http.StatusNoContent, performUserCriticalRequest(transferRouter, userOne))
		})
	}
}

func TestUserCriticalRateLimitRejectsMissingAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUserCriticalRateLimitConfig(t, false)
	router := newUserCriticalRateLimitRouter(fmt.Sprintf("missing-%d", userCriticalRateLimitTestID.Add(1)))
	require.Equal(t, http.StatusUnauthorized, performUserCriticalRequest(router, 0))
}
