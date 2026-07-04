package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const (
	EmailVerificationRateLimitMark = "EV"
	EmailVerificationMaxRequests   = 2  // 30秒内最多2次
	EmailVerificationDuration      = 30 // 30秒时间窗口
	EmailVerificationDailyMax      = 10 // 单邮箱每日最多10次
	EmailVerificationDailyDuration = 86400
)

func emailVerificationRateLimitKeys(c *gin.Context) []string {
	ip := c.ClientIP()
	email := strings.ToLower(strings.TrimSpace(c.Query("email")))
	keys := []string{EmailVerificationRateLimitMark + ":ip:" + ip}
	if email == "" {
		return keys
	}
	keys = append(keys,
		EmailVerificationRateLimitMark+":email:"+email,
		EmailVerificationRateLimitMark+":email-day:"+email,
	)
	return keys
}

func redisEmailVerificationRateLimiter(c *gin.Context) {
	ctx := context.Background()
	rdb := common.RDB
	keys := emailVerificationRateLimitKeys(c)

	for i, suffix := range keys {
		limit := EmailVerificationMaxRequests
		duration := EmailVerificationDuration
		if i == 2 {
			limit = EmailVerificationDailyMax
			duration = EmailVerificationDailyDuration
		}

		key := "emailVerification:" + suffix
		allowed, err := redisSlidingWindowRateLimitScript.Run(ctx, rdb, []string{key}, limit, int64(duration)*1000, int64(duration)).Int()
		if err != nil {
			// fallback
			memoryEmailVerificationRateLimiter(c)
			return
		}

		if allowed == 1 {
			continue
		}

		ttl, err := rdb.TTL(ctx, key).Result()
		waitSeconds := int64(duration)
		if err == nil && ttl > 0 {
			waitSeconds = int64(ttl.Seconds())
		}
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", waitSeconds),
		})
		c.Abort()
		return
	}

	c.Next()
}

func memoryEmailVerificationRateLimiter(c *gin.Context) {
	for i, key := range emailVerificationRateLimitKeys(c) {
		limit := EmailVerificationMaxRequests
		duration := int64(EmailVerificationDuration)
		if i == 2 {
			limit = EmailVerificationDailyMax
			duration = EmailVerificationDailyDuration
		}
		if inMemoryRateLimiter.Request(key, limit, duration) {
			continue
		}
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": "发送过于频繁，请稍后再试",
		})
		c.Abort()
		return
	}

	c.Next()
}

func EmailVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.RedisEnabled {
			redisEmailVerificationRateLimiter(c)
		} else {
			inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
			memoryEmailVerificationRateLimiter(c)
		}
	}
}
