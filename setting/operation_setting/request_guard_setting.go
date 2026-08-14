package operation_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	RequestGuardModeOff     = "off"
	RequestGuardModeObserve = "observe"
	RequestGuardModeEnforce = "enforce"

	RequestGuardFailureClosed = "closed"
	RequestGuardFailureOpen   = "open"

	RequestGuardInputFullClientControlled = "full_client_controlled"

	RequestGuardCodecQwen3Guard = "qwen3guard"
	RequestGuardCodecJSONPolicy = "json_policy"

	RequestGuardProxyDisabled    = "disabled"
	RequestGuardProxyEnvironment = "environment"
	RequestGuardProxyExplicit    = "explicit"
)

type RequestGuardScope struct {
	AllGroups bool     `json:"all_groups"`
	Groups    []string `json:"groups"`
	Models    []string `json:"models"`
	Protocols []string `json:"protocols"`
}

type RequestGuardBulkhead struct {
	MaxConcurrent  int `json:"max_concurrent"`
	MaxPerEndpoint int `json:"max_per_endpoint"`
}

type RequestGuardObserve struct {
	WorkerCount   int `json:"worker_count"`
	QueueCapacity int `json:"queue_capacity"`
}

type RequestGuardEndpoint struct {
	ID              string `json:"id"`
	Enabled         bool   `json:"enabled"`
	Priority        int    `json:"priority"`
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	Codec           string `json:"codec"`
	TimeoutMs       int    `json:"timeout_ms"`
	InputLimitRunes int    `json:"input_limit_runes"`
	AllowPrivateIP  bool   `json:"allow_private_ip"`
	ProxyPolicy     string `json:"proxy_policy"`
	ProxyURL        string `json:"proxy_url,omitempty"`
}

type RequestGuardSetting struct {
	Enabled              bool                   `json:"enabled"`
	Mode                 string                 `json:"mode"`
	FailurePolicy        string                 `json:"failure_policy"`
	InputMode            string                 `json:"input_mode"`
	MaxInputRunes        int                    `json:"max_input_runes"`
	EvaluationTimeoutMs  int                    `json:"evaluation_timeout_ms"`
	Scope                RequestGuardScope      `json:"scope"`
	Bulkhead             RequestGuardBulkhead   `json:"bulkhead"`
	Observe              RequestGuardObserve    `json:"observe"`
	StorePassEvents      bool                   `json:"store_pass_events"`
	StoreRedactedPreview bool                   `json:"store_redacted_preview"`
	Endpoints            []RequestGuardEndpoint `json:"endpoints"`
}

var requestGuardSetting = RequestGuardSetting{
	Enabled:             false,
	Mode:                RequestGuardModeOff,
	FailurePolicy:       RequestGuardFailureClosed,
	InputMode:           RequestGuardInputFullClientControlled,
	MaxInputRunes:       16_000,
	EvaluationTimeoutMs: 2_500,
	Scope: RequestGuardScope{
		AllGroups: false,
		Groups:    []string{},
		Models:    []string{"*"},
		Protocols: []string{"openai_chat", "openai_responses", "anthropic", "gemini"},
	},
	Bulkhead:  RequestGuardBulkhead{MaxConcurrent: 64, MaxPerEndpoint: 16},
	Observe:   RequestGuardObserve{WorkerCount: 4, QueueCapacity: 4096},
	Endpoints: []RequestGuardEndpoint{},
}

var requestGuardSnapshot atomic.Pointer[RequestGuardSetting]

func init() {
	config.GlobalConfig.Register("request_guard_setting", &requestGuardSetting)
	ApplyRequestGuardSetting(requestGuardSetting)
}

func (s *RequestGuardSetting) MarkExplicitConfigFields(_ map[string]string) {
	if s == nil {
		return
	}
	ApplyRequestGuardSetting(*s)
}

