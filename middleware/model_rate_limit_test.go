package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
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
