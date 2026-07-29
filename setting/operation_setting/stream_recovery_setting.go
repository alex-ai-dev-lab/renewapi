package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type StreamRecoverySetting struct {
	// Enabled is the legacy pre-commit retry switch. It intentionally does not
	// disable next-request route repair when false.
	Enabled                          bool `json:"enabled"`
	PreCommitRetryEnabled            bool `json:"pre_commit_retry_enabled"`
	EmptyStreamRetryLimit            int  `json:"empty_stream_retry_limit"`
	SessionRouteRepairEnabled        bool `json:"session_route_repair_enabled"`
	PostCommitRouteRepairEnabled     bool `json:"post_commit_route_repair_enabled"`
	SessionNegativeTTLSeconds        int  `json:"session_negative_ttl_seconds"`
	UnknownFailureNegativeTTLSeconds int  `json:"unknown_failure_negative_ttl_seconds"`
	KeyNegativeTTLSeconds            int  `json:"key_negative_ttl_seconds"`
	MaxCrossRequestRouteChanges      int  `json:"max_cross_request_route_changes"`
	RecoveryChainWindowSeconds       int  `json:"recovery_chain_window_seconds"`
	ChannelModelEscalationEnabled    bool `json:"channel_model_escalation_enabled"`
	DistinctSessionThreshold         int  `json:"distinct_session_threshold"`
	EvidenceWindowSeconds            int  `json:"evidence_window_seconds"`

	preCommitRetryConfigured bool
}

var streamRecoverySetting = StreamRecoverySetting{
	Enabled:                          false,
	PreCommitRetryEnabled:            false,
	EmptyStreamRetryLimit:            1,
	SessionRouteRepairEnabled:        true,
	PostCommitRouteRepairEnabled:     true,
	SessionNegativeTTLSeconds:        90,
	UnknownFailureNegativeTTLSeconds: 30,
	KeyNegativeTTLSeconds:            90,
	MaxCrossRequestRouteChanges:      2,
	RecoveryChainWindowSeconds:       60,
	ChannelModelEscalationEnabled:    false,
	DistinctSessionThreshold:         3,
	EvidenceWindowSeconds:            60,
}

func init() {
	config.GlobalConfig.Register("stream_recovery_setting", &streamRecoverySetting)
}

func GetStreamRecoverySetting() *StreamRecoverySetting {
	return &streamRecoverySetting
}

func (s *StreamRecoverySetting) PreCommitRetryOn() bool {
	if s == nil {
		return false
	}
	if s.preCommitRetryConfigured {
		return s.PreCommitRetryEnabled
	}
	return s.PreCommitRetryEnabled || s.Enabled
}

func (s *StreamRecoverySetting) MarkExplicitConfigFields(fields map[string]string) {
	if s == nil {
		return
	}
	_, s.preCommitRetryConfigured = fields["pre_commit_retry_enabled"]
	if !s.preCommitRetryConfigured {
		s.PreCommitRetryEnabled = false
	}
}
