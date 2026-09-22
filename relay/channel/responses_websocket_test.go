package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestResponsesWSProbeKeepsHTTPTransport(t *testing.T) {
	methods := make(chan string, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			methods <- "UPGRADE"
		} else {
			methods <- r.Method
		}
		_, _ = io.WriteString(w, `{ "object":"response", "output":[] }`)
	}))
	defer upstream.Close()
	transport := NewResponsesWSTransport(context.Background())
	defer transport.Close()
	for _, stream := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx := relaycommon.WithResponsesWSExecution(context.Background(), &relaycommon.ResponsesWSExecution{Transport: transport})
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{}`)).WithContext(ctx)
		req, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(`{"model":"probe","input":[]}`))
		require.NoError(t, err)
		info := &relaycommon.RelayInfo{IsStream: stream, IsChannelTest: true, RelayFormat: types.RelayFormatOpenAIResponses,
			ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{ResponsesWebSocket: true}}}
		response, err := DoRequest(c, req, info)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.MethodPost, <-methods)
	}
}

func TestResponsesWSPrefixPreservesToolArgumentPrecision(t *testing.T) {
	first := []json.RawMessage{json.RawMessage(`{"type":"function_call","arguments":{"value":9007199254740992}}`)}
	second := []json.RawMessage{json.RawMessage(`{"type":"function_call","arguments":{"value":9007199254740993}}`)}
	require.NotEqual(t, responsesWSPrefixHash(first), responsesWSPrefixHash(second))
}
