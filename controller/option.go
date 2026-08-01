package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

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
			"upstream-error-rules": {}, "anti-poison-guard": {},
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
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	option.Value = optionValueToString(option.Value)
	switch option.Key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value.(string)) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
	default:
		if isPaymentComplianceOptionKey(option.Key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "theme.frontend":
		if option.Value != "default" && option.Value != "classic" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的主题值，可选值：default（新版前端）、classic（经典前端）",
			})
			return
		}
	case "theme.customization_preset":
		if !isOneOfOptionValue(option.Value.(string), "default", "anthropic", "simple-large", "underground", "rose-garden", "lake-view", "sunset-glow", "forest-whisper", "ocean-breeze", "lavender-dream") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的主题预设值",
			})
			return
		}
	case "theme.customization_font":
		if !isOneOfOptionValue(option.Value.(string), "default", "sans", "serif") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的字体值",
			})
			return
		}
	case "theme.customization_radius":
		if !isOneOfOptionValue(option.Value.(string), "default", "none", "sm", "md", "lg", "xl") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的圆角值",
			})
			return
		}
	case "theme.customization_scale":
		if !isOneOfOptionValue(option.Value.(string), "default", "sm", "lg", "xl") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的密度值",
			})
			return
		}
	case "theme.content_layout":
		if !isOneOfOptionValue(option.Value.(string), "full", "centered") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的内容布局值",
			})
			return
		}
	case "theme.custom_accent_enabled":
		if !isOneOfOptionValue(option.Value.(string), "true", "false") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的自定义强调色开关",
			})
			return
		}
	case "theme.custom_accent_color":
		if !isHexColorOptionValue(option.Value.(string)) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的自定义强调色，请使用 #RRGGBB 格式",
			})
			return
		}
	case "theme.custom_palette_enabled":
		if !isOneOfOptionValue(option.Value.(string), "true", "false") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的自定义界面调色板开关",
			})
			return
		}
	case "theme.custom_background_color", "theme.custom_surface_color", "theme.custom_sidebar_color", "theme.custom_chart_color":
		if !isHexColorOptionValue(option.Value.(string)) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的自定义界面颜色，请使用 #RRGGBB 格式",
			})
			return
		}
	case "DashboardDefaultTimeRange":
		if !isOneOfOptionValue(option.Value.(string), "1d", "7d", "30d", "1y", "all") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计默认时间范围",
			})
			return
		}
	case "DashboardRefreshIntervalSeconds":
		if !isOneOfOptionValue(option.Value.(string), "5", "15", "30", "60") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计刷新间隔",
			})
			return
		}
	case "DashboardDefaultPageSize":
		if !isOneOfOptionValue(option.Value.(string), "10", "25", "50", "100") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计表格页大小",
			})
			return
		}
	case "DashboardDefaultHealthFilter":
		if !isOneOfOptionValue(option.Value.(string), "all", "active", "risk", "slow") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计默认健康筛选",
			})
			return
		}
	case "DashboardDefaultTrendMode":
		if !isOneOfOptionValue(option.Value.(string), "overview", "traffic", "reliability", "latency", "spend") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计默认趋势模式",
			})
			return
		}
	case "DashboardDefaultChartTimeRangeDays":
		if !isOneOfOptionValue(option.Value.(string), "1", "7", "14", "29") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计图表默认时间窗口",
			})
			return
		}
	case "DashboardDefaultConsumptionChart":
		if !isOneOfOptionValue(option.Value.(string), "bar", "area") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的消费分布图默认类型",
			})
			return
		}
	case "DashboardDefaultModelAnalyticsChart":
		if !isOneOfOptionValue(option.Value.(string), "trend", "proportion", "top") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的模型分析图默认类型",
			})
			return
		}
	case "DashboardVisibleSections":
		if !isCSVSubsetOptionValue(option.Value.(string), "overview", "models", "channels", "users") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的 Dashboard 可见分区",
			})
			return
		}
	case "SidebarSectionOrder":
		if !isCSVSubsetOptionValue(option.Value.(string), "chat", "console", "personal", "admin") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的侧边栏分组顺序",
			})
			return
		}
	case "SystemSettingsNavigation":
		if !isSystemSettingsNavigationOptionValue(option.Value.(string)) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的系统设置导航配置",
			})
			return
		}
	case "DashboardSlowFirstTokenThresholdMs":
		if !isFloatInRangeOptionValue(option.Value.(string), 100, 120000) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的慢首字阈值",
			})
			return
		}
	case "DashboardErrorRateWarningThreshold", "DashboardErrorRateCriticalThreshold", "DashboardSuccessRateGoodThreshold", "DashboardSuccessRateDegradedThreshold":
		if !isFloatInRangeOptionValue(option.Value.(string), 0, 100) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的统计健康阈值",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if err = validateAntiPoisonAuditOptions(map[string]string{
		option.Key: option.Value.(string),
	}); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
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
		key = strings.TrimSpace(key)
		if key == "" {
			common.ApiErrorMsg(c, "设置项不能为空")
			return
		}
		value := optionValueToString(rawValue)
		if isPaymentComplianceOptionKey(key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
		if (key == "QuotaForInviter" || key == "QuotaForInvitee") && isPositiveOptionValue(value) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
		values[key] = value
	}
	if err := validateAntiPoisonAuditOptions(values); err != nil {
		common.ApiErrorMsg(c, err.Error())
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
