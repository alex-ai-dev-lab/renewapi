package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

type modelRateLimitReservation struct {
	id              string
	totalReserved   bool
	successReserved bool
}

func modelRateLimitKeys(userID int, redis bool) (string, string) {
	user := strconv.Itoa(userID)
	if redis {
		return fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitCountMark, user),
			fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, user)
	}
	return ModelRequestRateLimitCountMark + user, ModelRequestRateLimitSuccessCountMark + user
}

func reserveModelRateLimit(ctx context.Context, rdb *redis.Client, useRedis bool, key string, maxCount int, duration int64, reservationID string) (bool, error) {
	if maxCount <= 0 {
		return true, nil
	}
	if useRedis {
		return redisSlidingWindowReserve(ctx, rdb, key, maxCount, duration, time.Duration(duration)*time.Second, reservationID)
	}
	return inMemoryRateLimiter.Reserve(key, maxCount, duration, reservationID), nil
}

func cancelModelRateLimit(ctx context.Context, rdb *redis.Client, useRedis bool, key, reservationID string) {
	if useRedis {
		cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if err := redisSlidingWindowCancel(cancelCtx, rdb, key, reservationID); err != nil {
			logRateLimitRedisError(err)
		}
		return
	}
	inMemoryRateLimiter.Cancel(key, reservationID)
}

func retryAfterModelRateLimit(ctx context.Context, rdb *redis.Client, useRedis bool, key string, duration int64) int64 {
	if useRedis {
		retryAfter, err := redisSlidingWindowRetryAfter(ctx, rdb, key, duration)
		if err != nil {
			logRateLimitRedisError(err)
			return duration
		}
		return retryAfter
	}
	return inMemoryRateLimiter.RetryAfter(key, duration)
}

func logModelRateLimitReject(c *gin.Context, bucket string, limit int, retryAfter int64) {
	common.SysLog(fmt.Sprintf(
		"model_rate_limit_reject bucket=%s user_id=%d request_id=%s limit=%d retry_after=%d",
		bucket,
		c.GetInt("id"),
		c.GetString(common.RequestIdKey),
		limit,
		retryAfter,
	))
}

func logModelRateLimitCancel(c *gin.Context, bucket string, outcome service.ModelRouteOutcome) {
	common.SysLog(fmt.Sprintf(
		"model_rate_limit_cancel bucket=%s outcome=%s user_id=%d request_id=%s",
		bucket,
		outcome,
		c.GetInt("id"),
		c.GetString(common.RequestIdKey),
	))
}

func abortModelRateLimit(c *gin.Context, bucket, message string, retryAfter int64, limit int) {
	if retryAfter <= 0 {
		retryAfter = 1
	}
	logModelRateLimitReject(c, bucket, limit, retryAfter)
	if limit > 0 {
		c.Header("X-RateLimit-Limit-Requests", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining-Requests", "0")
		c.Header("X-RateLimit-Reset-Requests", fmt.Sprintf("%ds", retryAfter))
	}
	c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	abortWithOpenAiMessage(c, http.StatusTooManyRequests, message, types.ErrorCodeRateLimitExceeded)
}

func modelRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, useRedis bool) gin.HandlerFunc {
	if duration <= 0 {
		duration = 1
	}
	if !useRedis {
		inMemoryRateLimiter.Init(time.Duration(duration) * time.Second)
	}
	return func(c *gin.Context) {
		service.InitModelRouteOutcome(c)
		ctx := c.Request.Context()
		rdb := common.RDB
		if useRedis && rdb == nil {
			logRateLimitRedisError(fmt.Errorf("model rate limiter Redis client is nil"))
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "rate_limit_unavailable")
			return
		}

		totalKey, successKey := modelRateLimitKeys(c.GetInt("id"), useRedis)
		reservation := modelRateLimitReservation{
			id: common.GetRandomString(24),
		}
		allowed, err := reserveModelRateLimit(ctx, rdb, useRedis, totalKey, totalMaxCount, duration, reservation.id)
		if err != nil {
			logRateLimitRedisError(err)
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "rate_limit_check_failed")
			return
		}
		if !allowed {
			retryAfter := retryAfterModelRateLimit(ctx, rdb, useRedis, totalKey, duration)
			abortModelRateLimit(c, "total", fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount), retryAfter, totalMaxCount)
			return
		}
		reservation.totalReserved = totalMaxCount > 0

		allowed, err = reserveModelRateLimit(ctx, rdb, useRedis, successKey, successMaxCount, duration, reservation.id)
		if err != nil || !allowed {
			if reservation.totalReserved {
				cancelModelRateLimit(ctx, rdb, useRedis, totalKey, reservation.id)
			}
			if err != nil {
				logRateLimitRedisError(err)
				abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "rate_limit_check_failed")
				return
			}
			retryAfter := retryAfterModelRateLimit(ctx, rdb, useRedis, successKey, duration)
			abortModelRateLimit(c, "success", fmt.Sprintf("您已达到成功请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount), retryAfter, successMaxCount)
			return
		}
		reservation.successReserved = successMaxCount > 0

		completed := false
		defer func() {
			outcome := service.FinalModelRouteOutcome(c, completed)
			if reservation.successReserved && outcome != service.ModelRouteOutcomeSuccess {
				cancelModelRateLimit(ctx, rdb, useRedis, successKey, reservation.id)
				logModelRateLimitCancel(c, "success", outcome)
			}
			if reservation.totalReserved && service.ModelRouteOutcomeRefundsTotal(outcome) {
				cancelModelRateLimit(ctx, rdb, useRedis, totalKey, reservation.id)
				logModelRateLimitCancel(c, "total", outcome)
			}
		}()

		c.Next()
		completed = true
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算限流参数
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := setting.ModelRequestRateLimitCount
		successMaxCount := setting.ModelRequestRateLimitSuccessCount

		// 获取分组
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}

		//获取分组的限流配置
		groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group)
		if found {
			totalMaxCount = groupTotalCount
			successMaxCount = groupSuccessCount
		}

		modelRateLimitHandler(duration, totalMaxCount, successMaxCount, common.RedisEnabled)(c)
	}
}
