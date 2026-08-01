package controller

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var errPaymentComplianceRequired = errors.New("payment compliance confirmation is required")

func writeOptionValidationError(c *gin.Context, err error) {
	if errors.Is(err, errPaymentComplianceRequired) {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}
	common.ApiErrorMsg(c, err.Error())
}

func effectiveOptionValue(values map[string]string, key, current string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return current
}

func effectiveOptionBool(values map[string]string, key string, current bool) bool {
	return strings.EqualFold(strings.TrimSpace(effectiveOptionValue(values, key, fmt.Sprintf("%t", current))), "true")
}

func effectiveOptionConfigured(values map[string]string, key, current string) bool {
	return strings.TrimSpace(effectiveOptionValue(values, key, current)) != ""
}

func optionCSVHasValue(value string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
}

func normalizeOptionValues(values map[string]string) (map[string]string, error) {
	normalized := make(map[string]string, len(values))
	for rawKey, value := range values {
		key := strings.TrimSpace(rawKey)
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("设置项重复: %s", key)
		}
		if key == "ServerAddress" {
			var err error
			value, err = normalizeServerAddress(value)
			if err != nil {
				return nil, err
			}
		}
		normalized[key] = value
	}
	return normalized, nil
}

func normalizeServerAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("服务器地址必须是完整的 http:// 或 https:// URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("服务器地址不能包含用户信息、查询参数或片段")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed.String(), nil
}

func validateJSONFloatMap(value string) error {
	var parsed map[string]float64
	return common.UnmarshalJsonStr(value, &parsed)
}

func invalidOptionValue(message string) error {
	return errors.New(message)
}

