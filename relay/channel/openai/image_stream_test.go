package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newImageStreamTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	return c, recorder
}

func TestOpenaiImageStreamHandlerForwardsEventsAndUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	body := strings.Join([]string{
		`data: {"type":"image_generation.partial_image","b64_json":"partial"}`,
		``,
		`data: {"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7,"input_tokens_details":{"image_tokens":2,"text_tokens":1}}}`,
		``,
		`data: [DONE]`,
	}, "\n")
	c, recorder := newImageStreamTestContext()
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, IsStream: true}

	usage, relayErr := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.ImageTokens)
	require.Contains(t, recorder.Body.String(), "event: image_generation.partial_image")
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}

func TestOpenaiImageStreamHandlerWrapsJSONAndReturnsErrorsBeforeWrite(t *testing.T) {
	t.Run("json fallback", func(t *testing.T) {
		c, recorder := newImageStreamTestContext()
		body := `{"created":1710000000,"data":[{"b64_json":"final"}],"usage":{"input_tokens":3,"output_tokens":4}}`
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, IsStream: true}
		usage, relayErr := OpenaiImageStreamHandler(c, info, resp)
		require.Nil(t, relayErr)
		require.Equal(t, 7, usage.TotalTokens)
		require.Contains(t, recorder.Body.String(), "event: image_generation.completed")
		require.Contains(t, recorder.Body.String(), "data: [DONE]")
	})

	t.Run("upstream json error", func(t *testing.T) {
		c, recorder := newImageStreamTestContext()
		body := `{"error":{"message":"image failed","type":"upstream_error","code":"bad_image"}}`
		resp := &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
		usage, relayErr := OpenaiImageHandler(c, &relaycommon.RelayInfo{}, resp)
		require.Nil(t, usage)
		require.NotNil(t, relayErr)
		require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
		require.Empty(t, recorder.Body.String())
	})
}

func TestOpenaiImageStreamHandlerRecordsUpstreamErrorEvent(t *testing.T) {
	c, recorder := newImageStreamTestContext()
	body := `data: {"type":"upstream_error","error":{"message":"stream failed"}}` + "\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, IsStream: true}
	usage, relayErr := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Contains(t, []relaycommon.StreamEndReason{
		relaycommon.StreamEndReasonHandlerStop,
		relaycommon.StreamEndReasonEOF,
	}, info.StreamStatus.EndReason)
	require.True(t, info.StreamStatus.HasErrors())
	require.Contains(t, recorder.Body.String(), "event: upstream_error")
}

func TestNormalizeOpenAIImageUsageDoesNotDoubleCount(t *testing.T) {
	usage := &dto.Usage{InputTokens: 5, OutputTokens: 4, PromptTokens: 5, CompletionTokens: 4}
	normalizeOpenAIImageUsage(usage)
	require.Equal(t, 5, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 9, usage.TotalTokens)
}
