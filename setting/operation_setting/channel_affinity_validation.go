package operation_setting

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ValidateChannelAffinityRulesJSON is the server-side validation boundary for
// rules loaded from the settings API, imports, or any other JSON-backed path.
func ValidateChannelAffinityRulesJSON(raw string) error {
	var rules []ChannelAffinityRule
	if err := common.UnmarshalJsonStr(raw, &rules); err != nil {
		return fmt.Errorf("channel affinity rules must be a JSON array: %w", err)
	}
	return ValidateChannelAffinityRules(rules)
}

// ValidateChannelAffinityRules validates the parts of a rule that affect
// routing, cache identity, or request mutation. The runtime uses Go's RE2
// regexp implementation, so accepting a browser-only pattern would create a
// rule that appears saved but can never match.
func ValidateChannelAffinityRules(rules []ChannelAffinityRule) error {
	seenNames := make(map[string]struct{}, len(rules))
	for index, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return fmt.Errorf("channel affinity rule %d: name is required", index)
		}
		if name != rule.Name {
			return fmt.Errorf("channel affinity rule %q: name cannot have surrounding whitespace", rule.Name)
		}
		nameKey := strings.ToLower(name)
		if _, exists := seenNames[nameKey]; exists {
			return fmt.Errorf("channel affinity rule %q is duplicated", name)
		}
		seenNames[nameKey] = struct{}{}

		if len(rule.ModelRegex) == 0 {
			return fmt.Errorf("channel affinity rule %q: model_regex is required", name)
		}
		if err := validateRegexList(name, "model_regex", rule.ModelRegex); err != nil {
			return err
		}
		if err := validateRegexList(name, "path_regex", rule.PathRegex); err != nil {
			return err
		}
		if strings.TrimSpace(rule.ValueRegex) != "" {
			if _, err := regexp.Compile(rule.ValueRegex); err != nil {
				return fmt.Errorf("channel affinity rule %q value_regex is invalid: %w", name, err)
			}
		}
		for _, value := range rule.UserAgentInclude {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("channel affinity rule %q contains an empty user_agent_include value", name)
			}
		}
		if rule.TTLSeconds < 0 {
			return fmt.Errorf("channel affinity rule %q: ttl_seconds cannot be negative", name)
		}
		if len(rule.KeySources) == 0 {
			return fmt.Errorf("channel affinity rule %q: at least one key source is required", name)
		}
		for sourceIndex, source := range rule.KeySources {
			if err := validateChannelAffinityKeySource(name, sourceIndex, source); err != nil {
				return err
			}
		}
		if err := validateChannelAffinityTemplate(name, rule.ParamOverrideTemplate); err != nil {
			return err
		}
	}
	return nil
}

func validateRegexList(ruleName, field string, patterns []string) error {
	for index, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("channel affinity rule %q %s[%d] cannot be empty", ruleName, field, index)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("channel affinity rule %q %s[%d] is invalid: %w", ruleName, field, index, err)
		}
	}
	return nil
}

func validateChannelAffinityKeySource(ruleName string, index int, source ChannelAffinityKeySource) error {
	typeName := strings.TrimSpace(source.Type)
	if typeName != source.Type || typeName != strings.ToLower(typeName) {
		return fmt.Errorf("channel affinity rule %q key_sources[%d]: type must be lowercase without surrounding whitespace", ruleName, index)
	}
	switch typeName {
	case "context_int", "context_string", "request_header":
		if strings.TrimSpace(source.Key) == "" {
			return fmt.Errorf("channel affinity rule %q key_sources[%d]: key is required", ruleName, index)
		}
	case "gjson":
		if strings.TrimSpace(source.Path) == "" {
			return fmt.Errorf("channel affinity rule %q key_sources[%d]: path is required", ruleName, index)
		}
	default:
		return fmt.Errorf("channel affinity rule %q key_sources[%d]: unsupported type %q", ruleName, index, source.Type)
	}
	return nil
}

var channelAffinityOperationFields = map[string]struct{}{
	"path":        {},
	"mode":        {},
	"value":       {},
	"keep_origin": {},
	"from":        {},
	"to":          {},
	"conditions":  {},
	"logic":       {},
}

