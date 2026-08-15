package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var inMemoryRateLimiter common.InMemoryRateLimiter

const panelRateLimitAppliedKey = "panel_rate_limit_applied"

var panelRedisErrorLogUnix atomic.Int64
var rateLimitRedisErrorLogUnix atomic.Int64

// rateLimitMemberNonce 生成 ZSET member 的唯一后缀。
// 旧实现在 Lua 里用 INCR key..':seq' 做序号，但该 key 未声明在 KEYS 中，
// Redis Cluster 下会直接 CROSSSLOT 报错。改由调用方传入 nonce，脚本只碰 KEYS[1]。
func rateLimitMemberNonce() string {
	return common.GetRandomString(12)
}

var redisSlidingWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local ttl_seconds = tonumber(ARGV[3])
local nonce = ARGV[4]
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
local current = redis.call('ZCARD', key)
if current >= max_requests then
  redis.call('EXPIRE', key, ttl_seconds)
  return 0
end
redis.call('ZADD', key, now_ms, nonce)
redis.call('EXPIRE', key, ttl_seconds)
return 1
`)

var redisWeightedSlidingWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local weight = tonumber(ARGV[2])
local window_ms = tonumber(ARGV[3])
local ttl_seconds = tonumber(ARGV[4])
local nonce = ARGV[5]
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
local current = redis.call('ZCARD', key)
if current + weight > max_requests then
  redis.call('EXPIRE', key, ttl_seconds)
  return 0
end
local prefix = tostring(now_ms) .. '-' .. nonce .. '-'
for i = 1, weight do
  redis.call('ZADD', key, now_ms, prefix .. tostring(i))
end
redis.call('EXPIRE', key, ttl_seconds)
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
local nonce = ARGV[3]
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
redis.call('ZADD', key, now_ms, tostring(now_ms) .. '-' .. nonce)
redis.call('EXPIRE', key, ttl_seconds)
return 1
`)

var defNext = func(c *gin.Context) {
	c.Next()
}

// logRateLimitRedisError 限频输出限流器 Redis 错误，避免 Redis 抖动时刷爆 stdout。
func logRateLimitRedisError(err error) {
	now := time.Now().Unix()
	last := rateLimitRedisErrorLogUnix.Load()
	if now-last < 30 || !rateLimitRedisErrorLogUnix.CompareAndSwap(last, now) {
		return
	}
	common.SysLog(fmt.Sprintf("rate limiter Redis error: %v", err))
}

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := "rateLimit:" + mark + c.ClientIP()
	allowed, err := redisSlidingWindowAllow(c.Request.Context(), common.RDB, key, maxRequestNum, duration, common.RateLimitKeyExpirationDuration)
	if err != nil {
		logRateLimitRedisError(err)
		// 保持失效关闭，但依赖不可用应为 503 而非 500
		c.Status(http.StatusServiceUnavailable)
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
	return redisSlidingWindowReserve(ctx, rdb, key, maxRequestNum, duration, expiration, rateLimitMemberNonce())
}

func redisSlidingWindowReserve(ctx context.Context, rdb *redis.Client, key string, maxRequestNum int, duration int64, expiration time.Duration, reservationID string) (bool, error) {
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
	res, err := redisSlidingWindowRateLimitScript.Run(ctx, rdb, []string{key}, maxRequestNum, windowMs, ttlSeconds, reservationID).Result()
	if err != nil {
		return false, err
	}
	return redisScriptBool(res)
}

func redisSlidingWindowCancel(ctx context.Context, rdb *redis.Client, key, reservationID string) error {
	if key == "" || reservationID == "" {
		return nil
	}
	return rdb.ZRem(ctx, key, reservationID).Err()
}

func redisSlidingWindowRetryAfter(ctx context.Context, rdb *redis.Client, key string, duration int64) (int64, error) {
	if rdb == nil || key == "" {
		return 1, nil
	}
	if duration <= 0 {
		duration = 1
	}
	items, err := rdb.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil {
		return 1, err
	}
	if len(items) == 0 {
		return 1, nil
	}
	nowMs := time.Now().UnixMilli()
	retryMs := duration*1000 - (nowMs - int64(items[0].Score))
	if retryMs <= 0 {
		return 1, nil
	}
	return (retryMs + 999) / 1000, nil
}

