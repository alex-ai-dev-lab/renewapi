package operation_setting

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/constant"
)

// ModelEndpointDefaultEntry maps a model-name pattern to a route profile. The
// legacy channel_type-only shape remains valid; missing endpoint fields are
// inferred from the model name and channel type at read time.
type ModelEndpointDefaultEntry struct {
	// MatchType is "exact" or "prefix".
	MatchType string `json:"match_type"`
	// Pattern is the model name (for "exact") or a case-insensitive prefix (for
	// "prefix") to match against the requested model name.
	Pattern string `json:"pattern"`
	// ChannelType is the upstream channel type/adaptor matching models route to.
	ChannelType int `json:"channel_type"`
	// DefaultEndpoint is the most common endpoint for the matched model family.
	DefaultEndpoint string `json:"default_endpoint,omitempty"`
	// SupportedEndpoints lists endpoint families that can be served for this
	// model. A correct client request in this list is respected as-is.
	SupportedEndpoints []string `json:"supported_endpoints,omitempty"`
	// FallbackEndpoint is used for safe internal endpoint correction. Empty means
	// DefaultEndpoint.
	FallbackEndpoint string `json:"fallback_endpoint,omitempty"`
	// AutoCorrect controls safe internal endpoint correction. Nil keeps the
	// default behavior (enabled).
	AutoCorrect *bool `json:"auto_correct,omitempty"`
}

// ModelEndpointDefaults is the global, admin-configurable registry that maps
// model names to a default route profile regardless of the serving channel's own
// type. Per-channel per-model overrides still take precedence.
type ModelEndpointDefaults struct {
	Enabled bool                        `json:"enabled"`
	Entries []ModelEndpointDefaultEntry `json:"entries"`
}

