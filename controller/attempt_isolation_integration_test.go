package controller

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestChannelFailoverIsolatesProtocolAndGuardState(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	seedRecoveryPrincipal(t, db)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/responses", r.URL.Path)
		writeFailoverSuccess(w)
	}))
	t.Cleanup(server.Close)
	for i := 0; i < 2; i++ {
		ch := recoveryIntegrationChannel(9200+i, "attempt-isolation", server.URL, int64(100-i), failoverTestModel)
		require.NoError(t, db.Create(ch).Error)
		require.NoError(t, ch.AddAbilities(nil))
	}
	model.InitChannelCache()
	original := relayDispatchUpstream
	t.Cleanup(func() { relayDispatchUpstream = original })
	calls := 0
	relayDispatchUpstream = func(c *gin.Context, info *relaycommon.RelayInfo, format types.RelayFormat, ws *websocket.Conn) *types.NewAPIError {
		calls++
		if calls == 1 {
			info.InitChannelMeta(c)
			info.RelayMode = relayconstant.RelayModeChatCompletions
			info.RequestURLPath = "/v1/chat/completions"
			info.FinalRequestRelayFormat = types.RelayFormatClaude
			info.AppendRequestConversion(types.RelayFormatClaude)
			info.AntiPoisonGuardPrefix = "previous-channel-guard"
			info.AntiPoisonAnswerEnvelopeNonce = "previous-channel-envelope"
			info.AntiPoisonResponseProofNonce = "previous-channel-proof"
			info.AntiPoisonCanaryNonce = "previous-channel-canary"
			openaicompat.SetResponsesBridgeToolMapping(c, openaicompat.ResponsesBridgeToolMapping{NamespaceTools: map[string]openaicompat.ResponsesNamespaceToolName{"team__send": {Namespace: "team", Name: "send"}}})
			return types.NewOpenAIError(errors.New("first upstream failed"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		require.Empty(t, info.AntiPoisonGuardPrefix)
		require.Empty(t, info.AntiPoisonAnswerEnvelopeNonce)
		require.Empty(t, info.AntiPoisonResponseProofNonce)
		require.Empty(t, info.AntiPoisonCanaryNonce)
		require.Equal(t, relayconstant.RelayModeResponses, info.RelayMode)
		require.Equal(t, "/v1/responses", info.RequestURLPath)
		require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
		require.Empty(t, openaicompat.GetResponsesBridgeToolMapping(c).NamespaceTools)
		return original(c, info, format, ws)
	}
	router, used, _ := recoveryIntegrationRouter("attempt-isolation")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"recovery-integration-model","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, 2, calls)
	require.Len(t, *used, 2)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "winner")
	perfmetrics.WaitForPendingSamples()
}
