package common

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxModelMappingDepth          = 32
	MaxModelMappingBytes          = 1 << 20
	MaxModelMappingRules          = 1024
	MaxModelMappingModelRunes     = 255
	MaxModelMappingCandidates     = 128
	MaxModelMappingEdges          = 4096
	MaxModelMappingWork           = 8192
	MaxModelMappingValidationWork = 65536
	ModelMappingVersionLegacy     = 1
	ModelMappingVersionRules      = 2
)

var ErrUnsupportedModelMappingVersion = errors.New("unsupported model mapping version")

type ModelMappingRule struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Priority int64  `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

type ModelMappingCandidate struct {
	Model    string
	RuleID   string
	Priority int64
}

// Legacy is a recursive alias graph; Rules are explicitly single-hop mappings.
// Keep that distinction through execution, even when both expose the same sources.
type ModelMappingConfig struct {
	Version int
	Legacy  map[string][]string
	Rules   []ModelMappingRule
}

func (config *ModelMappingConfig) IsRules() bool {
	return config != nil && config.Version == ModelMappingVersionRules
}

func ParseModelMappingConfig(raw string) (*ModelMappingConfig, error) {
	if len(raw) > MaxModelMappingBytes {
		return nil, fmt.Errorf("model mapping exceeds %d bytes", MaxModelMappingBytes)
	}
	raw = strings.TrimSpace(raw)
	config := &ModelMappingConfig{Version: ModelMappingVersionLegacy, Legacy: make(map[string][]string)}
	// Existing persisted null mappings historically behaved like an empty map.
	// ValidateModelMapping rejects newly written null without breaking reads.
	if raw == "" || raw == "{}" || raw == "null" {
		return config, nil
	}
	var fields map[string]RawMessage
	if err := UnmarshalJsonStr(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("model mapping must be a JSON object")
	}
	versionRaw := strings.TrimSpace(string(fields["version"]))
	if len(versionRaw) > 0 && (versionRaw[0] == '-' || (versionRaw[0] >= '0' && versionRaw[0] <= '9')) {
		var version float64
		if err := Unmarshal(fields["version"], &version); err != nil || math.Trunc(version) != version {
			return nil, fmt.Errorf("model mapping version must be an integer")
		}
		if version != ModelMappingVersionRules {
			return nil, fmt.Errorf("%w: %g", ErrUnsupportedModelMappingVersion, version)
		}
		uniqueFields, err := DecodeUniqueJSONObject(StringToByteSlice(raw))
		if err != nil {
			return nil, err
		}
		fields = uniqueFields
		if len(fields) != 2 || fields["rules"] == nil {
			return nil, fmt.Errorf("model mapping v2 requires only version and rules")
		}
		if strings.TrimSpace(string(fields["rules"])) == "null" {
			return nil, fmt.Errorf("model mapping rules must be an array")
		}
		var rawRules []RawMessage
		if err := Unmarshal(fields["rules"], &rawRules); err != nil {
			return nil, fmt.Errorf("model mapping rules must be an array")
		}
		if len(rawRules) > MaxModelMappingRules {
			return nil, fmt.Errorf("model mapping exceeds %d rules", MaxModelMappingRules)
		}
		config.Version = ModelMappingVersionRules
		config.Rules = make([]ModelMappingRule, 0, len(rawRules))
		seenIDs := make(map[string]bool, len(rawRules))
		for index, rawRule := range rawRules {
			rule, err := parseModelMappingRule(rawRule)
			if err != nil {
				return nil, fmt.Errorf("model mapping rule %d: %w", index+1, err)
			}
			if seenIDs[rule.ID] {
				return nil, fmt.Errorf("duplicate model mapping rule id: %q", rule.ID)
			}
			seenIDs[rule.ID] = true
			config.Rules = append(config.Rules, rule)
		}
		return config, nil
	}

	if len(fields) > MaxModelMappingRules {
		return nil, fmt.Errorf("model mapping exceeds %d sources", MaxModelMappingRules)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	edges := 0
	for _, originalSource := range keys {
		source, err := normalizeModelMappingName(originalSource)
		if err != nil {
			return nil, fmt.Errorf("model mapping source: %w", err)
		}
		if _, exists := config.Legacy[source]; exists {
			return nil, fmt.Errorf("duplicate model mapping source after trimming: %q", source)
		}
		value := strings.TrimSpace(string(fields[originalSource]))
		var targets []string
		if strings.HasPrefix(value, "\"") {
			var target string
			if err := Unmarshal(fields[originalSource], &target); err != nil {
				return nil, err
			}
			targets = []string{target}
		} else if strings.HasPrefix(value, "[") {
			var rawTargets []RawMessage
			if err := Unmarshal(fields[originalSource], &rawTargets); err != nil {
				return nil, err
			}
			for _, rawTarget := range rawTargets {
				var target string
				if strings.TrimSpace(string(rawTarget)) == "null" || Unmarshal(rawTarget, &target) != nil {
					return nil, fmt.Errorf("model mapping targets for %q must be strings", source)
				}
				targets = append(targets, target)
			}
		} else {
			return nil, fmt.Errorf("model mapping target for %q must be a string or string array", source)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("model mapping targets for %q must not be empty", source)
		}
		edges += len(targets)
		if edges > MaxModelMappingEdges {
			return nil, fmt.Errorf("model mapping exceeds %d edges", MaxModelMappingEdges)
		}
		seen := make(map[string]bool, len(targets))
		for _, rawTarget := range targets {
			target, err := normalizeModelMappingName(rawTarget)
			if err != nil {
				return nil, fmt.Errorf("model mapping target for %q: %w", source, err)
			}
			if !seen[target] {
				seen[target] = true
				config.Legacy[source] = append(config.Legacy[source], target)
			}
		}
	}
	return config, nil
}

func normalizeModelMappingName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("model name must not be empty")
	}
	if utf8.RuneCountInString(value) > MaxModelMappingModelRunes {
		return "", fmt.Errorf("model name exceeds %d characters", MaxModelMappingModelRunes)
	}
	return value, nil
}

func parseModelMappingRule(raw RawMessage) (ModelMappingRule, error) {
	rule := ModelMappingRule{Enabled: true}
	fields, err := DecodeUniqueJSONObject(raw)
	if err != nil {
		return rule, fmt.Errorf("invalid rule object: %w", err)
	}
	for key := range fields {
		switch key {
		case "id", "from", "to", "priority", "enabled":
		default:
			return rule, fmt.Errorf("unknown rule field: %q", key)
		}
	}
	for key, destination := range map[string]*string{"id": &rule.ID, "from": &rule.From, "to": &rule.To} {
		if fields[key] == nil || strings.TrimSpace(string(fields[key])) == "null" || Unmarshal(fields[key], destination) != nil {
			return rule, fmt.Errorf("%s must be a non-empty string", key)
		}
		*destination = strings.TrimSpace(*destination)
		if *destination == "" {
			return rule, fmt.Errorf("%s must not be empty", key)
		}
	}
	if utf8.RuneCountInString(rule.ID) > 128 {
		return rule, fmt.Errorf("rule id exceeds 128 characters")
	}
	if _, err := normalizeModelMappingName(rule.From); err != nil {
		return rule, fmt.Errorf("from: %w", err)
	}
	if _, err := normalizeModelMappingName(rule.To); err != nil {
		return rule, fmt.Errorf("to: %w", err)
	}
	if value, present := fields["priority"]; present {
		var priority float64
		if strings.TrimSpace(string(value)) == "null" || Unmarshal(value, &priority) != nil || math.Trunc(priority) != priority || priority < -2147483648 || priority > 2147483647 {
			return rule, fmt.Errorf("priority must be a signed 32-bit integer")
		}
		rule.Priority = int64(priority)
	}
	if value, present := fields["enabled"]; present {
		if strings.TrimSpace(string(value)) == "null" || Unmarshal(value, &rule.Enabled) != nil {
			return rule, fmt.Errorf("enabled must be a boolean")
		}
	}
	return rule, nil
}

func (config *ModelMappingConfig) Sources() []string {
	if config == nil {
		return nil
	}
	seen := make(map[string]bool)
	if config.IsRules() {
		for _, rule := range config.Rules {
			if rule.Enabled {
				seen[rule.From] = true
			}
		}
	} else {
		for source := range config.Legacy {
			seen[source] = true
		}
	}
	sources := make([]string, 0, len(seen))
	for source := range seen {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

func (config *ModelMappingConfig) Candidates(source string) ([]ModelMappingCandidate, error) {
	if config == nil {
		return nil, nil
	}
	source = strings.TrimSpace(source)
	if !config.IsRules() {
		models, err := ResolveModelMappingCandidates(config.Legacy, source)
		if err != nil {
			return nil, err
		}
		candidates := make([]ModelMappingCandidate, 0, len(models))
		for _, name := range models {
			candidates = append(candidates, ModelMappingCandidate{Model: name})
		}
		return candidates, nil
	}
	candidates := make([]ModelMappingCandidate, 0)
	for _, rule := range config.Rules {
		if rule.Enabled && rule.From == source {
			candidates = append(candidates, ModelMappingCandidate{Model: rule.To, RuleID: rule.ID, Priority: rule.Priority})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Priority > candidates[j].Priority })
	seen := make(map[string]bool)
	result := make([]ModelMappingCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate.Model] {
			continue
		}
		seen[candidate.Model] = true
		result = append(result, candidate)
		if len(result) > MaxModelMappingCandidates {
			return nil, fmt.Errorf("model mapping exceeds %d resolved candidates", MaxModelMappingCandidates)
		}
	}
	return result, nil
}

// ParseModelMapping retains the legacy lookup contract for callers enumerating
// routing sources. Execution must use Config.Candidates to preserve v2's direct mode.
func ParseModelMapping(raw string) (map[string][]string, error) {
	config, err := ParseModelMappingConfig(raw)
	if err != nil {
		return nil, err
	}
	if !config.IsRules() {
		return config.Legacy, nil
	}
	mapping := make(map[string][]string)
	for _, source := range config.Sources() {
		candidates, err := config.Candidates(source)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			mapping[source] = append(mapping[source], candidate.Model)
		}
	}
	return mapping, nil
}

func ResolveModelMappingCandidates(mapping map[string][]string, source string) ([]string, error) {
	work := MaxModelMappingWork
	return resolveLegacyModelMapping(mapping, source, &work)
}

func resolveLegacyModelMapping(mapping map[string][]string, source string, remaining *int) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	if _, exists := mapping[source]; !exists {
		return nil, nil
	}
	resolved := make([]string, 0)
	seen := make(map[string]bool)
	path := make(map[string]bool)
	add := func(target string) error {
		if seen[target] {
			return nil
		}
		if len(resolved) >= MaxModelMappingCandidates {
			return fmt.Errorf("model mapping exceeds %d resolved candidates", MaxModelMappingCandidates)
		}
		seen[target] = true
		resolved = append(resolved, target)
		return nil
	}
	var expand func(string, int) error
	expand = func(current string, depth int) error {
		if *remaining <= 0 {
			return fmt.Errorf("model mapping exceeds expansion work limit")
		}
		*remaining--
		if depth > maxModelMappingDepth {
			return fmt.Errorf("model mapping exceeds maximum depth")
		}
		if path[current] {
			return fmt.Errorf("model mapping contains cycle at %q", current)
		}
		targets, exists := mapping[current]
		if !exists {
			return add(current)
		}
		path[current] = true
		defer delete(path, current)
		for _, target := range targets {
			if target == current {
				if *remaining <= 0 {
					return fmt.Errorf("model mapping exceeds expansion work limit")
				}
				*remaining--
				if err := add(target); err != nil {
					return err
				}
			} else if err := expand(target, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := expand(source, 0); err != nil {
		return nil, err
	}
	return resolved, nil
}

func ValidateModelMapping(raw string) error {
	if len(raw) > MaxModelMappingBytes {
		return fmt.Errorf("model mapping exceeds %d bytes", MaxModelMappingBytes)
	}
	if strings.TrimSpace(raw) != "" {
		if _, err := DecodeUniqueJSONObject(StringToByteSlice(raw)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(raw) == "null" {
		return fmt.Errorf("model mapping must not be null")
	}
	config, err := ParseModelMappingConfig(raw)
	if err != nil {
		return err
	}
	remaining := MaxModelMappingValidationWork
	for _, source := range config.Sources() {
		if config.IsRules() {
			if _, err := config.Candidates(source); err != nil {
				return err
			}
			continue
		}
		work := MaxModelMappingWork
		if remaining < work {
			work = remaining
		}
		before := work
		if _, err := resolveLegacyModelMapping(config.Legacy, source, &work); err != nil {
			return err
		}
		remaining -= before - work
	}
	return nil
}

func ValidateModelMappingChange(previous, next string) error {
	if previous == next {
		return nil
	}
	if err := ValidateModelMapping(next); err != nil {
		return err
	}
	before, previousError := ParseModelMappingConfig(previous)
	if errors.Is(previousError, ErrUnsupportedModelMappingVersion) {
		return previousError
	}
	after, err := ParseModelMappingConfig(next)
	if err != nil {
		return err
	}
	if before != nil && before.IsRules() && !after.IsRules() {
		return fmt.Errorf("v2 model mapping cannot be silently downgraded; use version 2 with an empty rules array to clear it")
	}
	return nil
}