// ModelEndpointRouteProfile is the normalized, effective profile for a model.
type ModelEndpointRouteProfile struct {
	MatchType          string   `json:"match_type"`
	Pattern            string   `json:"pattern"`
	ChannelType        int      `json:"channel_type"`
	DefaultEndpoint    string   `json:"default_endpoint"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	FallbackEndpoint   string   `json:"fallback_endpoint"`
	AutoCorrect        bool     `json:"auto_correct"`
}

// ModelEndpointDecision describes how a model/profile treats a client endpoint.
type ModelEndpointDecision struct {
	RequestedEndpoint  string
	EffectiveEndpoint  string
	DefaultEndpoint    string
	SupportedEndpoints []string
	ChannelType        int
	AutoCorrected      bool
	Supported          bool
	Reason             string
}

var (
	modelEndpointDefaults     = defaultModelEndpointDefaults()
	modelEndpointDefaultsLock sync.RWMutex
)

func boolPtr(v bool) *bool {
	return &v
}

func endpoint(v constant.EndpointType) string {
	return string(v)
}

func endpoints(values ...constant.EndpointType) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, endpoint(value))
	}
	return items
}

func profile(matchType, pattern string, channelType int, defaultEndpoint constant.EndpointType, supported ...constant.EndpointType) ModelEndpointDefaultEntry {
	return ModelEndpointDefaultEntry{
		MatchType:          matchType,
		Pattern:            pattern,
		ChannelType:        channelType,
		DefaultEndpoint:    endpoint(defaultEndpoint),
		SupportedEndpoints: endpoints(supported...),
		FallbackEndpoint:   endpoint(defaultEndpoint),
		AutoCorrect:        boolPtr(true),
	}
}

// defaultModelEndpointDefaults seeds route profiles for the mainstream model
// families. The list intentionally mixes exact and prefix rules: exact rules
// cover endpoint-sensitive models, while broad prefixes cover the common public
// model families that make up the practical "top 100" used by clients.
func defaultModelEndpointDefaults() ModelEndpointDefaults {
	textOpenAI := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAIResponseCompact,
	}
	return ModelEndpointDefaults{
		Enabled: false,
		Entries: []ModelEndpointDefaultEntry{
			// Image generation models must not be treated as chat models.
			profile("prefix", "gpt-image", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "dall-e", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "imagen", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "flux", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "stable-diffusion", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "sd3", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "recraft", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),
			profile("prefix", "ideogram", 1, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits),

			// Embeddings.
			profile("prefix", "text-embedding", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "embedding", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "bge-m3", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "bge-large", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "e5-", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "nomic-embed", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "jina-embeddings", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "voyage", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),
			profile("prefix", "cohere-embed", 1, constant.EndpointTypeEmbeddings, constant.EndpointTypeEmbeddings),

			// Audio.
			profile("exact", "gpt-4o-mini-tts", 1, constant.EndpointTypeAudioSpeech, constant.EndpointTypeAudioSpeech),
			profile("prefix", "tts", 1, constant.EndpointTypeAudioSpeech, constant.EndpointTypeAudioSpeech),
			profile("exact", "gpt-4o-transcribe", 1, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranslation),
			profile("exact", "gpt-4o-mini-transcribe", 1, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranslation),
			profile("prefix", "whisper", 1, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranslation),

			// Rerank.
			profile("prefix", "rerank", 1, constant.EndpointTypeJinaRerank, constant.EndpointTypeJinaRerank),
			profile("prefix", "bge-reranker", 1, constant.EndpointTypeJinaRerank, constant.EndpointTypeJinaRerank),
			profile("prefix", "jina-reranker", 1, constant.EndpointTypeJinaRerank, constant.EndpointTypeJinaRerank),
			profile("prefix", "cohere-rerank", 1, constant.EndpointTypeJinaRerank, constant.EndpointTypeJinaRerank),

			// OpenAI text and reasoning models.
			profile("exact", "o3-pro", 1, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact),
			profile("exact", "o3-deep-research", 1, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact),
			profile("exact", "o4-mini-deep-research", 1, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact),
			profile("prefix", "gpt-5", 1, constant.EndpointTypeOpenAIResponse, textOpenAI...),
			profile("prefix", "gpt5", 1, constant.EndpointTypeOpenAIResponse, textOpenAI...),
			profile("prefix", "gpt-4.5", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "gpt-4.1", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "gpt-4o", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "gpt-4", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "gpt-3.5", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "gpt", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "chatgpt", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "o1", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "o3", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "o4", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "codex", 1, constant.EndpointTypeOpenAI, textOpenAI...),

			// Native protocol families with OpenAI-compatible client support.
			profile("prefix", "claude", 14, constant.EndpointTypeAnthropic, constant.EndpointTypeAnthropic, constant.EndpointTypeOpenAI),
			profile("prefix", "gemini", 24, constant.EndpointTypeGemini, constant.EndpointTypeGemini, constant.EndpointTypeOpenAI),
			profile("prefix", "gemma", 24, constant.EndpointTypeGemini, constant.EndpointTypeGemini, constant.EndpointTypeOpenAI),
			profile("prefix", "grok", 48, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse),

			// Popular OpenAI-compatible third-party families.
			profile("prefix", "deepseek", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "qwen3", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "qwen2.5", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "qwen", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "qwq", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "qvq", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "kimi-k2", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "kimi", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "moonshot", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "mistral-large", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "magistral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "codestral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "ministral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "pixtral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "mixtral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "mistral", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "llama-4", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "llama-3.3", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "llama-3.1", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "llama-3", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "meta-llama", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "llama", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "command-a", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "command-r", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "command", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "cohere", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "glm-4.5", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "glm-4", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "glm", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "zai-", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "yi", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "abab", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "minimax", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "doubao", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "ernie", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "baichuan", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "internlm", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "hunyuan", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "sparkdesk", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "spark", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "phi-4", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "phi", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "nemotron", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "solar", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "sonar", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "perplexity", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "pplx", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "reka", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "ai21", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "jamba", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "nova", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "titan", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "granite", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "olmo", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "vicuna", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "openchat", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "wizardlm", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "nous", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "hermes", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "dolphin", 1, constant.EndpointTypeOpenAI, textOpenAI...),
			profile("prefix", "zephyr", 1, constant.EndpointTypeOpenAI, textOpenAI...),

			// Common video generation families.
			profile("prefix", "sora", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "veo", 24, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "kling", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "hailuo", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "luma", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "runway", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
			profile("prefix", "vidu", 1, constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAIVideo),
		},
	}
}

// GetModelEndpointDefaults returns a copy-safe snapshot of the current registry.
func GetModelEndpointDefaults() ModelEndpointDefaults {
	modelEndpointDefaultsLock.RLock()
	defer modelEndpointDefaultsLock.RUnlock()
	return modelEndpointDefaults
}

// ModelEndpointDefaults2JsonString serializes the current registry for storage
// in the OptionMap. It mirrors the other *2JsonString helpers in this package.
func ModelEndpointDefaults2JsonString() string {
	modelEndpointDefaultsLock.RLock()
	defer modelEndpointDefaultsLock.RUnlock()
	b, err := json.Marshal(modelEndpointDefaults)
	if err != nil {
		return ""
	}
	return string(b)
}

// UpdateModelEndpointDefaultsByJsonString replaces the in-memory registry from a
// persisted JSON string. An empty string resets to an empty (disabled) registry.
func UpdateModelEndpointDefaultsByJsonString(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		modelEndpointDefaultsLock.Lock()
		modelEndpointDefaults = defaultModelEndpointDefaults()
		modelEndpointDefaultsLock.Unlock()
		return nil
	}
	var parsed ModelEndpointDefaults
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}
	modelEndpointDefaultsLock.Lock()
	modelEndpointDefaults = mergeWithBuiltInProfiles(parsed)
	modelEndpointDefaultsLock.Unlock()
	return nil
}

// ResolveModelDefaultProfile returns the normalized route profile for a model.
func ResolveModelDefaultProfile(modelName string) (ModelEndpointRouteProfile, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return ModelEndpointRouteProfile{}, false
	}
	modelEndpointDefaultsLock.RLock()
	defer modelEndpointDefaultsLock.RUnlock()
	if !modelEndpointDefaults.Enabled {
		return ModelEndpointRouteProfile{}, false
	}
	if entry, ok := matchModelEndpointEntryLocked(name); ok {
		return normalizeModelEndpointProfile(name, entry), true
	}
	return ModelEndpointRouteProfile{}, false
}

// ForceModelDefaultEndpoint returns the configured default endpoint for a model.
// It is used by opted-in channels that intentionally ignore the client endpoint
// and force the upstream request family from the global model rules.
func ForceModelDefaultEndpoint(modelName string) (string, bool) {
	profile, ok := ResolveModelDefaultProfile(modelName)
	if !ok || profile.DefaultEndpoint == "" {
		return "", false
	}
	return profile.DefaultEndpoint, true
}

// ResolveModelDefaultChannelType keeps the legacy protocol-only API for callers
// that do not need endpoint information.
func ResolveModelDefaultChannelType(modelName string) (int, bool) {
	profile, ok := ResolveModelDefaultProfile(modelName)
	if !ok {
		return 0, false
	}
	return profile.ChannelType, true
}

// ResolveModelEndpointDecision compares a client-requested endpoint to the
// model profile. Correct client endpoints are respected. Unsafe mismatches are
// reported with a recommended endpoint instead of silently changing semantics.
func ResolveModelEndpointDecision(modelName, requestedEndpoint string) (ModelEndpointDecision, bool) {
	profile, ok := ResolveModelDefaultProfile(modelName)
	if !ok {
		return ModelEndpointDecision{}, false
	}
	requested := normalizeEndpoint(requestedEndpoint)
	decision := ModelEndpointDecision{
		RequestedEndpoint:  requested,
		EffectiveEndpoint:  profile.DefaultEndpoint,
		DefaultEndpoint:    profile.DefaultEndpoint,
		SupportedEndpoints: append([]string(nil), profile.SupportedEndpoints...),
		ChannelType:        profile.ChannelType,
		Supported:          true,
	}
	if requested == "" {
		decision.Reason = "no endpoint requested"
		return decision, true
	}
	if endpointInList(requested, profile.SupportedEndpoints) {
		decision.EffectiveEndpoint = requested
		decision.Reason = "requested endpoint supported"
		return decision, true
	}
	fallback := profile.FallbackEndpoint
	if fallback == "" {
		fallback = profile.DefaultEndpoint
	}
	decision.EffectiveEndpoint = fallback
	decision.Supported = false
	if profile.AutoCorrect && canSafelyCorrectEndpoint(requested, fallback) {
		decision.AutoCorrected = true
		decision.Supported = true
		decision.Reason = "safe endpoint correction"
		return decision, true
	}
	decision.Reason = "unsupported endpoint for model"
	return decision, true
}

func matchModelEndpointEntryLocked(name string) (ModelEndpointDefaultEntry, bool) {
	for _, e := range modelEndpointDefaults.Entries {
		if strings.EqualFold(e.MatchType, "exact") && strings.ToLower(strings.TrimSpace(e.Pattern)) == name {
			return e, true
		}
	}
	bestLen := -1
	var best ModelEndpointDefaultEntry
	for _, e := range modelEndpointDefaults.Entries {
		if !strings.EqualFold(e.MatchType, "prefix") {
			continue
		}
		p := strings.ToLower(strings.TrimSpace(e.Pattern))
		if p == "" {
			continue
		}
		if strings.HasPrefix(name, p) && len(p) > bestLen {
			bestLen = len(p)
			best = e
		}
	}
	return best, bestLen >= 0
}

func normalizeModelEndpointProfile(modelName string, e ModelEndpointDefaultEntry) ModelEndpointRouteProfile {
	channelType := e.ChannelType
	if channelType == 0 {
		channelType = 1
	}
	defaultEndpoint := normalizeEndpoint(e.DefaultEndpoint)
	if defaultEndpoint == "" {
		defaultEndpoint = inferDefaultEndpoint(modelName, channelType)
	}
	supported := normalizeEndpointList(e.SupportedEndpoints)
	if len(supported) == 0 {
		supported = inferSupportedEndpoints(defaultEndpoint, channelType)
	}
	if !endpointInList(defaultEndpoint, supported) {
		supported = append([]string{defaultEndpoint}, supported...)
	}
	fallback := normalizeEndpoint(e.FallbackEndpoint)
	if fallback == "" {
		fallback = defaultEndpoint
	}
	autoCorrect := true
	if e.AutoCorrect != nil {
		autoCorrect = *e.AutoCorrect
	}
	return ModelEndpointRouteProfile{
		MatchType:          normalizeMatchType(e.MatchType),
		Pattern:            strings.TrimSpace(e.Pattern),
		ChannelType:        channelType,
		DefaultEndpoint:    defaultEndpoint,
		SupportedEndpoints: supported,
		FallbackEndpoint:   fallback,
		AutoCorrect:        autoCorrect,
	}
}

func normalizeMatchType(value string) string {
	if strings.EqualFold(value, "exact") {
		return "exact"
	}
	return "prefix"
}

func normalizeEndpoint(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeEndpointList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeEndpoint(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func endpointInList(endpoint string, values []string) bool {
	endpoint = normalizeEndpoint(endpoint)
	if endpoint == "" {
		return false
	}
	for _, value := range values {
		if endpoint == normalizeEndpoint(value) {
			return true
		}
	}
	return false
}

func inferDefaultEndpoint(modelName string, channelType int) string {
	name := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case hasAnyPrefix(name, "gpt-image", "dall-e", "imagen", "flux", "stable-diffusion", "sd3", "recraft", "ideogram"):
		return endpoint(constant.EndpointTypeImageGeneration)
	case hasAnyPrefix(name, "text-embedding", "embedding", "bge-m3", "bge-large", "e5-", "nomic-embed", "jina-embeddings", "voyage", "cohere-embed"):
		return endpoint(constant.EndpointTypeEmbeddings)
	case hasAnyPrefix(name, "tts") || name == "gpt-4o-mini-tts":
		return endpoint(constant.EndpointTypeAudioSpeech)
	case hasAnyPrefix(name, "whisper") || name == "gpt-4o-transcribe" || name == "gpt-4o-mini-transcribe":
		return endpoint(constant.EndpointTypeAudioTranscription)
	case hasAnyPrefix(name, "rerank", "bge-reranker", "jina-reranker", "cohere-rerank"):
		return endpoint(constant.EndpointTypeJinaRerank)
	case hasAnyPrefix(name, "sora", "veo", "kling", "hailuo", "luma", "runway", "vidu"):
		return endpoint(constant.EndpointTypeOpenAIVideo)
	}
	switch channelType {
	case 14:
		return endpoint(constant.EndpointTypeAnthropic)
	case 24:
		return endpoint(constant.EndpointTypeGemini)
	default:
		return endpoint(constant.EndpointTypeOpenAI)
	}
}

func inferSupportedEndpoints(defaultEndpoint string, channelType int) []string {
	switch defaultEndpoint {
	case endpoint(constant.EndpointTypeImageGeneration):
		return endpoints(constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits)
	case endpoint(constant.EndpointTypeEmbeddings):
		return endpoints(constant.EndpointTypeEmbeddings)
	case endpoint(constant.EndpointTypeAudioSpeech):
		return endpoints(constant.EndpointTypeAudioSpeech)
	case endpoint(constant.EndpointTypeAudioTranscription), endpoint(constant.EndpointTypeAudioTranslation):
		return endpoints(constant.EndpointTypeAudioTranscription, constant.EndpointTypeAudioTranslation)
	case endpoint(constant.EndpointTypeJinaRerank):
		return endpoints(constant.EndpointTypeJinaRerank)
	case endpoint(constant.EndpointTypeOpenAIVideo):
		return endpoints(constant.EndpointTypeOpenAIVideo)
	}
	switch channelType {
	case 14:
		return endpoints(constant.EndpointTypeAnthropic, constant.EndpointTypeOpenAI)
	case 24:
		return endpoints(constant.EndpointTypeGemini, constant.EndpointTypeOpenAI)
	case 48:
		return endpoints(constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse)
	default:
		return endpoints(constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact)
	}
}

func hasAnyPrefix(name string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func mergeWithBuiltInProfiles(custom ModelEndpointDefaults) ModelEndpointDefaults {
	builtIn := defaultModelEndpointDefaults()
	custom.Entries = normalizeMergedEntries(custom.Entries, builtIn.Entries)
	return custom
}

func normalizeMergedEntries(primary, fallback []ModelEndpointDefaultEntry) []ModelEndpointDefaultEntry {
	merged := make([]ModelEndpointDefaultEntry, 0, len(primary)+len(fallback))
	seen := make(map[string]struct{}, len(primary)+len(fallback))
	appendEntries := func(entries []ModelEndpointDefaultEntry) {
		for _, entry := range entries {
			key := normalizeMatchType(entry.MatchType) + "|" + strings.ToLower(strings.TrimSpace(entry.Pattern))
			if key == "prefix|" || key == "exact|" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, entry)
		}
	}
	appendEntries(primary)
	appendEntries(fallback)
	return merged
}

func canSafelyCorrectEndpoint(requested, fallback string) bool {
	requested = normalizeEndpoint(requested)
	fallback = normalizeEndpoint(fallback)
	if requested == "" || fallback == "" || requested == fallback {
		return false
	}
	// These are the text request families for which the relay already has
	// request/response conversion paths. Responses requests are intentionally
	// excluded from correction because their request shape is not chat-compatible.
	textConvertible := map[string]bool{
		endpoint(constant.EndpointTypeOpenAI):    true,
		endpoint(constant.EndpointTypeAnthropic): true,
		endpoint(constant.EndpointTypeGemini):    true,
	}
	return textConvertible[requested] && textConvertible[fallback]
}