var channelAffinityOperationModes = map[string]struct{}{
	"delete":        {},
	"set":           {},
	"move":          {},
	"copy":          {},
	"prepend":       {},
	"append":        {},
	"trim_prefix":   {},
	"trim_suffix":   {},
	"ensure_prefix": {},
	"ensure_suffix": {},
	"trim_space":    {},
	"to_lower":      {},
	"to_upper":      {},
	"replace":       {},
	"regex_replace": {},
	"return_error":  {},
	"prune_objects": {},
	"set_header":    {},
	"delete_header": {},
	"copy_header":   {},
	"move_header":   {},
	"pass_headers":  {},
	"sync_fields":   {},
}

var channelAffinityPathModes = map[string]struct{}{
	"delete":        {},
	"set":           {},
	"prepend":       {},
	"append":        {},
	"trim_prefix":   {},
	"trim_suffix":   {},
	"ensure_prefix": {},
	"ensure_suffix": {},
	"trim_space":    {},
	"to_lower":      {},
	"to_upper":      {},
	"replace":       {},
	"regex_replace": {},
}

func validateChannelAffinityTemplate(ruleName string, template map[string]interface{}) error {
	if len(template) == 0 {
		return nil
	}

	var operationsValue interface{}
	hasOperations := false
	for key, value := range template {
		if strings.EqualFold(strings.TrimSpace(key), "operations") {
			if key != "operations" {
				return fmt.Errorf("channel affinity rule %q: template field %q must be named operations", ruleName, key)
			}
			if hasOperations {
				return fmt.Errorf("channel affinity rule %q: template.operations is duplicated", ruleName)
			}
			hasOperations = true
			operationsValue = value
		}
	}
	// A template without operations is the legacy form: every top-level key is
	// a literal request parameter set by the runtime. Keep it valid so older
	// channel configurations continue to work.
	if !hasOperations {
		for key := range template {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("channel affinity rule %q: template contains an empty legacy field", ruleName)
			}
		}
		return nil
	}
	if len(template) != 1 {
		return fmt.Errorf("channel affinity rule %q: operations templates cannot contain legacy top-level fields", ruleName)
	}

	operations, ok := normalizeChannelAffinityOperations(operationsValue)
	if !ok {
		return fmt.Errorf("channel affinity rule %q: template.operations must be an array", ruleName)
	}
	for index, operation := range operations {
		if err := validateChannelAffinityOperation(ruleName, index, operation); err != nil {
			return err
		}
	}
	return nil
}

func validateChannelAffinityOperation(ruleName string, index int, operation map[string]interface{}) error {
	for key := range operation {
		if _, allowed := channelAffinityOperationFields[key]; !allowed {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: field %q is not allowed", ruleName, index, key)
		}
	}

	modeRaw, ok := operation["mode"].(string)
	if !ok {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: mode is required", ruleName, index)
	}
	mode := strings.TrimSpace(modeRaw)
	if mode == "" {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: mode is required", ruleName, index)
	}
	if mode != strings.ToLower(mode) || mode != modeRaw {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: mode must be lowercase without surrounding whitespace", ruleName, index)
	}
	if _, supported := channelAffinityOperationModes[mode]; !supported {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: unsupported mode %q", ruleName, index, mode)
	}

	if logic, exists := operation["logic"]; exists {
		logicValue, ok := logic.(string)
		if !ok {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: logic must be AND or OR", ruleName, index)
		}
		logicValue = strings.ToUpper(strings.TrimSpace(logicValue))
		if logicValue != "AND" && logicValue != "OR" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: logic must be AND or OR", ruleName, index)
		}
	}
	if conditions, exists := operation["conditions"]; exists {
		if err := validateChannelAffinityConditions(ruleName, index, conditions); err != nil {
			return err
		}
	}

	path := channelAffinityStringField(operation, "path")
	from := channelAffinityStringField(operation, "from")
	to := channelAffinityStringField(operation, "to")
	if _, pathRequired := channelAffinityPathModes[mode]; pathRequired && path == "" {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: path is required for %s", ruleName, index, mode)
	}

	switch mode {
	case "move", "copy", "sync_fields":
		if from == "" || to == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: from and to are required for %s", ruleName, index, mode)
		}
	case "replace", "regex_replace":
		if from == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: from is required for %s", ruleName, index, mode)
		}
		if mode == "regex_replace" {
			if _, err := regexp.Compile(from); err != nil {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: regex pattern is invalid: %w", ruleName, index, err)
			}
		}
	case "trim_prefix", "trim_suffix", "ensure_prefix", "ensure_suffix":
		if value, exists := operation["value"]; !exists || value == nil || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: value is required for %s", ruleName, index, mode)
		}
	case "set", "prepend", "append":
		if _, exists := operation["value"]; !exists {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: value is required for %s", ruleName, index, mode)
		}
	case "return_error":
		if err := validateChannelAffinityReturnErrorValue(ruleName, index, operation["value"]); err != nil {
			return err
		}
	case "prune_objects":
		if err := validateChannelAffinityPruneValue(ruleName, index, operation["value"]); err != nil {
			return err
		}
	case "set_header":
		if path == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: path is required for set_header", ruleName, index)
		}
		if value, exists := operation["value"]; !exists || value == nil {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: value is required for set_header", ruleName, index)
		}
	case "delete_header":
		if path == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: path is required for delete_header", ruleName, index)
		}
	case "copy_header", "move_header":
		if (from == "" || to == "") && path == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: path or from/to is required for %s", ruleName, index, mode)
		}
	case "pass_headers":
		if value, exists := operation["value"]; !exists {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: value is required for pass_headers", ruleName, index)
		} else if err := validateChannelAffinityHeaderValue(ruleName, index, value); err != nil {
			return err
		}
	}
	return nil
}

