package errornorm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestActionableErrorsSurviveBothNormalizationStages(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 257}}
	for _, code := range []types.ErrorCode{types.ErrorCodeInsufficientUserQuota, types.ErrorCodePreConsumeTokenQuotaFailed, types.ErrorCodeInvalidRequest, types.ErrorCodeModelNotFound, types.ErrorCodeRateLimitExceeded} {
		original := types.NewErrorWithStatusCode(errors.New("local actionable explanation"), code, 400)
		normalized := New().OnClientResponseError(&gin.Context{}, info, original)
		public := types.PublicRelayError(normalized)
		require.Equal(t, code, public.GetErrorCode())
		require.Equal(t, "local actionable explanation", public.Error())
	}
	for _, code := range []string{"model_not_found", "model_unavailable"} {
		original := types.WithOpenAIError(types.OpenAIError{Code: code, Message: "private-host secret diagnostic"}, 400)
		public := types.PublicRelayError(New().OnClientResponseError(&gin.Context{}, info, original))
		require.Equal(t, types.ErrorCode(code), public.GetErrorCode())
		require.NotContains(t, public.Error(), "private")
		require.Contains(t, original.Error(), "private")
	}
}

func TestHook_OnClientResponseError_ReplacesRelayMessage(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("upstream injected private message"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 75}}

	h := New()
	normalized := h.OnClientResponseError(&gin.Context{}, info, err)

	require.Equal(t, "请求参数错误，请检查模型、消息格式或上下文长度。", normalized.Error())
	require.NotContains(t, normalized.ToOpenAIError().Message, "injected")
}

func TestHook_OnClientResponseError_UsesAcceptLanguage(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("upstream injected private message"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusUnauthorized,
	)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 75}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Accept-Language", "en-US,en;q=0.9")

	normalized := New().OnClientResponseError(c, info, err)

	require.Equal(t, "Authentication failed. Check the API key or upstream credentials.", normalized.Error())
	require.NotContains(t, normalized.ToOpenAIError().Message, "injected")
}

func TestHook_OnClientResponseError_SkipsBeforeChannelSelected(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("local validation message"),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
	)

	h := New()
	normalized := h.OnClientResponseError(&gin.Context{}, nil, err)

	require.Equal(t, "local validation message", normalized.Error())
}

func TestApplyRuleForRequest_UsesCustomMessageFirst(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("upstream injected message"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusForbidden,
	)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Accept-Language", "en-US")

	applyRuleForRequest(c, err, &Rule{CustomMessage: "custom"}, http.StatusForbidden)

	require.Equal(t, "custom", err.Error())
}

func TestApplyRuleForRequest_FallbackUsesAcceptLanguage(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("upstream injected message"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusForbidden,
	)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Accept-Language", "en-US")

	applyRuleForRequest(c, err, &Rule{}, http.StatusForbidden)

	require.Equal(t, "Permission denied. Check model access, group access, or region restrictions.", err.Error())
}

func TestApplyRule_NeverPassesThroughUpstreamBody(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("upstream injected message"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusForbidden,
	)
	rule := &Rule{
		PassthroughCode: true,
		PassthroughBody: true,
	}

	applyRule(err, rule, http.StatusForbidden)

	require.Equal(t, http.StatusForbidden, err.StatusCode)
	require.Equal(t, "权限不足，请检查模型权限、分组权限或地区限制。", err.Error())
	require.NotContains(t, err.ToOpenAIError().Message, "injected")
}

func TestNormalizeRuleForSaveDisablesDeprecatedPassthroughFields(t *testing.T) {
	rule := &Rule{
		PassthroughBody: true,
		SkipMonitoring:  true,
	}

	normalizeRuleForSave(rule)

	require.False(t, rule.PassthroughBody)
	require.False(t, rule.SkipMonitoring)
}
