package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const (
	tokenLimitWindow              = time.Minute
	tokenConcurrencyLeaseDuration = 2 * time.Minute
	tokenConcurrencyRenewInterval = 30 * time.Second
)

var (
	tokenLimitMu          sync.Mutex
	tokenLimitCounters    = map[string]*tokenLimitCounter{}
	tokenConcurrencyMu    sync.Mutex
	tokenConcurrencyCount = map[int]int{}
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
local clock = redis.call('TIME')
local now = clock[1] * 1000 + math.floor(clock[2] / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[1]) then
  return 0
end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[2]), ARGV[3])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) * 2)
return 1
`)
	tokenConcurrencyReleaseScript = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
if redis.call('ZCARD', KEYS[1]) == 0 then
  redis.call('UNLINK', KEYS[1])
end
return 1
`)
	tokenConcurrencyRenewScript = redis.NewScript(`
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) then
  return 0
end
local clock = redis.call('TIME')
local now = clock[1] * 1000 + math.floor(clock[2] / 1000)
redis.call('ZADD', KEYS[1], 'XX', now + tonumber(ARGV[2]), ARGV[1])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) * 2)
return 1
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

// TokenConcurrencySlot owns one request's slot and its cancellation context.
// Release is idempotent and always uses the backend that granted this slot.
type TokenConcurrencySlot struct {
	Context context.Context
	release func()
	client  *redis.Client
	key, id string
}

func (s *TokenConcurrencySlot) Release() { s.release() }

func AcquireTokenConcurrency(ctx context.Context, tokenID int, limit int) (*TokenConcurrencySlot, bool) {
	if tokenID <= 0 || limit <= 0 {
		return &TokenConcurrencySlot{Context: ctx, release: func() {}}, true
	}
	if client := common.RDB; common.RedisEnabled && client != nil {
		slot := &TokenConcurrencySlot{client: client, key: tokenConcurrencyKey(tokenID), id: common.GetUUID()}
		opCtx, cancel := common.RedisOperationContext(ctx)
		acquired, err := tokenConcurrencyAcquireScript.Run(opCtx, client, []string{slot.key}, limit, tokenConcurrencyLeaseDuration.Milliseconds(), slot.id).Int()
		cancel()
		if err == nil {
			if acquired != 1 {
				return nil, false
			}
			slot.startRenewal(ctx, tokenConcurrencyRenewInterval)
			return slot, true
		}
		common.SysError(fmt.Sprintf("token concurrency redis failed, fallback to memory: %v", err))
	}
	tokenConcurrencyMu.Lock()
	defer tokenConcurrencyMu.Unlock()
	current := tokenConcurrencyCount[tokenID]
	if current >= limit {
		return nil, false
	}
	tokenConcurrencyCount[tokenID] = current + 1
	return &TokenConcurrencySlot{Context: ctx, release: sync.OnceFunc(func() { releaseTokenConcurrencyMemory(tokenID) })}, true
}

func releaseTokenConcurrencyMemory(tokenID int) {
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

func (s *TokenConcurrencySlot) renew(ctx context.Context) (bool, error) {
	ctx, stop := context.WithTimeout(ctx, tokenConcurrencyRenewInterval/2)
	defer stop()
	ctx, cancel := common.RedisOperationContext(ctx)
	defer cancel()
	renewed, err := tokenConcurrencyRenewScript.Run(ctx, s.client, []string{s.key}, s.id, tokenConcurrencyLeaseDuration.Milliseconds()).Int()
	return renewed == 1, err
}

func (s *TokenConcurrencySlot) startRenewal(parent context.Context, interval time.Duration) {
	ctx, cancel := context.WithCancel(parent)
	s.Context = ctx
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		lastRenewed := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewed, err := s.renew(ctx)
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					common.SysError(fmt.Sprintf("token concurrency renewal failed: %v", err))
					// Stop before the lease expires if Redis remains unavailable.
					if time.Since(lastRenewed) < tokenConcurrencyLeaseDuration-tokenConcurrencyRenewInterval {
						continue
					}
				} else if renewed {
					lastRenewed = time.Now()
					continue
				}
				cancel()
				return
			}
		}
	}()
	s.release = sync.OnceFunc(func() {
		cancel()
		<-done
		opCtx, stop := common.RedisOperationContext(context.Background())
		defer stop()
		if err := tokenConcurrencyReleaseScript.Run(opCtx, s.client, []string{s.key}, s.id).Err(); err != nil {
			common.SysError(fmt.Sprintf("token concurrency redis release failed: %v", err))
		}
	})
}

func tokenLimitKey(kind string, tokenID int) string {
	return fmt.Sprintf("%s:%d", kind, tokenID)
}

func tokenConcurrencyKey(tokenID int) string {
	return fmt.Sprintf("token_limit:concurrency:v2:%d", tokenID)
}
