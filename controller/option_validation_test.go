package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestValidateOptionValuesUsesEffectiveBulkCredentials(t *testing.T) {
	err := validateOptionValues(map[string]string{
		"GitHubOAuthEnabled": "true",
		"GitHubClientId":     "client-id",
		"GitHubClientSecret": "client-secret",
	})
	if err != nil {
		t.Fatalf("bulk OAuth credentials should be validated as one effective update: %v", err)
	}
}

func TestValidateOptionValuesRequiresBothTurnstileCredentials(t *testing.T) {
	err := validateOptionValues(map[string]string{
		"TurnstileCheckEnabled": "true",
		"TurnstileSiteKey":      "site-key",
	})
	if err == nil {
		t.Fatal("expected Turnstile secret key to be required")
	}

	err = validateOptionValues(map[string]string{
		"TurnstileCheckEnabled": "true",
		"TurnstileSiteKey":      "site-key",
		"TurnstileSecretKey":    "secret-key",
	})
	if err != nil {
		t.Fatalf("complete Turnstile bulk update should pass: %v", err)
	}
}

func TestValidateOptionValuesRejectsInvalidRatioWithoutMutation(t *testing.T) {
	if err := validateOptionValues(map[string]string{"ImageRatio": "{\"gpt-image-1\":}"}); err == nil {
		t.Fatal("expected malformed ratio JSON to be rejected")
	}
}

func TestValidateOptionValuesUsesEffectiveRateLimitScalars(t *testing.T) {
	err := validateOptionValues(map[string]string{
		"ModelRequestRateLimitDurationMinutes": "0",
		"ModelRequestRateLimitCount":           "10",
		"ModelRequestRateLimitSuccessCount":    "5",
	})
	if err == nil {
		t.Fatal("expected non-positive rate-limit duration to be rejected")
	}

	err = validateOptionValues(map[string]string{
		"ModelRequestRateLimitDurationMinutes": "5",
		"ModelRequestRateLimitCount":           "10",
		"ModelRequestRateLimitSuccessCount":    "5",
	})
	if err != nil {
		t.Fatalf("valid rate-limit bulk update rejected: %v", err)
	}
}

func TestNormalizeOptionValuesCanonicalizesServerAddressAndRejectsDuplicates(t *testing.T) {
	values, err := normalizeOptionValues(map[string]string{
		" ServerAddress ": "https://example.com/base///",
	})
	if err != nil {
		t.Fatalf("valid server address rejected: %v", err)
	}
	if got := values["ServerAddress"]; got != "https://example.com/base" {
		t.Fatalf("server address = %q, want canonical URL", got)
	}

	if _, err := normalizeOptionValues(map[string]string{
		"ServerAddress":   "https://example.com",
		" ServerAddress ": "https://other.example.com",
	}); err == nil {
		t.Fatal("expected duplicate normalized keys to be rejected")
	}
}

func TestValidateOptionValuesRejectsInvertedCheckinRange(t *testing.T) {
	err := validateOptionValues(map[string]string{
		"checkin_setting.min_quota": "100",
		"checkin_setting.max_quota": "10",
	})
	if err == nil {
		t.Fatal("expected inverted check-in range to be rejected")
	}
}

func TestValidateOptionValuesRejectsInvalidChannelAffinityRule(t *testing.T) {
	err := validateOptionValues(map[string]string{
		"channel_affinity_setting.rules": `[{"name":"broken","model_regex":["(?<=gpt-)4"],"key_sources":[{"type":"gjson","path":"prompt_cache_key"}]}]`,
	})
	if err == nil {
		t.Fatal("expected invalid Channel Affinity regex to be rejected")
	}

	valid := operation_setting.ChannelAffinityRule{
		Name:       "valid",
		ModelRegex: []string{"^gpt-"},
		KeySources: []operation_setting.ChannelAffinityKeySource{{Type: "gjson", Path: "prompt_cache_key"}},
	}
	if err := validateOptionValues(map[string]string{
		"channel_affinity_setting.rules": mustMarshalChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{valid}),
	}); err != nil {
		t.Fatalf("valid Channel Affinity rule rejected: %v", err)
	}
}

func mustMarshalChannelAffinityRules(t *testing.T, rules []operation_setting.ChannelAffinityRule) string {
	t.Helper()
	raw, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("marshal Channel Affinity rules: %v", err)
	}
	return string(raw)
}
