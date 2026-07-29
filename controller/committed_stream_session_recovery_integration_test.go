package controller

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCommittedResponseFailureRepairsNextRequestWithoutSplicing(t *testing.T) {
	const (
		modelName  = "recovery-integration-model"
		sessionKey = "session-committed-failure-recovery"
		requestA   = "req-committed-failure-a"
		requestB   = "req-committed-failure-b"
	)
	body := []byte(`{"model":"recovery-integration-model","input":"hello","stream":true,"prompt_cache_key":"session-committed-failure-recovery"}`)
	var callsA, callsB int
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callsA++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_a\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"recovery-integration-model\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial-a\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_a\",\"status\":\"failed\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"temporary upstream failure\"}}}\n\n")
	}))
	t.Cleanup(failing.Close)
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callsB++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_b\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"recovery-integration-model\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"from-b\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_b\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"recovery-integration-model\",\"output\":[],\"usage\":{\"input_tokens\":11,\"output_tokens\":3,\"total_tokens\":14}}}\n\n")
	}))
	t.Cleanup(healthy.Close)

	db := setupEmptyStreamRecoveryDB(t)
	common.LogConsumeEnabled = true
	channelA := recoveryIntegrationChannel(201, "committed-failure", failing.URL, 20, modelName)
	channelB := recoveryIntegrationChannel(205, "recovery-success", healthy.URL, 10, modelName)
	for _, channel := range []*model.Channel{channelA, channelB} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	seedRecoveryPrincipal(t, db)
	model.InitChannelCache()
	seedRecoveryAffinity(t, body, modelName, sessionKey, channelA.Id)

	routerA, usedA, _ := recoveryIntegrationRouter(requestA)
	recorderA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	reqA.Header.Set("Content-Type", "application/json")
	routerA.ServeHTTP(recorderA, reqA)
	require.Equal(t, http.StatusOK, recorderA.Code)
	require.Equal(t, []string{"201"}, *usedA)
	require.Contains(t, recorderA.Body.String(), "partial-a")
	require.NotContains(t, recorderA.Body.String(), "from-b")
	require.Equal(t, 1, callsA)
	require.Zero(t, callsB)

	routerB, usedB, finalB := recoveryIntegrationRouter(requestB)
	recorderB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	reqB.Header.Set("Content-Type", "application/json")
	routerB.ServeHTTP(recorderB, reqB)
	require.Equal(t, http.StatusOK, recorderB.Code, recorderB.Body.String())
	require.Equal(t, []string{"205"}, *usedB)
	require.Equal(t, channelB.Id, *finalB)
	require.Contains(t, recorderB.Body.String(), "from-b")
	require.Equal(t, 1, callsA)
	require.Equal(t, 1, callsB)

	var ledgerA, ledgerB model.BillingLedger
	require.NoError(t, db.Where("request_id = ?", requestA).First(&ledgerA).Error)
	require.NoError(t, db.Where("request_id = ?", requestB).First(&ledgerB).Error)
	require.Equal(t, model.BillingLedgerStateRefunded, ledgerA.State)
	require.Equal(t, model.BillingLedgerStateSettled, ledgerB.State)
	require.NotEqual(t, ledgerA.ID, ledgerB.ID)
	model.FlushSpendLogBatchForTest(time.Second)
	var consumeLogB model.Log
	require.NoError(t, db.Where("request_id = ? AND type = ?", requestB, model.LogTypeConsume).First(&consumeLogB).Error)
	other, err := common.StrToMap(consumeLogB.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	recoveryInfo, ok := adminInfo["session_recovery"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "recovered_next_request", recoveryInfo["action"])
	require.Equal(t, "success", recoveryInfo["result"])
	require.EqualValues(t, channelA.Id, recoveryInfo["previous_channel"])
	require.EqualValues(t, channelB.Id, recoveryInfo["channel_id"])

	affinityCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	affinityCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	affinityCtx.Request.Header.Set("Content-Type", "application/json")
	preferred, found := service.GetPreferredChannelByAffinity(affinityCtx, modelName, "default")
	require.True(t, found)
	require.Equal(t, channelB.Id, preferred)

	otherBody := bytes.ReplaceAll(body, []byte(sessionKey), []byte("another-session"))
	otherCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	otherCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(otherBody))
	otherCtx.Request.Header.Set("Content-Type", "application/json")
	_, found = service.GetPreferredChannelByAffinity(otherCtx, modelName, "default")
	require.False(t, found)
	require.False(t, service.ShouldAvoidChannelForSession(otherCtx, channelA.Id))
	perfmetrics.WaitForPendingSamples()
}
