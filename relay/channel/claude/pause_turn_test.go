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

func claudePauseTurnStreamBody() string {
	return strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_pause","type":"message","role":"assistant","model":"claude-test","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"pause_turn"},"usage":{"output_tokens":2}}`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
}

func claudePauseTurnResponseBody() string {
	return `{"id":"msg_pause","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"partial"}],"stop_reason":"pause_turn","usage":{"input_tokens":1,"output_tokens":2}}`
}

func TestClaudeStreamBridgeRejectsPauseTurnWithoutFakeTerminal(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(claudePauseTurnStreamBody())

	usage, relayErr := claudeStreamHandlerWithCompletionGuard(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.Contains(t, relayErr.Error(), "paused before completion")
	require.True(t, types.IsSkipRetryError(relayErr))
	require.True(t, info.ClientResponseCommitted())
	require.Contains(t, recorder.Body.String(), "partial")
	require.NotContains(t, recorder.Body.String(), `"finish_reason":"pause_turn"`)
	require.NotContains(t, recorder.Body.String(), "data: [DONE]")
}

func TestClaudeAggregateBridgeRejectsPauseTurnBeforeReplay(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(claudePauseTurnStreamBody())

	usage, relayErr := claudeAggregateStreamThenReplayWithCompletionGuard(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.True(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String())
}

func TestRawClaudeStreamAllowsPauseTurn(t *testing.T) {
	c, recorder, resp, info := newClaudeStreamTestContext(claudePauseTurnStreamBody())
	info.RelayFormat = types.RelayFormatClaude

	usage, relayErr := claudeStreamHandlerWithCompletionGuard(c, resp, info)
	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "pause_turn")
}

func TestClaudeNonStreamBridgeRejectsPauseTurnBeforeWrite(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(claudePauseTurnResponseBody())),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
		RelayFormat: types.RelayFormatOpenAI,
	}

	usage, relayErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.True(t, types.IsSkipRetryError(relayErr))
	require.Empty(t, recorder.Body.String())
}

func TestClaudePauseTurnErrorAllowsNativeRelay(t *testing.T) {
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude}
	require.Nil(t, claudePauseTurnError(info, "pause_turn"))
}
