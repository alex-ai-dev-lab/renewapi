// Package errornorm normalizes upstream errors before they reach the client.
// This module implements compat.RelayHook.OnClientResponseError.
package errornorm

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/compat"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// Hook implements compat.RelayHook for upstream error normalization.
type Hook struct {
	compat.NoOpRelayHook
}

// New returns a new error normalization hook.
func New() *Hook {
	return &Hook{}
}

func (h *Hook) Name() string { return "errornorm" }

func (h *Hook) OnClientResponseError(c *gin.Context, info *relaycommon.RelayInfo, err *types.NewAPIError) *types.NewAPIError {
	if err == nil || types.IsLocalRelayRejection(err) || types.PublicUpstreamModelError(err) != nil || !shouldNormalize(info) {
		return err
	}
	status := err.StatusCode
	if status == 0 {
		status = http.StatusInternalServerError
	}
	original := err.MaskSensitiveErrorWithStatusCode()
	if original != "" {
		common.SysLog(fmt.Sprintf("normalized upstream error for client: channel=%d status=%d original=%s",
			channelIDForLog(info), status, common.LocalLogPreview(original)))
	}

	// DB rule first; fall back to built-in FixedMessage.
	if rule := matchRuleFromContext(info, status, original); rule != nil {
		applyRuleForRequest(c, err, rule, status)
		compat.GetMetrics().ErrorNormalizeHit.Add(1)
		return err
	}

	err.SetMessage(FixedMessageForRequest(c, status))
	compat.GetMetrics().ErrorNormalizeHit.Add(1)
	return err
}

// matchRuleFromContext queries the global Store using channel-derived platform string.
func matchRuleFromContext(info *relaycommon.RelayInfo, status int, body string) *Rule {
	store := GlobalStore()
	if store == nil {
		return nil
	}
	platform := ""
	if info != nil && info.ChannelType != 0 {
		platform = strconv.Itoa(info.ChannelType)
	}
	return store.Match(platform, status, body)
}

// applyRuleForRequest mutates err according to the DB rule.
func applyRuleForRequest(c *gin.Context, err *types.NewAPIError, r *Rule, upstreamStatus int) {
	if !r.PassthroughCode && r.ResponseCode != 0 {
		err.StatusCode = r.ResponseCode
	}
	if r.CustomMessage != "" {
		err.SetMessage(r.CustomMessage)
	} else {
		err.SetMessage(FixedMessageForRequest(c, upstreamStatus))
	}
}

// applyRule is retained for tests and callers that do not have request context.
func applyRule(err *types.NewAPIError, r *Rule, upstreamStatus int) {
	applyRuleForRequest(nil, err, r, upstreamStatus)
}

func shouldNormalize(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	global := operation_setting.GetUpstreamErrorNormalizeSetting()
	if global != nil && !global.Enabled {
		return false
	}
	if info != nil && info.ChannelSetting.NormalizeUpstreamErrors != nil {
		return *info.ChannelSetting.NormalizeUpstreamErrors
	}
	return true
}

type fixedMessageSet struct {
	BadRequest          string
	Unauthorized        string
	PaymentRequired     string
	Forbidden           string
	NotFound            string
	Conflict            string
	RequestTooLarge     string
	UnprocessableEntity string
	TooManyRequests     string
	Timeout             string
	Overloaded          string
	ServerError         string
	ClientError         string
	Default             string
}

var fixedMessagesByLanguage = map[string]fixedMessageSet{
	"zh": {
		BadRequest:          "请求参数错误，请检查模型、消息格式或上下文长度。",
		Unauthorized:        "鉴权失败，请检查 API Key 或上游凭证。",
		PaymentRequired:     "余额或账单异常，请检查上游账户额度。",
		Forbidden:           "权限不足，请检查模型权限、分组权限或地区限制。",
		NotFound:            "资源不存在，请检查模型名、接口路径或上游地址。",
		Conflict:            "请求冲突，请稍后重试。",
		RequestTooLarge:     "请求体过大，请压缩输入或拆分请求。",
		UnprocessableEntity: "请求格式可解析但参数组合不被支持。",
		TooManyRequests:     "请求过于频繁或额度不足，请稍后重试。",
		Timeout:             "请求超时，请稍后重试或改用流式输出。",
		Overloaded:          "上游模型过载，请稍后重试。",
		ServerError:         "上游服务异常，请稍后重试。",
		ClientError:         "请求无法被上游处理，请检查请求参数和模型权限。",
		Default:             "上游请求失败，请稍后重试。",
	},
	"en": {
		BadRequest:          "Invalid request. Check the model, message format, or context length.",
		Unauthorized:        "Authentication failed. Check the API key or upstream credentials.",
		PaymentRequired:     "Billing or balance issue. Check the upstream account quota.",
		Forbidden:           "Permission denied. Check model access, group access, or region restrictions.",
		NotFound:            "Resource not found. Check the model name, endpoint path, or upstream address.",
		Conflict:            "Request conflict. Please retry later.",
		RequestTooLarge:     "Request body is too large. Compress the input or split the request.",
		UnprocessableEntity: "The request format is valid, but this parameter combination is unsupported.",
		TooManyRequests:     "Too many requests or insufficient quota. Please retry later.",
		Timeout:             "Request timed out. Retry later or use streaming output.",
		Overloaded:          "The upstream model is overloaded. Please retry later.",
		ServerError:         "Upstream service error. Please retry later.",
		ClientError:         "The upstream service could not process the request. Check parameters and model access.",
		Default:             "Upstream request failed. Please retry later.",
	},
}

func FixedMessageForRequest(c *gin.Context, status int) string {
	return fixedMessage(status, languageFromRequest(c))
}

// FixedMessage returns the Chinese fixed client-visible message for callers
// without request language context.
func FixedMessage(status int) string {
	return fixedMessage(status, "zh")
}

func fixedMessage(status int, language string) string {
	messages, ok := fixedMessagesByLanguage[language]
	if !ok {
		messages = fixedMessagesByLanguage["zh"]
	}
	switch status {
	case http.StatusBadRequest:
		return messages.BadRequest
	case http.StatusUnauthorized:
		return messages.Unauthorized
	case http.StatusPaymentRequired:
		return messages.PaymentRequired
	case http.StatusForbidden:
		return messages.Forbidden
	case http.StatusNotFound:
		return messages.NotFound
	case http.StatusConflict:
		return messages.Conflict
	case http.StatusRequestEntityTooLarge:
		return messages.RequestTooLarge
	case http.StatusUnprocessableEntity:
		return messages.UnprocessableEntity
	case http.StatusTooManyRequests:
		return messages.TooManyRequests
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return messages.Timeout
	}
	if status == 529 {
		return messages.Overloaded
	}
	if status >= 500 && status <= 599 {
		return messages.ServerError
	}
	if status >= 400 && status <= 499 {
		return messages.ClientError
	}
	return messages.Default
}

func languageFromRequest(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return "zh"
	}
	for _, part := range strings.Split(c.GetHeader("Accept-Language"), ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.Split(part, ";")[0]))
		switch {
		case tag == "en" || strings.HasPrefix(tag, "en-"):
			return "en"
		case tag == "zh" || strings.HasPrefix(tag, "zh-"):
			return "zh"
		}
	}
	return "zh"
}

func channelIDForLog(info *relaycommon.RelayInfo) int {
	if info == nil || info.ChannelMeta == nil {
		return 0
	}
	return info.ChannelId
}
