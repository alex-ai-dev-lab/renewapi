package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type StreamRecoverySetting struct {
	Enabled                       bool `json:"enabled"`
	EmptyStreamRetryLimit         int  `json:"empty_stream_retry_limit"`
	SessionNegativeTTLSeconds     int  `json:"session_negative_ttl_seconds"`
	ChannelModelEscalationEnabled bool `json:"channel_model_escalation_enabled"`
	DistinctSessionThreshold      int  `json:"distinct_session_threshold"`
	EvidenceWindowSeconds         int  `json:"evidence_window_seconds"`
}

var streamRecoverySetting = StreamRecoverySetting{
	Enabled:                       false,
	EmptyStreamRetryLimit:         1,
	SessionNegativeTTLSeconds:     90,
	ChannelModelEscalationEnabled: false,
	DistinctSessionThreshold:      3,
	EvidenceWindowSeconds:         60,
}

func init() {
	config.GlobalConfig.Register("stream_recovery_setting", &streamRecoverySetting)
}

func GetStreamRecoverySetting() *StreamRecoverySetting {
	return &streamRecoverySetting
}
