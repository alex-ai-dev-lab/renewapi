package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

const (
	antiPoisonAuditEnabledKey = "anti_poison_setting.signed_header_audit_enabled"
	antiPoisonAuditSecretKey  = "anti_poison_setting.signed_header_audit_secret"
)

// Validate the effective pair instead of validating each field in isolation.
// Bulk updates may enable the audit and supply its secret in one request.
func validateAntiPoisonAuditOptions(values map[string]string) error {
	setting := operation_setting.GetAntiPoisonSetting()
	enabled := setting.SignedHeaderAuditEnabled
	secret := strings.TrimSpace(setting.SignedHeaderAuditSecret)
	if value, ok := values[antiPoisonAuditEnabledKey]; ok {
		enabled = strings.EqualFold(strings.TrimSpace(value), "true")
	}
	if value, ok := values[antiPoisonAuditSecretKey]; ok {
		secret = strings.TrimSpace(value)
	}
	if enabled && secret == "" {
		return fmt.Errorf("反投毒签名审计开启时必须配置密钥")
	}
	return nil
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func isOneOfOptionValue(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func isHexColorOptionValue(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func isCSVSubsetOptionValue(value string, allowed ...string) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	seen := make(map[string]struct{}, len(allowed))
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := allowedSet[trimmed]; !ok {
			return false
		}
		seen[trimmed] = struct{}{}
	}
	return len(seen) > 0
}

func isFloatInRangeOptionValue(value string, minValue float64, maxValue float64) bool {
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue >= minValue && floatValue <= maxValue
}

func isSystemSettingsNavigationOptionValue(value string) bool {
	type areaConfig struct {
		Enabled  *bool           `json:"enabled"`
		Order    []string        `json:"order"`
		Sections map[string]bool `json:"sections"`
	}
	type navigationConfig struct {
		Order []string              `json:"order"`
		Areas map[string]areaConfig `json:"areas"`
	}

	allowedSections := map[string]map[string]struct{}{
		"site": {
			"system-info": {}, "notice": {}, "header-navigation": {},
			"sidebar-modules": {}, "settings-navigation": {},
		},
		"auth": {
			"basic-auth": {}, "oauth": {}, "passkey": {},
			"bot-protection": {}, "custom-oauth": {},
		},
		"billing": {
			"quota": {}, "currency": {}, "checkin": {},
			"payment": {}, "model-pricing": {}, "group-pricing": {},
		},
		"models": {
			"overview": {}, "global": {}, "gemini": {}, "claude": {},
			"grok": {}, "user-agents": {}, "client-identity": {},
			"model-pricing": {}, "channel-affinity": {}, "model-deployment": {},
		},
		"security": {
			"rate-limit": {}, "sensitive-words": {}, "ssrf": {},
			"upstream-error-rules": {}, "anti-poison-guard": {}, "request-guard": {},
		},
		"content": {
			"dashboard": {}, "appearance": {}, "announcements": {},
			"api-info": {}, "faq": {}, "uptime-kuma": {}, "chat": {},
			"drawing": {},
		},
		"operations": {
			"overview": {}, "behavior": {}, "monitoring": {}, "email": {},
			"worker": {}, "logs": {}, "performance": {}, "update-checker": {},
		},
	}

	var cfg navigationConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return false
	}
	if len(cfg.Order) == 0 && len(cfg.Areas) == 0 {
		return false
	}
	for _, area := range cfg.Order {
		if _, ok := allowedSections[area]; !ok {
			return false
		}
	}
	for area, areaCfg := range cfg.Areas {
		sections, ok := allowedSections[area]
		if !ok {
			return false
		}
		for _, section := range areaCfg.Order {
			if _, ok := sections[section]; !ok {
				return false
			}
		}
		for section := range areaCfg.Sections {
			if _, ok := sections[section]; !ok {
				return false
			}
		}
	}
	return true
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey {
			if k == antiPoisonAuditSecretKey {
				options = append(options, &model.Option{
					Key:   "anti_poison_setting.signed_header_audit_secret_configured",
					Value: strconv.FormatBool(strings.TrimSpace(value) != ""),
				})
			}
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OptionsBulkUpdateRequest struct {
	Options map[string]any `json:"options"`
}

func optionValueToString(value any) string {
	switch typed := value.(type) {
	case bool:
		return common.Interface2String(typed)
	case float64:
		return common.Interface2String(typed)
	case int:
		return common.Interface2String(typed)
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &option); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	key := strings.TrimSpace(option.Key)
	value := optionValueToString(option.Value)
	values, err := normalizeOptionValues(map[string]string{key: value})
	if err != nil {
		writeOptionValidationError(c, err)
		return
	}
	if err := validateOptionValues(values); err != nil {
		writeOptionValidationError(c, err)
		return
	}
	if err := model.UpdateOption(key, values[key]); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateOptionsBulk(c *gin.Context) {
	var request OptionsBulkUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || len(request.Options) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	values := make(map[string]string, len(request.Options))
	for key, rawValue := range request.Options {
		values[key] = optionValueToString(rawValue)
	}
	normalizedValues, err := normalizeOptionValues(values)
	if err != nil {
		writeOptionValidationError(c, err)
		return
	}
	values = normalizedValues
	if err := validateOptionValues(values); err != nil {
		writeOptionValidationError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
