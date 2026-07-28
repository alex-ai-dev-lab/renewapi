package common

import (
	"fmt"
	"strings"
)

const maxModelMappingDepth = 32

// ParseModelMapping accepts both legacy one-to-one mappings and ordered
// fallback arrays.
func ParseModelMapping(raw string) (map[string][]string, error) {
	result := make(map[string][]string)
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" { return result, nil }
	var decoded map[string]any
	if err := UnmarshalJsonStr(raw, &decoded); err != nil { return nil, err }
	for source, value := range decoded {
		source = strings.TrimSpace(source)
		if source == "" { return nil, fmt.Errorf("model mapping source must not be empty") }
		var candidates []string
		switch typed := value.(type) {
		case string:
			candidates = []string{typed}
		case []any:
			for _, item := range typed {
				candidate, ok := item.(string)
				if !ok { return nil, fmt.Errorf("model mapping targets for %q must be strings", source) }
				candidates = append(candidates, candidate)
			}
		default:
			return nil, fmt.Errorf("model mapping target for %q must be a string or string array", source)
		}
		seen := make(map[string]struct{}, len(candidates))
		normalized := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" { return nil, fmt.Errorf("model mapping target for %q must not be empty", source) }
			if _, exists := seen[candidate]; exists { continue }
			seen[candidate] = struct{}{}
			normalized = append(normalized, candidate)
		}
		if len(normalized) == 0 { return nil, fmt.Errorf("model mapping targets for %q must not be empty", source) }
		result[source] = normalized
	}
	return result, nil
}

func ResolveModelMappingCandidates(mapping map[string][]string, source string) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" { return nil, nil }
	if _, exists := mapping[source]; !exists { return nil, nil }
	resolved := make([]string, 0)
	seenResolved := make(map[string]struct{})
	var expand func(string, map[string]bool, int) error
	expand = func(current string, path map[string]bool, depth int) error {
		if depth > maxModelMappingDepth { return fmt.Errorf("model mapping exceeds maximum depth") }
		if path[current] { return fmt.Errorf("model mapping contains cycle at %q", current) }
		targets, exists := mapping[current]
		if !exists {
			if _, seen := seenResolved[current]; !seen { seenResolved[current] = struct{}{}; resolved = append(resolved, current) }
			return nil
		}
		nextPath := make(map[string]bool, len(path)+1)
		for modelName, active := range path { nextPath[modelName] = active }
		nextPath[current] = true
		for _, target := range targets {
			if target == current {
				if _, seen := seenResolved[target]; !seen { seenResolved[target] = struct{}{}; resolved = append(resolved, target) }
				continue
			}
			if err := expand(target, nextPath, depth+1); err != nil { return err }
		}
		return nil
	}
	if err := expand(source, nil, 0); err != nil { return nil, err }
	return resolved, nil
}
