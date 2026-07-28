package dto

import (
	"fmt"
	"strings"
)

type ResponsesFunctionCallArgumentsFormat string

const (
	ResponsesFunctionCallArgumentsFormatAuto   ResponsesFunctionCallArgumentsFormat = "auto"
	ResponsesFunctionCallArgumentsFormatString ResponsesFunctionCallArgumentsFormat = "string"
	ResponsesFunctionCallArgumentsFormatObject ResponsesFunctionCallArgumentsFormat = "object"
)

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	TLSInsecureSkipVerify  bool   `json:"tls_insecure_skip_verify,omitempty"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
	// HTTPProtocol controls outbound negotiation: auto (default) or http1.
	HTTPProtocol string `json:"http_protocol,omitempty"`
	// HTTP2ConnectionShards spreads an origin across 1-8 independent pools.
	HTTP2ConnectionShards int `json:"http2_connection_shards,omitempty"`

	// User-Agent settings
	UserAgentID       *int   `json:"user_agent_id,omitempty"`       // UA from user_agents table
	UserAgentOverride string `json:"user_agent_override,omitempty"` // Custom UA string

	// Upstream error normalization. When nil, the global setting decides.
	NormalizeUpstreamErrors *bool `json:"normalize_upstream_errors,omitempty"`

	// Codex client identity support. When nil, infer from the upstream URL.
	RequiresCodexIdentity *bool `json:"requires_codex_identity,omitempty"`

	// Allows global model defaults to override the upstream protocol/adaptor for
	// this channel. Off by default so native-protocol channels keep their own
	// authentication and request format unless the admin opts in.
	AllowModelProtocolOverride bool `json:"allow_model_protocol_override,omitempty"`
	// Restricts global model protocol overrides to explicitly declared upstream
	// endpoint families. An empty list is fail-closed.
	ModelProtocolOverrideTargets []string `json:"model_protocol_override_targets,omitempty"`

	// Controls the upstream OpenAI Responses function_call.arguments shape.
	// auto keeps the official string format, except Codex and runtime fallback
	// retries that require a JSON object.
	ResponsesFunctionCallArgumentsFormat ResponsesFunctionCallArgumentsFormat `json:"responses_function_call_arguments_format,omitempty"`

	// Auto-test settings
	AutoTestInterval        int    `json:"auto_test_interval,omitempty"`          // minutes, 0 = use global
	AutoTestRetryCount      int    `json:"auto_test_retry_count,omitempty"`       // retry attempts per test
	AutoTestRetryThreshold  int    `json:"auto_test_retry_threshold,omitempty"`   // disable after N failures
	AutoTestTimeWindowStart string `json:"auto_test_time_window_start,omitempty"` // "HH:MM"
	AutoTestTimeWindowEnd   string `json:"auto_test_time_window_end,omitempty"`   // "HH:MM"
	AutoTestTimezone        string `json:"auto_test_timezone,omitempty"`          // IANA timezone; empty = local

	// Claude thinking passthrough support. When nil, infer from channel type.
	SupportsClaudeThinking *bool `json:"supports_claude_thinking,omitempty"`

	// Anti-poison guard settings. Nil fields inherit global settings.
	AntiPoisonProfile                  string `json:"anti_poison_profile,omitempty"`
	AntiPoisonEnabled                  *bool  `json:"anti_poison_enabled,omitempty"`
	AntiPoisonAnswerEnvelope           string `json:"anti_poison_answer_envelope,omitempty"`
	AntiPoisonResponseProof            string `json:"anti_poison_response_proof,omitempty"`
	AntiPoisonResponseProofEnabled     *bool  `json:"anti_poison_response_proof_enabled,omitempty"`
	AntiPoisonToolCallGuard            string `json:"anti_poison_tool_call_guard,omitempty"`
	AntiPoisonToolCallGuardStrict      *bool  `json:"anti_poison_tool_call_guard_strict,omitempty"`
	AntiPoisonOpaqueScan               string `json:"anti_poison_opaque_scan,omitempty"`
	AntiPoisonProbeBeforeEveryRequest  *bool  `json:"anti_poison_probe_before_every_request,omitempty"`
	AntiPoisonStreamMode               string `json:"anti_poison_stream_mode,omitempty"`
	AntiPoisonHardFailuresToQuarantine int    `json:"anti_poison_hard_failures_to_quarantine,omitempty"`
	AntiPoisonSoftFailuresToDegrade    int    `json:"anti_poison_soft_failures_to_degrade,omitempty"`
	AntiPoisonFailureMode              string `json:"anti_poison_failure_mode,omitempty"`
	// Canary echo: server-injected nonce in the last user message that the
	// model must echo at the end of its reply. Drops 200-OK ad payloads that
	// can not see per-request user content. Nil = inherit global; default off.
	AntiPoisonCanaryEchoEnabled *bool `json:"anti_poison_canary_echo_enabled,omitempty"`
	// Shape check: validate response id/model/object/finish_reason against the
	// protocol's known fingerprint. Nil = inherit global; default off.
	AntiPoisonShapeCheckEnabled *bool `json:"anti_poison_shape_check_enabled,omitempty"`
}

const (
	HTTPProtocolAuto         = "auto"
	HTTPProtocolHTTP1        = "http1"
	MaxHTTP2ConnectionShards = 8
)

func (s *ChannelSettings) ValidateHTTPTransport() error {
	if s == nil {
		return nil
	}
	protocol := strings.ToLower(strings.TrimSpace(s.HTTPProtocol))
	switch protocol {
	case "", HTTPProtocolAuto, HTTPProtocolHTTP1:
	default:
		return fmt.Errorf("invalid http_protocol: %s", s.HTTPProtocol)
	}
	if s.HTTP2ConnectionShards < 0 || s.HTTP2ConnectionShards > MaxHTTP2ConnectionShards {
		return fmt.Errorf("invalid http2_connection_shards: %d", s.HTTP2ConnectionShards)
	}
	if protocol == HTTPProtocolHTTP1 && s.HTTP2ConnectionShards > 1 {
		return fmt.Errorf("http2_connection_shards must be 1 when http_protocol is http1")
	}
	return nil
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string        `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool         `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool          `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool          `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool          `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool          `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool          `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool          `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool          `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType    `json:"aws_key_type,omitempty"`
	AutoTestAndRecoverDisabled            bool          `json:"auto_test_and_recover_disabled,omitempty"`             // 是否禁止该渠道被定时自动测试和自动恢复
	UpstreamModelUpdateCheckEnabled       bool          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	MockStatusCode                        int           `json:"mock_status_code,omitempty"`                           // Mock 渠道返回状态码，默认 200
	MockContent                           string        `json:"mock_content,omitempty"`                               // Mock 渠道响应文本
	MockLatencyMs                         int           `json:"mock_latency_ms,omitempty"`                            // Mock 渠道人为延迟，最大 5000ms
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
