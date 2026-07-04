package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var inMemoryRateLimiter common.InMemoryRateLimiter

var redisSlidingWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local ttl_seconds = tonumber(ARGV[3])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
local current = redis.call('ZCARD', key)
if current >= max_requests then
  redis.call('EXPIRE', key, ttl_seconds)
  return 0
end
local member = tostring(now_ms) .. '-' .. tostring(redis.call('INCR', key .. ':seq'))
redis.call('ZADD', key, now_ms, member)
redis.call('EXPIRE', key, ttl_seconds)
redis.call('EXPIRE', key .. ':seq', ttl_seconds)
return 1
`)

var redisSlidingWindowCheckScript = redis.NewScript(`
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local ttl_seconds = tonumber(ARGV[3])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
local current = redis.call('ZCARD', key)
redis.call('EXPIRE', key, ttl_seconds)
if current >= max_requests then
  return 0
end
return 1
`)

var redisSlidingWindowRecordScript = redis.NewScript(`
local key = KEYS[1]
local window_ms = tonumber(ARGV[1])
local ttl_seconds = tonumber(ARGV[2])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
local member = tostring(now_ms) .. '-' .. tostring(redis.call('INCR', key .. ':seq'))
redis.call('ZADD', key, now_ms, member)
redis.call('EXPIRE', key, ttl_seconds)
redis.call('EXPIRE', key .. ':seq', ttl_seconds)
return 1
`)

var defNext = func(c *gin.Context) {
	c.Next()
}

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := "rateLimit:" + mark + c.ClientIP()
	allowed, err := redisSlidingWindowAllow(context.Background(), common.RDB, key, maxRequestNum, duration, common.RateLimitKeyExpirationDuration)
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if !allowed {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func redisSlidingWindowAllow(ctx context.Context, rdb *redis.Client, key string, maxRequestNum int, duration int64, expiration time.Duration) (bool, error) {
	if maxRequestNum <= 0 {
		return true, nil
	}
	if duration <= 0 {
		duration = 1
	}
	ttlSeconds := int64(expiration.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = duration
	}
	windowMs := duration * 1000
	res, err := redisSlidingWindowRateLimitScript.Run(ctx, rdb, []string{key}, maxRequestNum, windowMs, ttlSeconds).Result()
	if err != nil {
		return false, err
	}
	return redisScriptBool(res)
}

func redisSlidingWindowCheck(ctx context.Context, rdb *redis.Client, key string, maxRequestNum int, duration int64, expiration time.Duration) (bool, error) {
	if maxRequestNum <= 0 {
		return true, nil
	}
	if duration <= 0 {
		duration = 1
	}
	ttlSeconds := int64(expiration.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = duration
	}
	windowMs := duration * 1000
	res, err := redisSlidingWindowCheckScript.Run(ctx, rdb, []string{key}, maxRequestNum, windowMs, ttlSeconds).Result()
	if err != nil {
		return false, err
	}
	return redisScriptBool(res)
}

func redisSlidingWindowRecord(ctx context.Context, rdb *redis.Client, key string, duration int64, expiration time.Duration) error {
	if duration <= 0 {
		duration = 1
	}
	ttlSeconds := int64(expiration.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = duration
	}
	windowMs := duration * 1000
	return redisSlidingWindowRecordScript.Run(ctx, rdb, []string{key}, windowMs, ttlSeconds).Err()
}

func redisScriptBool(res interface{}) (bool, error) {
	switch v := res.(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "1", nil
	default:
		allowed, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return err == nil && allowed == 1, err
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		return func(c *gin.Context) {
			memoryRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	if common.GlobalWebRateLimitEnable {
		return rateLimitFactory(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GW")
	}
	return defNext
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if common.GlobalApiRateLimitEnable {
		return rateLimitFactory(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA")
	}
	return defNext
}

func CriticalRateLimit() func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return rateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "CT")
	}
	return defNext
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.DownloadRateLimitNum, common.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.UploadRateLimitNum, common.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			userId := c.GetInt("id")
			if userId == 0 {
				c.Status(http.StatusUnauthorized)
				c.Abort()
				return
			}
			key := fmt.Sprintf("rateLimit:%s:user:%d", mark, userId)
			userRedisRateLimiter(c, maxRequestNum, duration, key)
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("%s:user:%d", mark, userId)
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}
	}
}

// userRedisRateLimiter is like redisRateLimiter but accepts a pre-built key
// (to support user-ID-based keys).
func userRedisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, key string) {
	allowed, err := redisSlidingWindowAllow(context.Background(), common.RDB, key, maxRequestNum, duration, common.RateLimitKeyExpirationDuration)
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if !allowed {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// Configurable via SEARCH_RATE_LIMIT_ENABLE / SEARCH_RATE_LIMIT / SEARCH_RATE_LIMIT_DURATION.
func SearchRateLimit() func(c *gin.Context) {
	if !common.SearchRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(common.SearchRateLimitNum, common.SearchRateLimitDuration, "SR")
}
