package openai

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateResponsesStreamBlocksEnvelopeOutsideText(t *testing.T) {
	enabled := true
	final := `{"type":"response.completed","response":{"id":"resp-test","object":"response","created_at":1,"model":"gpt-test","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"<newapi_answer nonce=\"n1\">OK</newapi_answer>\nU2FsdGVkX1+xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}]}]}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("data: " + final + "\n\ndata: [DONE]\n\n")),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         101,
			ChannelSetting:    dto.ChannelSettings{AntiPoisonEnabled: &enabled},
			UpstreamModelName: "gpt-test",
		},
		OriginModelName:               "gpt-test",
		AntiPoisonAnswerEnvelopeNonce: "n1",
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, _, err := aggregateResponsesStreamToResponse(ctx, info, resp)
	if err == nil || !errors.Is(err, antipoison.ErrEnvelopeOutsideText) {
		t.Fatalf("err=%v, want envelope outside text", err)
	}
}

func TestResponsesHandlersPreserveCacheWriteTokens(t *testing.T) {
	payload := `{"id":"resp-test","object":"response","created_at":1,"model":"gpt-test","output":[],"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30}}}`
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	usage, apiErr := OaiResponsesHandler(ctx, info, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload))})
	require.Nil(t, apiErr)
	require.Equal(t, 30, usage.PromptTokensDetails.CacheWriteTokens)

	streamUsage := responsesUsageFromResponse(info, &dto.OpenAIResponsesResponse{
		Usage: &dto.Usage{
			InputTokens:        100,
			OutputTokens:       5,
			TotalTokens:        105,
			InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 20, CacheWriteTokens: 30},
		},
	}, "")
	require.Equal(t, 30, streamUsage.PromptTokensDetails.CacheWriteTokens)
}

func runResponsesStreamTest(t *testing.T, body string) (*httptest.ResponseRecorder, *relaycommon.RelayInfo, *types.NewAPIError) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		DisablePing:     true,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
	_, apiErr := OaiResponsesStreamHandler(ctx, info, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	})
	return recorder, info, apiErr
}

func TestOaiResponsesStreamHandlerClassifiesEmptyAndMalformedBodies(t *testing.T) {
	_, emptyInfo, apiErr := runResponsesStreamTest(t, "")
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
	require.Equal(t, relaycommon.StreamAttemptEmptyResponse, emptyInfo.StreamStatus.Outcome().Code)

	_, malformedInfo, apiErr := runResponsesStreamTest(t, "data: not-json\n\n")
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	require.Equal(t, relaycommon.StreamAttemptBadResponseBody, malformedInfo.StreamStatus.Outcome().Code)
}

func TestOaiResponsesStreamHandlerRequiresCompletedTerminal(t *testing.T) {
	recorder, info, apiErr := runResponsesStreamTest(t, `data: {"type":"response.created"}`+"\n\n")
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	outcome := info.StreamStatus.Outcome()
	require.Equal(t, relaycommon.StreamAttemptIncomplete, outcome.Code)
	require.True(t, outcome.ClientCommitted)
	require.Contains(t, recorder.Body.String(), "response.created")
}

func TestOaiResponsesStreamHandlerCompletedAndFailedTerminals(t *testing.T) {
	completed := `data: {"type":"response.completed","response":{"id":"resp-test","object":"response","model":"gpt-test","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n"
	_, completedInfo, apiErr := runResponsesStreamTest(t, completed)
	require.Nil(t, apiErr)
	require.Equal(t, relaycommon.StreamAttemptOK, completedInfo.StreamStatus.Outcome().Code)

	recorder, failedInfo, apiErr := runResponsesStreamTest(t, `data: {"type":"response.failed"}`+"\n\n")
	require.NotNil(t, apiErr)
	require.Equal(t, relaycommon.StreamAttemptFailed, failedInfo.StreamStatus.Outcome().Code)
	require.Empty(t, recorder.Body.String(), "failed terminal must stay pre-commit so recovery can retry")
}
