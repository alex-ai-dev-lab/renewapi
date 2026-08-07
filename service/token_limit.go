package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const tokenLimitWindow = time.Minute

var (
	tokenLimitMu          sync.Mutex
	tokenLimitCounters    = map[string]*tokenLimitCounter{}
	tokenConcurrencyMu    sync.Mutex
	tokenConcurrencyCount = map[int]int{}
	tokenConcurrencyRedis = map[int]int{}
	tokenRateLimitScript  = redis.NewScript(`
local value = redis.call('INCRBY', KEYS[1], ARGV[1])
if redis.call('TTL', KEYS[1]) < 0 then
  redis.call('EXPIRE', KEYS[1], ARGV[3])
end
if value > tonumber(ARGV[2]) then
  redis.call('DECRBY', KEYS[1], ARGV[1])
  return 0
end
return 1
`)
	tokenConcurrencyAcquireScript = redis.NewScript(`
local value = redis.call('INCR', KEYS[1])
if redis.call('TTL', KEYS[1]) < 0 then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
end
if value > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
return 1
`)
	tokenConcurrencyReleaseScript = redis.NewScript(`
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
if current <= 1 then
  redis.call('UNLINK', KEYS[1])
  return 0
end
return redis.call('DECR', KEYS[1])
`)
)

type tokenLimitCounter struct {
	windowStart time.Time
	value       int
}

func CheckAndRecordTokenRPM(tokenID int, limit int) bool {
	return checkAndRecordTokenLimit(tokenLimitKey("rpm", tokenID), limit, 1)
}

func CheckAndRecordTokenTPM(tokenID int, limit int, tokens int) bool {
	if tokens <= 0 {
		tokens = 1
	}
	return checkAndRecordTokenLimit(tokenLimitKey("tpm", tokenID), limit, tokens)
}

func TryAcquireTokenConcurrency(tokenID int, limit int) bool {
	if tokenID <= 0 || limit <= 0 {
		return true
	}
	if common.RedisEnabled && common.RDB != nil {
		ok, err := tryAcquireTokenConcurrencyRedis(tokenID, limit)
		if err == nil {
			if ok {
				tokenConcurrencyMu.Lock()
				tokenConcurrencyRedis[tokenID]++
				tokenConcurrencyMu.Unlock()
			}
			return ok
		}
		common.SysError(fmt.Sprintf("token concurrency redis failed, fallback to memory: %v", err))
	}
	tokenConcurrencyMu.Lock()
	defer tokenConcurrencyMu.Unlock()
	current := tokenConcurrencyCount[tokenID]
	if current >= limit {
		return false
	}
	tokenConcurrencyCount[tokenID] = current + 1
	return true
}

func ReleaseTokenConcurrency(tokenID int, limit int) {
	if tokenID <= 0 || limit <= 0 {
		return
	}
	if common.RedisEnabled && common.RDB != nil && releaseTokenConcurrencyRedisMarker(tokenID) {
		if err := releaseTokenConcurrencyRedis(tokenID); err != nil {
			common.SysError(fmt.Sprintf("token concurrency redis release failed: %v", err))
		}
		return
	}
	tokenConcurrencyMu.Lock()
	defer tokenConcurrencyMu.Unlock()
	if current := tokenConcurrencyCount[tokenID]; current > 1 {
		tokenConcurrencyCount[tokenID] = current - 1
	} else {
		delete(tokenConcurrencyCount, tokenID)
	}
}

func ResetTokenLimitForTest() {
	tokenLimitMu.Lock()
	tokenLimitCounters = map[string]*tokenLimitCounter{}
	tokenLimitMu.Unlock()
	tokenConcurrencyMu.Lock()
	tokenConcurrencyCount = map[int]int{}
	tokenConcurrencyRedis = map[int]int{}
	tokenConcurrencyMu.Unlock()
}

func checkAndRecordTokenLimit(key string, limit int, amount int) bool {
	if limit <= 0 {
		return true
	}
	if amount > limit {
		return false
	}
	if common.RedisEnabled && common.RDB != nil {
		ok, err := checkAndRecordTokenLimitRedis(key, limit, amount)
		if err == nil {
			return ok
		}
		common.SysError(fmt.Sprintf("token rate limit redis failed, fallback to memory: %v", err))
	}
	return checkAndRecordTokenLimitMemory(key, limit, amount)
}

func checkAndRecordTokenLimitMemory(key string, limit int, amount int) bool {
	now := time.Now()
	tokenLimitMu.Lock()
	defer tokenLimitMu.Unlock()
	counter := tokenLimitCounters[key]
	if counter == nil || now.Sub(counter.windowStart) >= tokenLimitWindow {
		tokenLimitCounters[key] = &tokenLimitCounter{windowStart: now, value: amount}
		return true
	}
	if counter.value+amount > limit {
		return false
	}
	counter.value += amount
	return true
}

func checkAndRecordTokenLimitRedis(key string, limit int, amount int) (bool, error) {
	ctx, cancel := common.RedisOperationContext(context.Background())
	defer cancel()
	windowKey := fmt.Sprintf("token_limit:%s:%d", key, time.Now().Unix()/60)
	allowed, err := tokenRateLimitScript.Run(
		ctx,
		common.RDB,
		[]string{windowKey},
		amount,
		limit,
		int((tokenLimitWindow+5*time.Second)/time.Second),
	).Int()
	return allowed == 1, err
}

func tryAcquireTokenConcurrencyRedis(tokenID int, limit int) (bool, error) {
	ctx, cancel := common.RedisOperationContext(context.Background())
	defer cancel()
	key := tokenConcurrencyKey(tokenID)
	acquired, err := tokenConcurrencyAcquireScript.Run(
		ctx,
		common.RDB,
		[]string{key},
		limit,
		int((2*time.Minute)/time.Second),
	).Int()
	return acquired == 1, err
}

func releaseTokenConcurrencyRedis(tokenID int) error {
	ctx, cancel := common.RedisOperationContext(context.Background())
	defer cancel()
	key := tokenConcurrencyKey(tokenID)
	return tokenConcurrencyReleaseScript.Run(ctx, common.RDB, []string{key}).Err()
}

func releaseTokenConcurrencyRedisMarker(tokenID int) bool {
	tokenConcurrencyMu.Lock()
	defer tokenConcurrencyMu.Unlock()
	current := tokenConcurrencyRedis[tokenID]
	if current <= 0 {
		return false
	}
	if current == 1 {
		delete(tokenConcurrencyRedis, tokenID)
	} else {
		tokenConcurrencyRedis[tokenID] = current - 1
	}
	return true
}

func tokenLimitKey(kind string, tokenID int) string {
	return fmt.Sprintf("%s:%d", kind, tokenID)
}

func tokenConcurrencyKey(tokenID int) string {
	return fmt.Sprintf("token_limit:concurrency:%d", tokenID)
}