func channelAffinityStringField(operation map[string]interface{}, key string) string {
	value, ok := operation[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func validateChannelAffinityHeaderValue(ruleName string, index int, value interface{}) error {
	switch raw := value.(type) {
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers value cannot be empty", ruleName, index)
		}
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
			var parsed interface{}
			if err := common.UnmarshalJsonStr(trimmed, &parsed); err == nil {
				return validateChannelAffinityHeaderValue(ruleName, index, parsed)
			}
		}
	case []string:
		return validateChannelAffinityHeaderNames(ruleName, index, raw)
	case []interface{}:
		headers := make([]string, 0, len(raw))
		for _, item := range raw {
			header, ok := item.(string)
			if !ok {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers headers must be strings", ruleName, index)
			}
			headers = append(headers, header)
		}
		return validateChannelAffinityHeaderNames(ruleName, index, headers)
	case map[string]interface{}:
		for _, key := range []string{"headers", "names", "header"} {
			if nested, exists := raw[key]; exists {
				return validateChannelAffinityHeaderValue(ruleName, index, nested)
			}
		}
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers object must contain headers, names, or header", ruleName, index)
	default:
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers value must be a string, array, or object", ruleName, index)
	}
	return nil
}

func validateChannelAffinityHeaderNames(ruleName string, index int, headers []string) error {
	if len(headers) == 0 {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers value must be non-empty", ruleName, index)
	}
	for _, header := range headers {
		if strings.TrimSpace(header) == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: pass_headers contains an empty header", ruleName, index)
		}
	}
	return nil
}

func validateChannelAffinityConditions(ruleName string, index int, value interface{}) error {
	if value == nil {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: conditions cannot be null", ruleName, index)
	}
	if object, ok := value.(map[string]interface{}); ok {
		if len(object) == 0 {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: conditions cannot be empty", ruleName, index)
		}
		return nil
	}
	items, ok := value.([]interface{})
	if !ok {
		if typed, typedOK := value.([]map[string]interface{}); typedOK {
			items = make([]interface{}, len(typed))
			for i := range typed {
				items[i] = typed[i]
			}
			ok = true
		}
	}
	if !ok || len(items) == 0 {
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: conditions must be a non-empty array or object", ruleName, index)
	}
	for conditionIndex, raw := range items {
		condition, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("channel affinity rule %q template.operations[%d].conditions[%d]: condition must be an object", ruleName, index, conditionIndex)
		}
		for key := range condition {
			switch key {
			case "path", "mode", "value", "invert", "pass_missing_key":
			default:
				return fmt.Errorf("channel affinity rule %q template.operations[%d].conditions[%d]: field %q is not allowed", ruleName, index, conditionIndex, key)
			}
		}
		if channelAffinityStringField(condition, "path") == "" || channelAffinityStringField(condition, "mode") == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d].conditions[%d]: path and mode are required", ruleName, index, conditionIndex)
		}
		for _, key := range []string{"invert", "pass_missing_key"} {
			if rawValue, exists := condition[key]; exists {
				if _, ok := rawValue.(bool); !ok {
					return fmt.Errorf("channel affinity rule %q template.operations[%d].conditions[%d]: %s must be boolean", ruleName, index, conditionIndex, key)
				}
			}
		}
		switch channelAffinityStringField(condition, "mode") {
		case "full", "prefix", "suffix", "contains", "gt", "gte", "lt", "lte":
		default:
			return fmt.Errorf("channel affinity rule %q template.operations[%d].conditions[%d]: unsupported mode", ruleName, index, conditionIndex)
		}
	}
	return nil
}

