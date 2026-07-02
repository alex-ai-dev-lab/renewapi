package operation_setting

import (
	"encoding/json"
	"strings"
	"sync"
)

// ModelEndpointDefaultEntry maps a model-name pattern to an upstream protocol
// (channel type). It only controls the protocol/adaptor selection; the base URL
// is always left as the resolved channel's own base URL by the routing layer.
type ModelEndpointDefaultEntry struct {
	// MatchType is "exact" or "prefix".
	MatchType string `json:"match_type"`
	// Pattern is the model name (for "exact") or a case-insensitive prefix (for
	// "prefix") to match against the requested model name.
	Pattern string `json:"pattern"`
	// ChannelType is the upstream channel type/adaptor matching models route to.
	ChannelType int `json:"channel_type"`
}

// ModelEndpointDefaults is the global, admin-configurable registry that maps
// model names to a default upstream protocol regardless of the serving
// channel's own type. It is disabled by default; when enabled the routing layer
// consults it for requests that have no per-channel per-model override.
type ModelEndpointDefaults struct {
	Enabled bool                        `json:"enabled"`
	Entries []ModelEndpointDefaultEntry `json:"entries"`
}

var (
	modelEndpointDefaults     = defaultModelEndpointDefaults()
	modelEndpointDefaultsLock sync.RWMutex
)

// defaultModelEndpointDefaults seeds the registry with the mainstream model
// families grouped by API protocol. Channel type constants (kept as literals to
// avoid importing the model/constant packages and any import cycle):
//   1  = OpenAI-compatible, 14 = Anthropic, 24 = Gemini, 48 = xAI.
func defaultModelEndpointDefaults() ModelEndpointDefaults {
	return ModelEndpointDefaults{
		Enabled: false,
		Entries: []ModelEndpointDefaultEntry{
			{MatchType: "prefix", Pattern: "claude", ChannelType: 14},
			{MatchType: "prefix", Pattern: "gemini", ChannelType: 24},
			{MatchType: "prefix", Pattern: "gemma", ChannelType: 24},
			{MatchType: "prefix", Pattern: "grok", ChannelType: 48},
			{MatchType: "prefix", Pattern: "gpt", ChannelType: 1},
			{MatchType: "prefix", Pattern: "chatgpt", ChannelType: 1},
			{MatchType: "prefix", Pattern: "o1", ChannelType: 1},
			{MatchType: "prefix", Pattern: "o3", ChannelType: 1},
			{MatchType: "prefix", Pattern: "o4", ChannelType: 1},
			{MatchType: "prefix", Pattern: "text-embedding", ChannelType: 1},
			{MatchType: "prefix", Pattern: "dall-e", ChannelType: 1},
			{MatchType: "prefix", Pattern: "whisper", ChannelType: 1},
			{MatchType: "prefix", Pattern: "tts", ChannelType: 1},
			{MatchType: "prefix", Pattern: "deepseek", ChannelType: 1},
			{MatchType: "prefix", Pattern: "qwen", ChannelType: 1},
			{MatchType: "prefix", Pattern: "kimi", ChannelType: 1},
			{MatchType: "prefix", Pattern: "moonshot", ChannelType: 1},
			{MatchType: "prefix", Pattern: "mistral", ChannelType: 1},
			{MatchType: "prefix", Pattern: "glm", ChannelType: 1},
			{MatchType: "prefix", Pattern: "yi", ChannelType: 1},
			{MatchType: "prefix", Pattern: "minimax", ChannelType: 1},
			{MatchType: "prefix", Pattern: "llama", ChannelType: 1},
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
		modelEndpointDefaults = ModelEndpointDefaults{}
		modelEndpointDefaultsLock.Unlock()
		return nil
	}
	var parsed ModelEndpointDefaults
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}
	modelEndpointDefaultsLock.Lock()
	modelEndpointDefaults = parsed
	modelEndpointDefaultsLock.Unlock()
	return nil
}

// ResolveModelDefaultChannelType returns the globally-configured default upstream
// channel type for a model name. Exact patterns are matched first, then the
// longest matching prefix wins. The bool result is false when the registry is
// disabled or nothing matches, in which case callers keep their existing
// behavior. This only decides the protocol/adaptor; the base URL is never
// changed by the caller on the strength of this result.
func ResolveModelDefaultChannelType(modelName string) (int, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return 0, false
	}
	modelEndpointDefaultsLock.RLock()
	defer modelEndpointDefaultsLock.RUnlock()
	if !modelEndpointDefaults.Enabled {
		return 0, false
	}
	// Exact match first.
	for _, e := range modelEndpointDefaults.Entries {
		if strings.EqualFold(e.MatchType, "exact") && strings.ToLower(strings.TrimSpace(e.Pattern)) == name {
			return e.ChannelType, true
		}
	}
	// Then the longest matching prefix.
	bestLen := -1
	bestType := 0
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
			bestType = e.ChannelType
		}
	}
	if bestLen >= 0 {
		return bestType, true
	}
	return 0, false
}
