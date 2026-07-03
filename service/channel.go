package service

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func DisableChannelForAntiPoisonRisk(channelError types.ChannelError, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "anti-poison validation failed"
	}
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）命中防投毒校验，已标记风险并切换为手动禁用，原因：%s",
		channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	success := model.MarkChannelAntiPoisonRisk(channelError.ChannelId, channelError.UsingKey, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）命中防投毒风险", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）命中防投毒校验，系统已将其切换为手动禁用。原因：%s\n\n请在渠道列表查看风险标记，并在错误日志中按渠道 ID 或请求 ID 查看详细上下文。",
			channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusManuallyDisabled), subject, content)
	}
}

func IsAntiPoisonValidationError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if err.GetErrorCode() == types.ErrorCodeAntiPoisonValidationFailed {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "anti-poison validation failed")
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	monitorSetting := operation_setting.GetMonitorSetting()
	if IsTLSVerificationError(err) {
		// TLS/certificate errors are usually caused by the local trust store rather
		// than the upstream channel, so they are excluded by default. Admins can opt
		// in via monitor_setting.count_tls_errors_for_disable.
		return monitorSetting.CountTLSErrorsForDisable
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		// "skip retry" errors are typically caused by the client request itself
		// (e.g. a 400) and are excluded by default. Admins can opt in via
		// monitor_setting.count_skip_retry_errors_for_disable.
		return monitorSetting.CountSkipRetryErrorsForDisable
	}
	if IsModelScopedChannelFailureError(err) {
		// Model-scoped failures only affect a single model by default; enabling
		// monitor_setting.count_model_scoped_errors_for_disable escalates them to
		// disabling the whole channel.
		return monitorSetting.CountModelScopedErrorsForDisable
	}
	if IsImmediateChannelDisableError(err) {
		return true
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	if err.StatusCode == 402 || err.StatusCode == 403 || err.StatusCode == 429 {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func IsModelScopedChannelFailureError(err *types.NewAPIError) bool {
	if err == nil || IsTLSVerificationError(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	if err.StatusCode >= http.StatusBadRequest && err.StatusCode < http.StatusInternalServerError {
		return containsAnyModelFailureKeyword(msg, modelUnavailableKeywords)
	}
	if err.StatusCode != 0 &&
		err.StatusCode != http.StatusInternalServerError &&
		err.StatusCode != http.StatusBadGateway &&
		err.StatusCode != http.StatusServiceUnavailable &&
		err.StatusCode != http.StatusGatewayTimeout &&
		err.StatusCode != http.StatusNotImplemented &&
		!types.IsChannelError(err) {
		return false
	}
	return containsAnyModelFailureKeyword(msg, modelUnavailableKeywords) ||
		containsAnyModelFailureKeyword(msg, modelScopedChannelFailureKeywords)
}

var modelUnavailableKeywords = []string{
	"model not found",
	"model_not_found",
	"unknown model",
	"does not support this model",
	"model is not supported",
	"not implemented",
	"模型不存在",
	"未实现",
}

var modelScopedChannelFailureKeywords = []string{
	"no available account",
	"no available channel",
}

func containsAnyModelFailureKeyword(msg string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func IsChannelFailureError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if IsTLSVerificationError(err) {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if err.GetErrorCode() == types.ErrorCodeDoRequestFailed ||
		err.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded {
		return true
	}
	if err.StatusCode == http.StatusRequestTimeout ||
		err.StatusCode == http.StatusTooManyRequests ||
		err.StatusCode == http.StatusBadGateway ||
		err.StatusCode == http.StatusServiceUnavailable ||
		err.StatusCode == http.StatusGatewayTimeout {
		return true
	}
	if err.StatusCode == http.StatusInternalServerError {
		return isChannelFailureMessage(err)
	}
	return false
}

func IsImmediateChannelDisableError(err *types.NewAPIError) bool {
	if err == nil || IsTLSVerificationError(err) {
		return false
	}
	if IsModelScopedChannelFailureError(err) {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if err.StatusCode == http.StatusInternalServerError {
		return isDeterministicChannelFailureMessage(err)
	}
	return false
}

func isChannelFailureMessage(err *types.NewAPIError) bool {
	msg := strings.ToLower(err.Error())
	keywords := []string{
		"not implemented",
		"not support",
		"not supported",
		"unsupported",
		"no available account",
		"no available channel",
		"bad gateway",
		"gateway timeout",
		"upstream",
	}
	for _, keyword := range keywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func isDeterministicChannelFailureMessage(err *types.NewAPIError) bool {
	msg := strings.ToLower(err.Error())
	keywords := []string{
		"not implemented",
		"not support",
		"not supported",
		"unsupported",
		"no available account",
		"no available channel",
	}
	for _, keyword := range keywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

var tlsVerificationErrorKeywords = []string{
	"x509:",
	"certificate has expired",
	"certificate is not trusted",
	"certificate is not valid",
	"certificate signed by unknown authority",
	"cannot validate certificate",
	"failed to verify certificate",
	"hostname mismatch",
	"unknown authority",
	"tls: failed to verify",
	"跳过上游 tls 证书校验",
}

func IsTLSVerificationRawError(err error) bool {
	if err == nil {
		return false
	}

	var certificateVerificationError *tls.CertificateVerificationError
	if errors.As(err, &certificateVerificationError) {
		return true
	}

	var unknownAuthorityError *x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityError) {
		return true
	}

	var certificateInvalidError *x509.CertificateInvalidError
	if errors.As(err, &certificateInvalidError) {
		return true
	}

	var hostnameError *x509.HostnameError
	if errors.As(err, &hostnameError) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	for _, keyword := range tlsVerificationErrorKeywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

func IsTLSVerificationError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return IsTLSVerificationRawError(err)
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled && status != common.ChannelStatusManuallyDisabled {
		return false
	}
	return true
}