func effectiveOptionInt(values map[string]string, key string, current int) (int, error) {
	raw := effectiveOptionValue(values, key, strconv.Itoa(current))
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func validateModelRateLimitOptions(values map[string]string) error {
	hasRateLimitOption := false
	for _, key := range []string{
		"ModelRequestRateLimitEnabled",
		"ModelRequestRateLimitDurationMinutes",
		"ModelRequestRateLimitCount",
		"ModelRequestRateLimitSuccessCount",
	} {
		if _, ok := values[key]; ok {
			hasRateLimitOption = true
			break
		}
	}
	if !hasRateLimitOption {
		return nil
	}

	duration, err := effectiveOptionInt(values, "ModelRequestRateLimitDurationMinutes", setting.ModelRequestRateLimitDurationMinutes)
	if err != nil {
		return err
	}
	total, err := effectiveOptionInt(values, "ModelRequestRateLimitCount", setting.ModelRequestRateLimitCount)
	if err != nil {
		return err
	}
	success, err := effectiveOptionInt(values, "ModelRequestRateLimitSuccessCount", setting.ModelRequestRateLimitSuccessCount)
	if err != nil {
		return err
	}
	return setting.ValidateModelRequestRateLimitSettings(duration, total, success)
}

func hasAnyOption(values map[string]string, keys ...string) bool {
	for _, key := range keys {
		if _, ok := values[key]; ok {
			return true
		}
	}
	return false
}

// validateEnabledOptionRequirements runs for every write touching a related
// option, not only when the enabled flag itself changes. Otherwise clearing a
// credential while the feature is already enabled would leave an invalid
// runtime configuration behind.
func validateEnabledOptionRequirements(values map[string]string) error {
	if hasAnyOption(values, "GitHubOAuthEnabled", "GitHubClientId", "GitHubClientSecret") &&
		effectiveOptionBool(values, "GitHubOAuthEnabled", common.GitHubOAuthEnabled) &&
		(!effectiveOptionConfigured(values, "GitHubClientId", common.GitHubClientId) ||
			!effectiveOptionConfigured(values, "GitHubClientSecret", common.GitHubClientSecret)) {
		return errors.New("无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！")
	}
	if hasAnyOption(values, "discord.enabled", "discord.client_id", "discord.client_secret") {
		discord := system_setting.GetDiscordSettings()
		if effectiveOptionBool(values, "discord.enabled", discord.Enabled) &&
			(!effectiveOptionConfigured(values, "discord.client_id", discord.ClientId) ||
				!effectiveOptionConfigured(values, "discord.client_secret", discord.ClientSecret)) {
			return errors.New("无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！")
		}
	}
	if hasAnyOption(values, "oidc.enabled", "oidc.client_id", "oidc.client_secret") {
		oidc := system_setting.GetOIDCSettings()
		if effectiveOptionBool(values, "oidc.enabled", oidc.Enabled) &&
			(!effectiveOptionConfigured(values, "oidc.client_id", oidc.ClientId) ||
				!effectiveOptionConfigured(values, "oidc.client_secret", oidc.ClientSecret)) {
			return errors.New("无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！")
		}
	}
	if hasAnyOption(values, "LinuxDOOAuthEnabled", "LinuxDOClientId", "LinuxDOClientSecret") &&
		effectiveOptionBool(values, "LinuxDOOAuthEnabled", common.LinuxDOOAuthEnabled) &&
		(!effectiveOptionConfigured(values, "LinuxDOClientId", common.LinuxDOClientId) ||
			!effectiveOptionConfigured(values, "LinuxDOClientSecret", common.LinuxDOClientSecret)) {
		return errors.New("无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！")
	}
	if hasAnyOption(values, "EmailDomainRestrictionEnabled", "EmailDomainWhitelist") &&
		effectiveOptionBool(values, "EmailDomainRestrictionEnabled", common.EmailDomainRestrictionEnabled) &&
		!optionCSVHasValue(effectiveOptionValue(values, "EmailDomainWhitelist", strings.Join(common.EmailDomainWhitelist, ","))) {
		return errors.New("无法启用邮箱域名限制，请先填入限制的邮箱域名！")
	}
	if hasAnyOption(values, "WeChatAuthEnabled", "WeChatServerAddress") &&
		effectiveOptionBool(values, "WeChatAuthEnabled", common.WeChatAuthEnabled) &&
		!effectiveOptionConfigured(values, "WeChatServerAddress", common.WeChatServerAddress) {
		return errors.New("无法启用微信登录，请先填入微信登录相关配置信息！")
	}
	if hasAnyOption(values, "TurnstileCheckEnabled", "TurnstileSiteKey", "TurnstileSecretKey") &&
		effectiveOptionBool(values, "TurnstileCheckEnabled", common.TurnstileCheckEnabled) &&
		(!effectiveOptionConfigured(values, "TurnstileSiteKey", common.TurnstileSiteKey) ||
			!effectiveOptionConfigured(values, "TurnstileSecretKey", common.TurnstileSecretKey)) {
		return errors.New("无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！")
	}
	if hasAnyOption(values, "TelegramOAuthEnabled", "TelegramBotToken") &&
		effectiveOptionBool(values, "TelegramOAuthEnabled", common.TelegramOAuthEnabled) &&
		!effectiveOptionConfigured(values, "TelegramBotToken", common.TelegramBotToken) {
		return errors.New("无法启用 Telegram OAuth，请先填入 Telegram Bot Token！")
	}
	return nil
}

func validateCheckinOptions(values map[string]string) error {
	if _, minPresent := values["checkin_setting.min_quota"]; !minPresent {
		if _, maxPresent := values["checkin_setting.max_quota"]; !maxPresent {
			return nil
		}
	}
	current := operation_setting.GetCheckinSetting()
	minQuota, err := effectiveOptionInt(values, "checkin_setting.min_quota", current.MinQuota)
	if err != nil {
		return err
	}
	maxQuota, err := effectiveOptionInt(values, "checkin_setting.max_quota", current.MaxQuota)
	if err != nil {
		return err
	}
	if minQuota < 0 || maxQuota < 0 {
		return errors.New("签到额度不能为负数")
	}
	if maxQuota < minQuota {
		return errors.New("签到最大额度不能小于最小额度")
	}
	return nil
}

func validateChannelAffinityOptions(values map[string]string) error {
	if rawRules, ok := values["channel_affinity_setting.rules"]; ok {
		if err := operation_setting.ValidateChannelAffinityRulesJSON(rawRules); err != nil {
			return err
		}
	}
	if rawMax, ok := values["channel_affinity_setting.max_entries"]; ok {
		maxEntries, err := strconv.Atoi(strings.TrimSpace(rawMax))
		if err != nil || maxEntries < 0 {
			return errors.New("Channel Affinity 最大缓存条目数必须是非负整数")
		}
	}
	if rawTTL, ok := values["channel_affinity_setting.default_ttl_seconds"]; ok {
		ttl, err := strconv.Atoi(strings.TrimSpace(rawTTL))
		if err != nil || ttl < 0 {
			return errors.New("Channel Affinity 默认 TTL 必须是非负整数")
		}
	}
	return nil
}

func isValidPasskeyRPID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "localhost" || net.ParseIP(value) != nil {
		return true
	}
	if value == "" || strings.ContainsAny(value, "/:@?#[]") {
		return false
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return len(value) <= 253
}

func splitPasskeyOrigins(value string) []string {
	return strings.FieldsFunc(value, func(char rune) bool { return char == ',' || char == '\n' || char == '\r' })
}

func validatePasskeyOptions(values map[string]string) error {
	if !hasAnyOption(values,
		"passkey.enabled",
		"passkey.rp_id",
		"passkey.origins",
		"passkey.allow_insecure_origin",
		"passkey.user_verification",
		"passkey.attachment_preference",
	) {
		return nil
	}
	current := system_setting.GetPasskeySettingsSnapshot()
	enabled := effectiveOptionBool(values, "passkey.enabled", current.Enabled)
	if !enabled {
		return nil
	}
	rpID := strings.TrimSpace(effectiveOptionValue(values, "passkey.rp_id", current.RPID))
	if !isValidPasskeyRPID(rpID) {
		return errors.New("Passkey RP ID 必须是裸域名、localhost 或 IP，不能包含协议、端口或路径")
	}
	origins := splitPasskeyOrigins(effectiveOptionValue(values, "passkey.origins", current.Origins))
	if len(origins) == 0 {
		return errors.New("启用 Passkey 时至少需要配置一个 Allowed Origin")
	}
	allowInsecure := effectiveOptionBool(values, "passkey.allow_insecure_origin", current.AllowInsecureOrigin)
	for _, rawOrigin := range origins {
		origin := strings.TrimSpace(rawOrigin)
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("Passkey Allowed Origin 无效: %s", origin)
		}
		if parsed.Scheme == "http" && !allowInsecure {
			return fmt.Errorf("Passkey 不允许使用不安全 Origin: %s", origin)
		}
		host := strings.ToLower(parsed.Hostname())
		rpHost := strings.ToLower(rpID)
		if net.ParseIP(rpHost) != nil {
			if host != rpHost {
				return fmt.Errorf("Passkey Origin 必须属于 RP ID %s: %s", rpID, origin)
			}
		} else if host != rpHost && !strings.HasSuffix(host, "."+rpHost) {
			return fmt.Errorf("Passkey Origin 必须属于 RP ID %s: %s", rpID, origin)
		}
	}
	for key, allowed := range map[string][]string{
		"passkey.user_verification":     {"required", "preferred", "discouraged"},
		"passkey.attachment_preference": {"", "platform", "cross-platform"},
	} {
		value := effectiveOptionValue(values, key, "")
		if key == "passkey.user_verification" {
			value = effectiveOptionValue(values, key, current.UserVerification)
		} else {
			value = effectiveOptionValue(values, key, current.AttachmentPreference)
		}
		valid := false
		for _, candidate := range allowed {
			if value == candidate {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("Passkey %s 配置无效", key)
		}
	}
	return nil
}

// validateOptionValues is the single validation path for both single and bulk
// settings writes. It only reads current settings and validates the effective
// batch; it never mutates runtime state.
func validateOptionValues(values map[string]string) error {
	for key, value := range values {
		if strings.TrimSpace(key) == "" {
			return errors.New("设置项不能为空")
		}
		if isPaymentComplianceOptionKey(key) {
			return errors.New("合规确认字段不允许通过通用设置接口修改")
		}

		switch key {
		case "QuotaForInviter", "QuotaForInvitee":
			if isPositiveOptionValue(value) && !operation_setting.IsPaymentComplianceConfirmed() {
				return errPaymentComplianceRequired
			}
		case "theme.frontend":
			if !isOneOfOptionValue(value, "default", "classic") {
				return invalidOptionValue("无效的主题值，可选值：default（新版前端）、classic（经典前端）")
			}
		case "theme.customization_preset":
			if !isOneOfOptionValue(value, "default", "anthropic", "simple-large", "underground", "rose-garden", "lake-view", "sunset-glow", "forest-whisper", "ocean-breeze", "lavender-dream") {
				return invalidOptionValue("无效的主题预设值")
			}
		case "theme.customization_font":
			if !isOneOfOptionValue(value, "default", "sans", "serif") {
				return invalidOptionValue("无效的字体值")
			}
		case "theme.customization_radius":
			if !isOneOfOptionValue(value, "default", "none", "sm", "md", "lg", "xl") {
				return invalidOptionValue("无效的圆角值")
			}
		case "theme.customization_scale":
			if !isOneOfOptionValue(value, "default", "sm", "lg", "xl") {
				return invalidOptionValue("无效的密度值")
			}
		case "theme.content_layout":
			if !isOneOfOptionValue(value, "full", "centered") {
				return invalidOptionValue("无效的内容布局值")
			}
		case "theme.custom_accent_enabled", "theme.custom_palette_enabled":
			if !isOneOfOptionValue(value, "true", "false") {
				return invalidOptionValue("无效的自定义界面开关")
			}
		case "theme.custom_accent_color":
			if !isHexColorOptionValue(value) {
				return invalidOptionValue("无效的自定义强调色，请使用 #RRGGBB 格式")
			}
		case "theme.custom_background_color", "theme.custom_surface_color", "theme.custom_sidebar_color", "theme.custom_chart_color":
			if !isHexColorOptionValue(value) {
				return invalidOptionValue("无效的自定义界面颜色，请使用 #RRGGBB 格式")
			}
		case "DashboardDefaultTimeRange":
			if !isOneOfOptionValue(value, "1d", "7d", "30d", "1y", "all") {
				return invalidOptionValue("无效的统计默认时间范围")
			}
		case "DashboardRefreshIntervalSeconds":
			if !isOneOfOptionValue(value, "5", "15", "30", "60") {
				return invalidOptionValue("无效的统计刷新间隔")
			}
		case "DashboardDefaultPageSize":
			if !isOneOfOptionValue(value, "10", "25", "50", "100") {
				return invalidOptionValue("无效的统计表格页大小")
			}
		case "DashboardDefaultHealthFilter":
			if !isOneOfOptionValue(value, "all", "active", "risk", "slow") {
				return invalidOptionValue("无效的统计默认健康筛选")
			}
		case "DashboardDefaultTrendMode":
			if !isOneOfOptionValue(value, "overview", "traffic", "reliability", "latency", "spend") {
				return invalidOptionValue("无效的统计默认趋势模式")
			}
		case "DashboardDefaultChartTimeRangeDays":
			if !isOneOfOptionValue(value, "1", "7", "14", "29") {
				return invalidOptionValue("无效的统计图表默认时间窗口")
			}
		case "DashboardDefaultConsumptionChart":
			if !isOneOfOptionValue(value, "bar", "area") {
				return invalidOptionValue("无效的消费分布图默认类型")
			}
		case "DashboardDefaultModelAnalyticsChart":
			if !isOneOfOptionValue(value, "trend", "proportion", "top") {
				return invalidOptionValue("无效的模型分析图默认类型")
			}
		case "DashboardVisibleSections":
			if !isCSVSubsetOptionValue(value, "overview", "models", "channels", "users") {
				return invalidOptionValue("无效的 Dashboard 可见分区")
			}
		case "SidebarSectionOrder":
			if !isCSVSubsetOptionValue(value, "chat", "console", "personal", "admin") {
				return invalidOptionValue("无效的侧边栏分组顺序")
			}
		case "SystemSettingsNavigation":
			if !isSystemSettingsNavigationOptionValue(value) {
				return invalidOptionValue("无效的系统设置导航配置")
			}
		case "DashboardSlowFirstTokenThresholdMs":
			if !isFloatInRangeOptionValue(value, 100, 120000) {
				return invalidOptionValue("无效的慢首字阈值")
			}
		case "DashboardErrorRateWarningThreshold", "DashboardErrorRateCriticalThreshold", "DashboardSuccessRateGoodThreshold", "DashboardSuccessRateDegradedThreshold":
			if !isFloatInRangeOptionValue(value, 0, 100) {
				return invalidOptionValue("无效的统计健康阈值")
			}
		case "GroupRatio":
			if err := ratio_setting.CheckGroupRatio(value); err != nil {
				return err
			}
		case "ImageRatio":
			if err := validateJSONFloatMap(value); err != nil {
				return fmt.Errorf("图片倍率设置失败: %w", err)
			}
		case "AudioRatio":
			if err := validateJSONFloatMap(value); err != nil {
				return fmt.Errorf("音频倍率设置失败: %w", err)
			}
		case "AudioCompletionRatio":
			if err := validateJSONFloatMap(value); err != nil {
				return fmt.Errorf("音频补全倍率设置失败: %w", err)
			}
		case "CreateCacheRatio":
			if err := validateJSONFloatMap(value); err != nil {
				return fmt.Errorf("缓存创建倍率设置失败: %w", err)
			}
		case "ModelRequestRateLimitGroup":
			if err := setting.CheckModelRequestRateLimitGroup(value); err != nil {
				return err
			}
		case "AutomaticDisableStatusCodes", "AutomaticRetryStatusCodes":
			if _, err := operation_setting.ParseHTTPStatusCodeRanges(value); err != nil {
				return err
			}
		case "console_setting.api_info":
			if err := console_setting.ValidateConsoleSettings(value, "ApiInfo"); err != nil {
				return err
			}
		case "console_setting.announcements":
			if err := console_setting.ValidateConsoleSettings(value, "Announcements"); err != nil {
				return err
			}
		case "console_setting.faq":
			if err := console_setting.ValidateConsoleSettings(value, "FAQ"); err != nil {
				return err
			}
		case "console_setting.uptime_kuma_groups":
			if err := console_setting.ValidateConsoleSettings(value, "UptimeKumaGroups"); err != nil {
				return err
			}
		case "ServerAddress":
			if _, err := normalizeServerAddress(value); err != nil {
				return err
			}
		}
	}

	if err := validateEnabledOptionRequirements(values); err != nil {
		return err
	}
	if err := validateModelRateLimitOptions(values); err != nil {
		return err
	}
	if err := validateCheckinOptions(values); err != nil {
		return err
	}
	if err := validateChannelAffinityOptions(values); err != nil {
		return err
	}
	if err := validatePasskeyOptions(values); err != nil {
		return err
	}
	return validateAntiPoisonAuditOptions(values)
}