func GetRequestGuardSetting() RequestGuardSetting {
	current := requestGuardSnapshot.Load()
	if current == nil {
		return cloneRequestGuardSetting(requestGuardSetting)
	}
	return cloneRequestGuardSetting(*current)
}

func GetRequestGuardSnapshot() *RequestGuardSetting {
	return requestGuardSnapshot.Load()
}

func ApplyRequestGuardSetting(next RequestGuardSetting) {
	normalized := normalizeRequestGuardSetting(next)
	requestGuardSetting = cloneRequestGuardSetting(normalized)
	immutable := cloneRequestGuardSetting(normalized)
	requestGuardSnapshot.Store(&immutable)
}

func ValidateRequestGuardSetting(setting RequestGuardSetting) error {
	setting = normalizeRequestGuardSetting(setting)
	if !isOneOf(setting.Mode, RequestGuardModeOff, RequestGuardModeObserve, RequestGuardModeEnforce) {
		return fmt.Errorf("mode must be off, observe, or enforce")
	}
	if !isOneOf(setting.FailurePolicy, RequestGuardFailureClosed, RequestGuardFailureOpen) {
		return fmt.Errorf("failure_policy must be closed or open")
	}
	if setting.InputMode != RequestGuardInputFullClientControlled {
		return fmt.Errorf("input_mode must be full_client_controlled in RequestGuard V1")
	}
	if setting.MaxInputRunes < 128 || setting.MaxInputRunes > 100_000 {
		return fmt.Errorf("max_input_runes must be between 128 and 100000")
	}
	if setting.EvaluationTimeoutMs < 100 || setting.EvaluationTimeoutMs > 30_000 {
		return fmt.Errorf("evaluation_timeout_ms must be between 100 and 30000")
	}
	if setting.Bulkhead.MaxConcurrent < 1 || setting.Bulkhead.MaxConcurrent > 1024 {
		return fmt.Errorf("bulkhead.max_concurrent must be between 1 and 1024")
	}
	if setting.Bulkhead.MaxPerEndpoint < 1 || setting.Bulkhead.MaxPerEndpoint > setting.Bulkhead.MaxConcurrent {
		return fmt.Errorf("bulkhead.max_per_endpoint must be between 1 and max_concurrent")
	}
	if setting.Observe.WorkerCount < 1 || setting.Observe.WorkerCount > 32 {
		return fmt.Errorf("observe.worker_count must be between 1 and 32")
	}
	if setting.Observe.QueueCapacity < 1 || setting.Observe.QueueCapacity > 65_536 {
		return fmt.Errorf("observe.queue_capacity must be between 1 and 65536")
	}

	knownProtocols := map[string]bool{
		"openai_chat": true, "openai_responses": true, "anthropic": true,
		"gemini": true, "image": true, "audio": true, "embedding": true, "rerank": true,
	}
	for _, protocol := range setting.Scope.Protocols {
		if !knownProtocols[protocol] {
			return fmt.Errorf("unsupported scope protocol %q", protocol)
		}
	}

	seen := make(map[string]struct{}, len(setting.Endpoints))
	enabledCount := 0
	for i, endpoint := range setting.Endpoints {
		if !requestGuardEndpointIDPattern.MatchString(endpoint.ID) {
			return fmt.Errorf("endpoints[%d].id must match %s", i, requestGuardEndpointIDPattern.String())
		}
		if _, ok := seen[endpoint.ID]; ok {
			return fmt.Errorf("duplicate endpoint id %q", endpoint.ID)
		}
		seen[endpoint.ID] = struct{}{}
		if endpoint.Enabled {
			enabledCount++
		}
		parsed, err := url.Parse(endpoint.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("endpoints[%d].base_url must be an absolute http or https URL", i)
		}
		if parsed.User != nil {
			return fmt.Errorf("endpoints[%d].base_url must not contain credentials", i)
		}
		if strings.TrimSpace(endpoint.Model) == "" {
			return fmt.Errorf("endpoints[%d].model is required", i)
		}
		if !isOneOf(endpoint.Codec, RequestGuardCodecQwen3Guard, RequestGuardCodecJSONPolicy) {
			return fmt.Errorf("endpoints[%d].codec is unsupported", i)
		}
		if endpoint.TimeoutMs < 100 || endpoint.TimeoutMs > 30_000 {
			return fmt.Errorf("endpoints[%d].timeout_ms must be between 100 and 30000", i)
		}
		if endpoint.InputLimitRunes < 128 || endpoint.InputLimitRunes > setting.MaxInputRunes {
			return fmt.Errorf("endpoints[%d].input_limit_runes must be between 128 and max_input_runes", i)
		}
		if !isOneOf(endpoint.ProxyPolicy, RequestGuardProxyDisabled, RequestGuardProxyEnvironment, RequestGuardProxyExplicit) {
			return fmt.Errorf("endpoints[%d].proxy_policy is unsupported", i)
		}
		if endpoint.ProxyPolicy == RequestGuardProxyExplicit {
			proxyURL, err := url.Parse(endpoint.ProxyURL)
			if err != nil || proxyURL.Host == "" {
				return fmt.Errorf("endpoints[%d].proxy_url is required for explicit proxy", i)
			}
			if !isOneOf(strings.ToLower(proxyURL.Scheme), "http", "https", "socks5", "socks5h") {
				return fmt.Errorf("endpoints[%d].proxy_url scheme is unsupported", i)
			}
			if proxyURL.User != nil {
				return fmt.Errorf("endpoints[%d].proxy_url must not contain credentials", i)
			}
		}
	}
	if setting.Enabled && setting.Mode != RequestGuardModeOff && enabledCount == 0 {
		return fmt.Errorf("at least one enabled endpoint is required")
	}
	return nil
}

var requestGuardEndpointIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func normalizeRequestGuardSetting(setting RequestGuardSetting) RequestGuardSetting {
	setting.Mode = strings.ToLower(strings.TrimSpace(setting.Mode))
	setting.FailurePolicy = strings.ToLower(strings.TrimSpace(setting.FailurePolicy))
	setting.InputMode = strings.ToLower(strings.TrimSpace(setting.InputMode))
	setting.Scope.Groups = normalizeStringList(setting.Scope.Groups)
	setting.Scope.Models = normalizeStringList(setting.Scope.Models)
	setting.Scope.Protocols = normalizeStringList(setting.Scope.Protocols)
	for i := range setting.Endpoints {
		endpoint := &setting.Endpoints[i]
		endpoint.ID = strings.TrimSpace(endpoint.ID)
		endpoint.BaseURL = strings.TrimSpace(endpoint.BaseURL)
		endpoint.Model = strings.TrimSpace(endpoint.Model)
		endpoint.Codec = strings.ToLower(strings.TrimSpace(endpoint.Codec))
		endpoint.ProxyPolicy = strings.ToLower(strings.TrimSpace(endpoint.ProxyPolicy))
		endpoint.ProxyURL = strings.TrimSpace(endpoint.ProxyURL)
	}
	sort.SliceStable(setting.Endpoints, func(i, j int) bool {
		if setting.Endpoints[i].Priority == setting.Endpoints[j].Priority {
			return setting.Endpoints[i].ID < setting.Endpoints[j].ID
		}
		return setting.Endpoints[i].Priority > setting.Endpoints[j].Priority
	})
	return setting
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneRequestGuardSetting(setting RequestGuardSetting) RequestGuardSetting {
	setting.Scope.Groups = append([]string{}, setting.Scope.Groups...)
	setting.Scope.Models = append([]string{}, setting.Scope.Models...)
	setting.Scope.Protocols = append([]string{}, setting.Scope.Protocols...)
	setting.Endpoints = append([]RequestGuardEndpoint{}, setting.Endpoints...)
	return setting
}

func isOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
