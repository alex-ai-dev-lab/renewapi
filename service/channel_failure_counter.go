package service

import (
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ChannelConsecutiveDisableThreshold is the default number of consecutive
// failures on the same channel+model that must accumulate before the channel is
// auto-disabled. It is used as a fallback when the operator has not configured
// monitor_setting.channel_consecutive_disable_threshold. Unified to 3 so that a
// single transient upstream error no longer hard-disables a channel.
const ChannelConsecutiveDisableThreshold = 3

// defaultChannelConsecutiveFailureTTL is the default rolling window within which
// consecutive failures accumulate. Used as a fallback when the operator has not
// configured monitor_setting.channel_failure_window_minutes.
const defaultChannelConsecutiveFailureTTL = 10 * time.Minute

// GetChannelConsecutiveDisableThreshold returns the operator-configured
// consecutive-failure threshold, falling back to
// ChannelConsecutiveDisableThreshold when it is unset or invalid.
func GetChannelConsecutiveDisableThreshold() int {
	if v := operation_setting.GetMonitorSetting().ChannelConsecutiveDisableThreshold; v > 0 {
		return v
	}
	return ChannelConsecutiveDisableThreshold
}

// getChannelConsecutiveFailureTTL returns the operator-configured failure
// window, falling back to defaultChannelConsecutiveFailureTTL when it is unset
// or invalid.
func getChannelConsecutiveFailureTTL() time.Duration {
	if m := operation_setting.GetMonitorSetting().ChannelFailureWindowMinutes; m > 0 {
		return time.Duration(m * float64(time.Minute))
	}
	return defaultChannelConsecutiveFailureTTL
}

type channelConsecutiveFailureState struct {
	count       int
	lastFailure time.Time
}

var channelConsecutiveFailureTracker = struct {
	sync.Mutex
	items map[string]channelConsecutiveFailureState
}{
	items: make(map[string]channelConsecutiveFailureState),
}

// channelConsecutiveFailureKey builds the tracker key for a channel+model. When
// the operator selects ChannelFailureScopeChannel, all models on a channel share
// a single counter so that any mix of failing models can trip the disable
// threshold together; the default ChannelFailureScopeChannelModel keeps counters
// isolated per model. Record/Peek/Clear all go through this helper, so failure
// counting and success clearing always use the same scope.
func channelConsecutiveFailureKey(channelID int, model string) string {
	if operation_setting.GetMonitorSetting().ChannelFailureScope == operation_setting.ChannelFailureScopeChannel {
		return strconv.Itoa(channelID)
	}
	return strconv.Itoa(channelID) + "|" + model
}

// RecordChannelConsecutiveFailure increments and returns the consecutive failure
// count for the given channel+model. Counts older than the TTL are reset first so
// that intermittent, widely-spaced failures do not accumulate forever.
func RecordChannelConsecutiveFailure(channelID int, model string) int {
	if channelID <= 0 {
		return 0
	}
	now := time.Now()
	key := channelConsecutiveFailureKey(channelID, model)
	ttl := getChannelConsecutiveFailureTTL()

	channelConsecutiveFailureTracker.Lock()
	defer channelConsecutiveFailureTracker.Unlock()

	state := channelConsecutiveFailureTracker.items[key]
	if !state.lastFailure.IsZero() && now.Sub(state.lastFailure) > ttl {
		state.count = 0
	}
	state.count++
	state.lastFailure = now
	channelConsecutiveFailureTracker.items[key] = state
	return state.count
}

// PeekChannelConsecutiveFailure returns the current consecutive failure count
// without mutating it. Counts older than the TTL are treated as zero.
func PeekChannelConsecutiveFailure(channelID int, model string) int {
	if channelID <= 0 {
		return 0
	}
	ttl := getChannelConsecutiveFailureTTL()
	channelConsecutiveFailureTracker.Lock()
	defer channelConsecutiveFailureTracker.Unlock()
	state := channelConsecutiveFailureTracker.items[channelConsecutiveFailureKey(channelID, model)]
	if !state.lastFailure.IsZero() && time.Since(state.lastFailure) > ttl {
		return 0
	}
	return state.count
}

// ClearChannelConsecutiveFailure resets the consecutive failure counter for the
// given channel+model, typically after a successful request.
func ClearChannelConsecutiveFailure(channelID int, model string) {
	if channelID <= 0 {
		return
	}
	channelConsecutiveFailureTracker.Lock()
	defer channelConsecutiveFailureTracker.Unlock()
	delete(channelConsecutiveFailureTracker.items, channelConsecutiveFailureKey(channelID, model))
}
