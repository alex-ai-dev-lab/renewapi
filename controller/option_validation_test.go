package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
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

func TestGetOptionsMasksAntiPoisonAuditSecretAndReportsConfiguredState(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		antiPoisonAuditEnabledKey: "true",
		antiPoisonAuditSecretKey:  "wave2-test-secret",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetOptions(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET options status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode options response: %v", err)
	}
	if !response.Success {
		t.Fatal("GET options should succeed")
	}

	configured := false
	for _, option := range response.Data {
		if option.Key == antiPoisonAuditSecretKey {
			t.Fatal("secret option must never be returned to the client")
		}
		if option.Value == "wave2-test-secret" {
			t.Fatal("secret value must never be returned to the client")
		}
		if option.Key == "anti_poison_setting.signed_header_audit_secret_configured" {
			configured = option.Value == "true"
		}
	}
	if !configured {
		t.Fatal("configured-state option should report true when a secret exists")
	}
}

func mustMarshalChannelAffinityRules(t *testing.T, rules []operation_setting.ChannelAffinityRule) string {
	t.Helper()
	raw, err := common.Marshal(rules)
	if err != nil {
		t.Fatalf("marshal Channel Affinity rules: %v", err)
	}
	return string(raw)
}
