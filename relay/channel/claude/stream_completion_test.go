package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newClaudeStreamTestContext(body string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}
	return c, recorder, resp, info
}

func incompleteClaudeStreamBody() string {
	return strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","model":"claude-test","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`,
		``,
	}, "\n")
}

func completeClaudeStreamWithoutMessageStopBody() string {
	return strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","model":"claude-test","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"complete"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		``,
	}, "\n")
}

func TestClaudeStreamHandlerRejectsMissingMessageDelta(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(incompleteClaudeStreamBody())

	usage, relayErr := claudeStreamHandlerWithCompletionGuard(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.Contains(t, relayErr.Error(), "missing message_delta")
	require.True(t, types.IsSkipRetryError(relayErr))
	require.Contains(t, recorder.Body.String(), "partial")
	require.NotContains(t, recorder.Body.String(), "data: [DONE]")
}

func TestRawClaudeStreamRejectsMissingMessageDeltaWithoutRetry(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(incompleteClaudeStreamBody())
	info.RelayFormat = types.RelayFormatClaude

	usage, relayErr := claudeStreamHandlerWithCompletionGuard(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.True(t, types.IsSkipRetryError(relayErr))
	require.True(t, c.Writer.Written())
	require.Contains(t, recorder.Body.String(), "partial")
}

func TestClaudeAggregateStreamRejectsMissingMessageDeltaBeforeReplay(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(incompleteClaudeStreamBody())

	usage, relayErr := claudeAggregateStreamThenReplayWithCompletionGuard(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.Contains(t, relayErr.Error(), "missing message_delta")
	require.False(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerAcceptsMessageDeltaWithoutMessageStop(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(completeClaudeStreamWithoutMessageStopBody())

	usage, relayErr := claudeStreamHandlerWithCompletionGuard(c, resp, info)
	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "complete")
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}
