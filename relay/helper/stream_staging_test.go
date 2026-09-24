package helper

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamStagingCommitsOnlySemanticOutput(t *testing.T) {
	for _, test := range []struct{ name, prelude, content, terminal string }{
		{"responses", `{"type":"response.created"}`, `{"type":"response.output_text.delta","delta":"hello"}`, `{"type":"response.completed"}`},
		{"openai", `{"choices":[{"delta":{"role":"assistant"}}]}`, `{"choices":[{"delta":{"content":"hello"}}]}`, "[DONE]"},
		{"claude", `{"type":"message_start"}`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`, `{"type":"message_stop"}`},
		{"gemini", `{"candidates":[{"content":{"role":"model"}}]}`, `{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`, `{"candidates":[{"finishReason":"STOP"}]}`},
		{"tool", `{"type":"response.created"}`, `{"type":"response.output_item.added","item":{"type":"function_call","name":"search","call_id":"call_one"}}`, `{"type":"response.completed"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(r)
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			info := &relaycommon.RelayInfo{IsStream: true, StreamStatus: relaycommon.NewStreamStatus()}
			err := WithStreamStaging(c, info, func() *types.NewAPIError {
				SetEventStreamHeaders(c)
				require.NoError(t, PingData(c))
				require.NoError(t, StringData(c, test.prelude))
				require.Empty(t, r.Body.String())
				require.False(t, c.Writer.Written())
				require.Empty(t, r.Header().Get("Content-Type"))
				require.NoError(t, StringData(c, test.content))
				require.Contains(t, r.Body.String(), test.prelude)
				require.Contains(t, r.Body.String(), test.content)
				require.NoError(t, StringData(c, test.terminal))
				return nil
			})
			require.Nil(t, err)
		})
	}
}

func TestStreamStagingDiscardsFailedPreludeAndHeaders(t *testing.T) {
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{IsStream: true}
	err := WithStreamStaging(c, info, func() *types.NewAPIError {
		SetEventStreamHeaders(c)
		c.Header("X-Private-Upstream", "private-host")
		require.NoError(t, StringData(c, `{"type":"response.created","response":{"id":"failed_response"}}`))
		return types.NewError(errors.New("upstream failed"), types.ErrorCodeBadResponse)
	})
	require.NotNil(t, err)
	require.Empty(t, r.Body.String())
	require.Empty(t, r.Header().Get("X-Private-Upstream"))
	require.False(t, c.Writer.Written())
}

func TestSemanticDeltaDisarmsWatchdog(t *testing.T) {
	previous := common.RelayFirstByteTimeout
	common.RelayFirstByteTimeout = 1
	t.Cleanup(func() { common.RelayFirstByteTimeout = previous })
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	err := WithStreamStaging(c, &relaycommon.RelayInfo{IsStream: true}, func() *types.NewAPIError {
		SetEventStreamHeaders(c)
		require.NoError(t, StringData(c, `{"type":"response.output_text.delta","delta":"hello"}`))
		timer := time.NewTimer(1200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-c.Request.Context().Done():
			t.Fatal("语义输出后 watchdog 仍取消请求")
		case <-timer.C:
		}
		require.NoError(t, StringData(c, `{"type":"response.completed"}`))
		return nil
	})
	require.Nil(t, err, fmt.Sprint(err))
}

func TestStagingUsesSavedTimeoutWhileWaitingForHeaders(t *testing.T) {
	old := common.OptionMap
	t.Cleanup(func() { common.OptionMap = old })
	common.OptionMap = map[string]string{"RelayFirstByteTimeout": "1"}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	started := time.Now()
	err := WithStreamStaging(c, &relaycommon.RelayInfo{IsStream: true}, func() *types.NewAPIError {
		select {
		case <-c.Request.Context().Done():
		case <-time.After(3 * time.Second):
			t.Fatal("saved timeout ignored")
		}
		return nil
	})
	require.NotNil(t, err)
	require.Less(t, time.Since(started), 2*time.Second)
}
