package service

import (
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	defaultRouterCooldownThreshold = 1
	defaultRouterCooldownSeconds   = 45
	defaultRouterBackoffBaseMs     = 200
	defaultRouterBackoffMaxMs      = 2000
)

type RouterCooldownState struct {
	Count         int
	LastFailure   time.Time
	CooldownUntil time.Time
}

var routerCooldownTracker = struct {
	sync.Mutex
	items map[string]RouterCooldownState
}{
	items: make(map[string]RouterCooldownState),
}

func RouterCooldownEnabled() bool {
	return common.GetEnvOrDefaultBool("ROUTER_COOLDOWN_ENABLED", true)
}

func RouterRetryBackoffEnabled() bool {
	return common.GetEnvOrDefaultBool("ROUTER_RETRY_BACKOFF_ENABLED", false)
}

func RouterCooldownThreshold() int {
	threshold := common.GetEnvOrDefault("ROUTER_COOLDOWN_THRESHOLD", defaultRouterCooldownThreshold)
	if threshold <= 0 {
		return defaultRouterCooldownThreshold
	}
	return threshold
}

func RouterCooldownTTL() time.Duration {
	seconds := common.GetEnvOrDefault("ROUTER_COOLDOWN_SECONDS", defaultRouterCooldownSeconds)
	if seconds <= 0 {
		return time.Duration(defaultRouterCooldownSeconds) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func RouterBackoffBase() time.Duration {
	return time.Duration(envPositiveInt("ROUTER_RETRY_BACKOFF_BASE_MS", defaultRouterBackoffBaseMs)) * time.Millisecond
}

func RouterBackoffMax() time.Duration {
	return time.Duration(envPositiveInt("ROUTER_RETRY_BACKOFF_MAX_MS", defaultRouterBackoffMaxMs)) * time.Millisecond
}

func ApplyRouterCooldownFilter(info *relaycommon.RelayInfo, param *RetryParam) {
	if !RouterCooldownEnabled() || info == nil || param == nil {
		return
	}
	now := time.Now()
	routerCooldownTracker.Lock()
	defer routerCooldownTracker.Unlock()
	for key, state := range routerCooldownTracker.items {
		if !state.CooldownUntil.IsZero() && now.After(state.CooldownUntil) {
			delete(routerCooldownTracker.items, key)
			continue
		}
		if state.CooldownUntil.IsZero() {
			if !state.LastFailure.IsZero() && now.Sub(state.LastFailure) > RouterCooldownTTL() {
				delete(routerCooldownTracker.items, key)
			}
			continue
		}
		channelID, ok := routerCooldownKeyChannelID(key, info)
		if !ok {
			continue
		}
		ExcludeChannelForRetry(param, channelID)
	}
}

func RecordRouterCooldownFailure(channelID int, info *relaycommon.RelayInfo, err *types.NewAPIError) {
	if !RouterCooldownEnabled() || channelID <= 0 || !ShouldRouterCooldownTrackError(err) {
		return
	}
	key := routerCooldownKey(channelID, info)
	now := time.Now()
	ttl := RouterCooldownTTL()
	if err.RetryHint > 0 {
		ttl = err.RetryHint
	}
	threshold := RouterCooldownThreshold()

	routerCooldownTracker.Lock()
	defer routerCooldownTracker.Unlock()
	state := routerCooldownTracker.items[key]
	if !state.LastFailure.IsZero() && now.Sub(state.LastFailure) > ttl {
		state = RouterCooldownState{}
	}
	state.Count++
	state.LastFailure = now
	if state.Count >= threshold {
		state.CooldownUntil = now.Add(ttl)
	} else {
		state.CooldownUntil = time.Time{}
	}
	routerCooldownTracker.items[key] = state
}

func ClearRouterCooldown(channelID int, info *relaycommon.RelayInfo) {
	if channelID <= 0 {
		return
	}
	routerCooldownTracker.Lock()
	defer routerCooldownTracker.Unlock()
	delete(routerCooldownTracker.items, routerCooldownKey(channelID, info))
}

func ShouldRouterCooldownTrackError(err *types.NewAPIError) bool {
	if err == nil || types.IsSkipRetryError(err) || types.IsClientCanceledError(err) || IsTLSVerificationError(err) {
		return false
	}
	if err.GetErrorCode() == types.ErrorCodeDoRequestFailed ||
		err.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded {
		return true
	}
	switch err.StatusCode {
	case http.StatusTooManyRequests, http.StatusRequestTimeout,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func SleepBeforeRouterRetry(ctx *gin.Context, retryIndex int) {
	if !RouterRetryBackoffEnabled() || retryIndex <= 0 {
		return
	}
	delay := RouterRetryBackoffDelay(retryIndex)
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Request.Context().Done():
	}
}

func RouterRetryBackoffDelay(retryIndex int) time.Duration {
	if retryIndex <= 0 {
		return 0
	}
	base := RouterBackoffBase()
	maxDelay := RouterBackoffMax()
	delay := base
	for i := 1; i < retryIndex; i++ {
		delay *= 2
		if delay >= maxDelay {
			delay = maxDelay
			break
		}
	}
	jitterRange := int64(delay / 2)
	if jitterRange > 0 {
		jitter := rand.Int63n(jitterRange*2+1) - jitterRange
		delay += time.Duration(jitter)
	}
	if delay < 0 {
		delay = 0
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	if common.RelayTimeout > 0 {
		remaining := time.Duration(common.RelayTimeout) * time.Second
		if delay > remaining {
			delay = remaining
		}
	}
	return delay
}

func routerCooldownKey(channelID int, info *relaycommon.RelayInfo) string {
	if info == nil {
		return strconv.Itoa(channelID)
	}
	return strconv.Itoa(channelID) + "|" + info.OriginModelName + "|" + strconv.Itoa(info.RelayMode)
}

func routerCooldownKeyChannelID(key string, info *relaycommon.RelayInfo) (int, bool) {
	prefix := key
	for i, r := range key {
		if r == '|' {
			prefix = key[:i]
			break
		}
	}
	channelID, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, false
	}
	if info == nil {
		return channelID, true
	}
	return channelID, key == routerCooldownKey(channelID, info)
}

func envPositiveInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
