package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateResponsesStreamRequiresTerminal(t *testing.T) {
	body := `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	finalResp, usage, apiErr := aggregateResponsesStreamToResponse(ctx, info, resp)
	require.Nil(t, finalResp)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "responses stream missing terminal event")
}

func TestAggregateResponsesStreamAcceptsIncompleteTerminal(t *testing.T) {
	body := `data: {"type":"response.incomplete","response":{"id":"resp-test","object":"response","status":"incomplete","model":"gpt-test","output":[],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	finalResp, usage, apiErr := aggregateResponsesStreamToResponse(ctx, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, finalResp)
	require.NotNil(t, finalResp.IncompleteDetails)
	require.Equal(t, "max_output_tokens", finalResp.IncompleteDetails.Reasoning)
	require.Equal(t, 3, usage.TotalTokens)

	encoded, err := common.Marshal(finalResp)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"incomplete_details":{"reason":"max_output_tokens"}`)
	require.NotContains(t, string(encoded), `"incomplete_details":{"reasoning":`)
}

func TestOaiResponsesStreamHandlerAcceptsIncompleteTerminal(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		`data: {"type":"response.incomplete","response":{"id":"resp-test","object":"response","status":"incomplete","model":"gpt-test","output":[],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Set(common.RequestIdKey, "native-incomplete-terminal-test")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAIResponses,
		DisablePing: true,
	}

	usage, apiErr := OaiResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamAttemptOK, info.StreamStatus.Outcome().Code)
	require.Contains(t, recorder.Body.String(), "response.incomplete")
}