func validateChannelAffinityReturnErrorValue(ruleName string, index int, value interface{}) error {
	switch raw := value.(type) {
	case string:
		if strings.TrimSpace(raw) == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: return_error message is required", ruleName, index)
		}
	case map[string]interface{}:
		message, _ := raw["message"].(string)
		if strings.TrimSpace(message) == "" {
			message, _ = raw["msg"].(string)
		}
		if strings.TrimSpace(message) == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: return_error message is required", ruleName, index)
		}
		for _, key := range []string{"status_code", "status"} {
			if rawStatus, exists := raw[key]; exists {
				status, ok := channelAffinityInteger(rawStatus)
				if !ok || status < 100 || status > 511 {
					return fmt.Errorf("channel affinity rule %q template.operations[%d]: return_error %s must be an integer between 100 and 511", ruleName, index, key)
				}
			}
		}
	default:
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: return_error value must be a string or object", ruleName, index)
	}
	return nil
}

func validateChannelAffinityPruneValue(ruleName string, index int, value interface{}) error {
	switch raw := value.(type) {
	case string:
		if strings.TrimSpace(raw) == "" {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects value is required", ruleName, index)
		}
	case map[string]interface{}:
		for key := range raw {
			switch key {
			case "logic", "recursive", "conditions", "where", "type":
			default:
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects field %q is not allowed", ruleName, index, key)
			}
		}
		if logic, exists := raw["logic"]; exists {
			logicValue, ok := logic.(string)
			if !ok {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects logic must be AND or OR", ruleName, index)
			}
			logicValue = strings.ToUpper(strings.TrimSpace(logicValue))
			if logicValue != "AND" && logicValue != "OR" {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects logic must be AND or OR", ruleName, index)
			}
		}
		if recursive, exists := raw["recursive"]; exists {
			if _, ok := recursive.(bool); !ok {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects recursive must be boolean", ruleName, index)
			}
		}
		conditionSources := 0
		if conditions, exists := raw["conditions"]; exists {
			if err := validateChannelAffinityConditions(ruleName, index, conditions); err != nil {
				return err
			}
			conditionSources++
		}
		if where, exists := raw["where"]; exists {
			whereMap, ok := where.(map[string]interface{})
			if !ok || len(whereMap) == 0 {
				return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects where must be a non-empty object", ruleName, index)
			}
			for key := range whereMap {
				if strings.TrimSpace(key) == "" {
					return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects where contains an empty path", ruleName, index)
				}
			}
			conditionSources++
		}
		if _, exists := raw["type"]; exists {
			conditionSources++
		}
		if conditionSources == 0 {
			return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects conditions are required", ruleName, index)
		}
	default:
		return fmt.Errorf("channel affinity rule %q template.operations[%d]: prune_objects value must be a string or object", ruleName, index)
	}
	return nil
}

func channelAffinityInteger(value interface{}) (int, bool) {
	switch raw := value.(type) {
	case int:
		return raw, true
	case int8:
		return int(raw), true
	case int16:
		return int(raw), true
	case int32:
		return int(raw), true
	case int64:
		return int(raw), int64(int(raw)) == raw
	case uint:
		return int(raw), uint(int(raw)) == raw
	case uint8:
		return int(raw), true
	case uint16:
		return int(raw), uint16(int(raw)) == raw
	case uint32:
		return int(raw), uint32(int(raw)) == raw
	case uint64:
		return int(raw), uint64(int(raw)) == raw
	case float64:
		if raw != float64(int(raw)) {
			return 0, false
		}
		return int(raw), true
	default:
		return 0, false
	}
}

func normalizeChannelAffinityOperations(value interface{}) ([]map[string]interface{}, bool) {
	switch operations := value.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(operations))
		for _, raw := range operations {
			operation, ok := raw.(map[string]interface{})
			if !ok {
				return nil, false
			}
			result = append(result, operation)
		}
		return result, true
	case []map[string]interface{}:
		return operations, true
	default:
		return nil, false
	}
}
