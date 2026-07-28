package controller

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestResolveChannelTestUserIDUsesRequestUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", 2)

	userID, err := resolveChannelTestUserID(ctx)

	require.NoError(t, err)
	require.Equal(t, 2, userID)
}

func TestNormalizeChannelTestEndpointUsesResponsesForCodexBaseURL(t *testing.T) {
	channel := &model.Channel{
		Id:      77,
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer("https://new.sharedchat.cc/codex"),
	}

	require.Equal(t, string(constant.EndpointTypeOpenAIResponse), normalizeChannelTestEndpoint(channel, "gpt-5.5", ""))
}

func TestNormalizeChannelTestEndpointUsesResponsesForCodexUserAgentOverride(t *testing.T) {
	channel := &model.Channel{
		Id:      106,
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer("https://free.lyclaude.site"),
		Setting: common.GetPointer(`{"user_agent_override":"Codex Desktop/0.137.0 (Windows 10.0.26200; x86_64) unknown (codex_exec; 0.137.0)"}`),
	}

	require.Equal(t, string(constant.EndpointTypeOpenAIResponse), normalizeChannelTestEndpoint(channel, "gpt-5.5", ""))
}

func TestNormalizeChannelTestEndpointUsesResponsesBeforeAnthropicForCodexModel(t *testing.T) {
	channel := &model.Channel{
		Id:   88,
		Type: constant.ChannelTypeAnthropic,
	}

	require.Equal(t, string(constant.EndpointTypeOpenAIResponse), normalizeChannelTestEndpoint(channel, "codex-mini-latest", ""))
}

func TestNormalizeChannelTestEndpointKeepsAnthropicForClaudeModel(t *testing.T) {
	channel := &model.Channel{
		Id:   89,
		Type: constant.ChannelTypeAnthropic,
	}

	require.Equal(t, string(constant.EndpointTypeAnthropic), normalizeChannelTestEndpoint(channel, "claude-sonnet-4-20250514", ""))
}

func TestBuildChannelTestHeadersClaude(t *testing.T) {
	headers := buildChannelTestHeaders(string(constant.EndpointTypeAnthropic), true)

	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "text/event-stream", headers.Get("Accept"))
	require.Equal(t, channelTestClaudeUserAgent, headers.Get("User-Agent"))
	require.Equal(t, "cli", headers.Get("X-App"))
	require.Equal(t, channelTestClaudeVersion, headers.Get("anthropic-version"))
}

func TestBuildChannelTestHeadersCodexResponses(t *testing.T) {
	headers := buildChannelTestHeaders(string(constant.EndpointTypeOpenAIResponse), false)

	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
	require.Equal(t, channelTestCodexUserAgent, headers.Get("User-Agent"))
	require.Equal(t, channelTestCodexOriginator, headers.Get("Originator"))
}

func TestBuildTestRequestUsesClaudeRequestForAnthropicEndpoint(t *testing.T) {
	request := buildTestRequest("claude-opus-4-6", string(constant.EndpointTypeAnthropic), &model.Channel{
		Id:   71,
		Type: constant.ChannelTypeAnthropic,
	}, false, "")

	claudeRequest, ok := request.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Equal(t, "claude-opus-4-6", claudeRequest.Model)
	require.NotNil(t, claudeRequest.MaxTokens)
	require.Equal(t, uint(16), *claudeRequest.MaxTokens)
	require.Len(t, claudeRequest.Messages, 1)
	require.Equal(t, "user", claudeRequest.Messages[0].Role)
	require.Equal(t, "hi", claudeRequest.Messages[0].Content)
}

