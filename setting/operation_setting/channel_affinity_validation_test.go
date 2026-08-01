package operation_setting

import "testing"

func TestValidateChannelAffinityRulesRejectsInvalidRegexAndDuplicateNames(t *testing.T) {
	valid := ChannelAffinityRule{
		Name:       "conversation",
		ModelRegex: []string{"^gpt-"},
		KeySources: []ChannelAffinityKeySource{{Type: "gjson", Path: "prompt_cache_key"}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{valid}); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}

	invalid := valid
	invalid.ModelRegex = []string{"(?<=gpt-)4"}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{invalid}); err == nil {
		t.Fatal("expected Go-incompatible regex to be rejected")
	}

	duplicate := valid
	duplicate.ModelRegex = []string{"^claude-"}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{valid, duplicate}); err == nil {
		t.Fatal("expected duplicate rule names to be rejected")
	}

	whitespaceName := valid
	whitespaceName.Name = " conversation "
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{whitespaceName}); err == nil {
		t.Fatal("expected rule name surrounding whitespace to be rejected")
	}

	uppercaseSource := valid
	uppercaseSource.KeySources = []ChannelAffinityKeySource{{Type: "GJSON", Path: "prompt_cache_key"}}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{uppercaseSource}); err == nil {
		t.Fatal("expected non-canonical key source type to be rejected")
	}
}

func TestValidateChannelAffinityRulesAcceptsRuntimeSupportedTemplates(t *testing.T) {
	rule := ChannelAffinityRule{
		Name:       "headers",
		ModelRegex: []string{"^gpt-"},
		KeySources: []ChannelAffinityKeySource{{Type: "request_header", Key: "Originator"}},
		ParamOverrideTemplate: map[string]interface{}{
			"operations": []map[string]interface{}{{
				"mode":  "pass_headers",
				"value": []string{"Originator", "User-Agent"},
			}},
		},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid pass-through template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode":  "set",
			"path":  "temperature",
			"value": 0.2,
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid body mutation template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode": "unsupported",
			"path": "temperature",
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err == nil {
		t.Fatal("expected unsupported operation mode to be rejected")
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"temperature": 0.2,
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid legacy template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("empty operations compatibility template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode": "prune_objects",
			"value": map[string]interface{}{
				"where": map[string]interface{}{"type": "redacted_thinking"},
			},
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid root prune template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode":  "set",
			"path":  "temperature",
			"value": 0.2,
			"conditions": []map[string]interface{}{{
				"path":             "retry.is_retry",
				"mode":             "full",
				"value":            true,
				"invert":           true,
				"pass_missing_key": true,
			}},
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid conditional template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode": "return_error",
			"value": map[string]interface{}{
				"message":     "blocked",
				"status_code": 422,
			},
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err != nil {
		t.Fatalf("valid return_error template rejected: %v", err)
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode":  "pass_headers",
			"value": "[]",
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err == nil {
		t.Fatal("expected empty JSON pass_headers string to be rejected")
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode":  "set",
			"path":  "temperature",
			"value": 0.2,
		}},
		"temperature": 0.3,
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err == nil {
		t.Fatal("expected mixed legacy and operations template to be rejected")
	}

	rule.ParamOverrideTemplate = map[string]interface{}{
		"operations": []map[string]interface{}{{
			"mode": "return_error",
			"value": map[string]interface{}{
				"message":     "blocked",
				"status_code": 99,
			},
		}},
	}
	if err := ValidateChannelAffinityRules([]ChannelAffinityRule{rule}); err == nil {
		t.Fatal("expected invalid return_error status code to be rejected")
	}
}
