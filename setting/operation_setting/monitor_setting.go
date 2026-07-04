package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	// ChannelFailureScopeChannelModel counts consecutive failures separately for
	// each channel+model pair. This is the default and preserves the original
	// behavior.
	ChannelFailureScopeChannelModel = "channel_model"
	// ChannelFailureScopeChannel counts consecutive failures per channel across
	// all models, so a mix of failing models on the same channel accumulates into
	// a single counter.
	ChannelFailureScopeChannel = "channel"
)

type MonitorSetting struct {
	AutoTestChannelEnabled bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes float64 `json:"auto_test_channel_minutes"`

	// ChannelConsecutiveDisableThreshold is the number of consecutive
	// disable-worthy failures on the same channel+model that must accumulate
	// before the channel is automatically disabled. Defaults to 3 so a single
	// transient upstream error never hard-disables a channel.
	ChannelConsecutiveDisableThreshold int `json:"channel_consecutive_disable_threshold"`
	// ChannelFailureWindowMinutes is the rolling window, in minutes, within which
	// consecutive failures are counted. Failures older than this window reset the
	// counter. Defaults to 10 minutes.
	ChannelFailureWindowMinutes float64 `json:"channel_failure_window_minutes"`
	// CountTLSErrorsForDisable controls whether upstream TLS/certificate
	// verification errors count toward the consecutive-failure threshold. These
	// are excluded by default because they are usually caused by the local trust
	// store rather than the upstream channel.
	CountTLSErrorsForDisable bool `json:"count_tls_errors_for_disable"`
	// CountSkipRetryErrorsForDisable controls whether "skip retry" errors (which
	// are typically caused by the client request itself, e.g. a 400) count toward
	// the consecutive-failure threshold. Excluded by default.
	CountSkipRetryErrorsForDisable bool `json:"count_skip_retry_errors_for_disable"`
	// CountModelScopedErrorsForDisable controls whether model-scoped failures
	// (e.g. "model not found" / "not implemented" for a single model) escalate to
	// disabling the whole channel. Excluded by default so only the affected
	// channel+model is impacted.
	CountModelScopedErrorsForDisable bool `json:"count_model_scoped_errors_for_disable"`
	// ChannelFailureScope selects the granularity of the consecutive-failure
	// counter: ChannelFailureScopeChannelModel (default) counts per channel+model,
	// while ChannelFailureScopeChannel merges all models on a channel into a
	// single counter so any mix of failing models can trip the threshold together.
	ChannelFailureScope string `json:"channel_failure_scope"`
	// CountAllRelayErrorsForDisable, when true, makes every relay error count
	// toward the consecutive-failure threshold regardless of the per-error-class
	// toggles above. This is an aggressive policy and is off by default so the
	// existing ShouldDisableChannel semantics are preserved.
	CountAllRelayErrorsForDisable bool `json:"count_all_relay_errors_for_disable"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:             true,
	AutoTestChannelMinutes:             30,
	ChannelConsecutiveDisableThreshold: 3,
	ChannelFailureWindowMinutes:        10,
	CountTLSErrorsForDisable:           false,
	CountSkipRetryErrorsForDisable:     false,
	CountModelScopedErrorsForDisable:   false,
	ChannelFailureScope:                ChannelFailureScopeChannelModel,
	CountAllRelayErrorsForDisable:      false,
}

func init() {
	if v := os.Getenv("CHANNEL_TEST_FREQUENCY"); v != "" {
		frequency, err := strconv.Atoi(v)
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}

	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	return &monitorSetting
}