func redisWeightedSlidingWindowAllow(ctx context.Context, rdb *redis.Client, key string, maxRequestNum int, duration int64, expiration time.Duration, weight int) (bool, error) {
	if maxRequestNum <= 0 {
		return true, nil
	}
	weight = normalizeRateLimitWeight(maxRequestNum, weight)
	if duration <= 0 {
		duration = 1
	}
	ttlSeconds := int64(expiration.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = duration
	}
	res, err := redisWeightedSlidingWindowRateLimitScript.Run(
		ctx,
		rdb,
		[]string{key},
		maxRequestNum,
		weight,
		duration*1000,
		ttlSeconds,
		rateLimitMemberNonce(),
	).Result()
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
	return redisSlidingWindowRecordScript.Run(ctx, rdb, []string{key}, windowMs, ttlSeconds, rateLimitMemberNonce()).Err()
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

// UserCriticalRateLimit applies the critical-action budget to an authenticated
// user and isolates each action with a stable scope. It must run after
// UserAuth so the user ID is available in the Gin context.
func UserCriticalRateLimit(scope string) func(c *gin.Context) {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "default"
	}
	return userRateLimitFactory(
		common.CriticalRateLimitNum,
		common.CriticalRateLimitDuration,
		"CT:"+scope,
	)
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
	allowed, err := redisSlidingWindowAllow(c.Request.Context(), common.RDB, key, maxRequestNum, duration, common.RateLimitKeyExpirationDuration)
	if err != nil {
		logRateLimitRedisError(err)
		c.Status(http.StatusServiceUnavailable)
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

func PanelRateLimit() func(c *gin.Context) {
	if !common.PanelRateLimitEnable {
		return defNext
	}
	return func(c *gin.Context) {
		if enforcePanelRateLimit(c) {
			c.Next()
		}
	}
}

func enforcePanelRateLimit(c *gin.Context) bool {
	if !common.PanelRateLimitEnable || c.GetBool(panelRateLimitAppliedKey) {
		return true
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return false
	}
	c.Set(panelRateLimitAppliedKey, true)

	class := "write"
	limit := common.PanelWriteRateLimitNum
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		class = "read"
		limit = common.PanelReadRateLimitNum
	}
	weight := panelRequestWeight(c)
	key := fmt.Sprintf("rateLimit:panel:%s:user:%d", class, userID)
	allowed := false
	if common.RedisEnabled && common.RDB != nil {
		var err error
		allowed, err = redisWeightedSlidingWindowAllow(c.Request.Context(), common.RDB, key, limit, common.PanelRateLimitDuration, common.RateLimitKeyExpirationDuration, weight)
		if err != nil {
			logPanelRedisFallback(err)
			allowed = localPanelRateLimit(key, limit, weight)
		}
	} else {
		allowed = localPanelRateLimit(key, limit, weight)
	}
	if !allowed {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return false
	}
	return true
}

func localPanelRateLimit(key string, limit int, weight int) bool {
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return inMemoryRateLimiter.RequestN(key, limit, common.PanelRateLimitDuration, normalizeRateLimitWeight(limit, weight))
}

// normalizeRateLimitWeight prevents a weighted endpoint from becoming
// permanently unavailable when an operator configures a budget below its
// nominal cost. A request consumes the whole available budget in that case.
func normalizeRateLimitWeight(limit, weight int) int {
	if weight <= 0 {
		return 1
	}
	if limit > 0 && weight > limit {
		return limit
	}
	return weight
}

func panelRequestWeight(c *gin.Context) int {
	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	switch {
	case strings.HasPrefix(path, "/api/stats/"):
		return 5
	case strings.HasSuffix(path, "/export"):
		return 5
	case strings.Contains(path, "/capabilities/") && strings.HasSuffix(path, "/probe"):
		return 5
	case strings.Contains(path, "/update_balance"), strings.Contains(path, "/fetch_models"), strings.Contains(path, "/test"):
		return 5
	case strings.Contains(path, "/search"), strings.HasPrefix(path, "/api/data/"), path == "/api/log/stat", strings.HasPrefix(path, "/api/performance/"):
		return 3
	default:
		return 1
	}
}

func logPanelRedisFallback(err error) {
	now := time.Now().Unix()
	last := panelRedisErrorLogUnix.Load()
	if now-last < 30 || !panelRedisErrorLogUnix.CompareAndSwap(last, now) {
		return
	}
	common.SysLog(fmt.Sprintf("panel rate limiter Redis error; using local fallback: %v", err))
}