func TestBuildTestRequestUsesCodexShapedResponsesRequest(t *testing.T) {
	request := buildTestRequest("gpt-5.5", string(constant.EndpointTypeOpenAIResponse), &model.Channel{
		Id:   106,
		Type: constant.ChannelTypeOpenAI,
	}, false, "")

	responseRequest, ok := request.(*dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", responseRequest.Model)
	require.JSONEq(t, `[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`, string(responseRequest.Input))
	require.JSONEq(t, `false`, string(responseRequest.Store))
	require.JSONEq(t, `"newapi-channel-test"`, string(responseRequest.PromptCacheKey))
	require.NotNil(t, responseRequest.Stream)
	require.False(t, *responseRequest.Stream)
}

func TestExtractCompactionItemFromTestBodySupportsJSONAndSSE(t *testing.T) {
	jsonBody := []byte(`{"object":"response.compaction","output":[{"type":"compaction_summary","encrypted_content":"opaque"}]}`)
	item := extractCompactionItemFromTestBody(jsonBody, false)
	require.JSONEq(t, `{"type":"compaction_summary","encrypted_content":"opaque"}`, string(item))

	sseBody := []byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"compaction_summary\",\"encrypted_content\":\"opaque-stream\"}}\n\n")
	item = extractCompactionItemFromTestBody(sseBody, true)
	require.JSONEq(t, `{"type":"compaction_summary","encrypted_content":"opaque-stream"}`, string(item))
}

func TestShouldUseStreamForChannelTestUsesStreamingTextEndpoints(t *testing.T) {
	channel := &model.Channel{
		Id:      77,
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer("https://new.sharedchat.cc/codex"),
	}

	require.True(t, shouldUseStreamForChannelTest(channel, "gpt-5.5", ""))
	require.True(t, shouldUseStreamForChannelTest(channel, "gpt-5.5", string(constant.EndpointTypeOpenAI)))
	require.False(t, shouldUseStreamForChannelTest(channel, "text-embedding-3-small", string(constant.EndpointTypeEmbeddings)))
}

func TestValidateTestResponseBodyDoesNotKeywordBlockWithoutNonce(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"OpenAI Chat 接口未开启，多多转发分享, 免费公益官网 https://icodex.pro, QQ群: 1054851130"},"index":0}]}

data: [DONE]
`)

	err := validateTestResponseBody(body, true, "")

	require.NoError(t, err)
}

func TestValidateTestResponseBodyRequiresNonce(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"hello"},"index":0}]}

data: [DONE]
`)

	err := validateTestResponseBody(body, true, "NEWAPI_TEST_deadbeef")

	require.Error(t, err)
	require.Contains(t, err.Error(), "nonce mismatch")
}

func TestValidateTestResponseBodyAcceptsNonce(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"NEWAPI_TEST_deadbeef"},"index":0}]}

data: [DONE]
`)

	err := validateTestResponseBody(body, true, "NEWAPI_TEST_deadbeef")

	require.NoError(t, err)
}

func TestValidateTestResponseBodyAcceptsSplitNonceAcrossSSEEvents(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"NEWAPI_"},"index":0}]}

data: {"choices":[{"delta":{"content":"TEST_dead"},"index":0}]}

data: {"choices":[{"delta":{"content":"beef"},"index":0}]}

data: [DONE]
`)

	err := validateTestResponseBody(body, true, "NEWAPI_TEST_deadbeef")

	require.NoError(t, err)
}

func TestValidateTestResponseBodyAcceptsNonceWithMinorFormatting(t *testing.T) {
	body := []byte(`data: {"choices":[{"delta":{"content":"NEWAPI TEST deadbeef"},"index":0}]}

data: [DONE]
`)

	err := validateTestResponseBody(body, true, "NEWAPI_TEST_deadbeef")

	require.NoError(t, err)
}

func TestChannelTestNonceDisabledWhenChannelAntiPoisonDisabled(t *testing.T) {
	disabled := false
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{AntiPoisonEnabled: &disabled})

	require.False(t, channelTestNonceEnabledForChannel(channel))
}

func TestReadTestResponseBodyKeepsLateResponsesNonce(t *testing.T) {
	largeInstructions := strings.Repeat("x", 70<<10)
	body := `event: response.created
data: {"type":"response.created","response":{"instructions":"` + largeInstructions + `"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"NEWAPI_TEST_deadbeef"}

event: response.completed
data: {"type":"response.completed"}
`

	read, err := readTestResponseBody(io.NopCloser(strings.NewReader(body)), true)
	require.NoError(t, err)

	err = validateTestResponseBody(read, true, "NEWAPI_TEST_deadbeef")

	require.NoError(t, err)
}
