package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestInvalidEncryptedHistoryDoesNotDisableChannel(t *testing.T) {
	for _, scenario := range []struct{ fallbackSuccess, streamError bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("fallback_%t_stream_error_%t", scenario.fallbackSuccess, scenario.streamError), func(t *testing.T) {
			fallbackSuccess := scenario.fallbackSuccess
			db := setupEmptyStreamRecoveryDB(t)
			seedRecoveryPrincipal(t, db)
			common.LogConsumeEnabled = true
			oldDisable := common.AutomaticDisableChannelEnabled
			oldMonitor := *operation_setting.GetMonitorSetting()
			oldCodes := operation_setting.AutomaticDisableStatusCodesToString()
			t.Cleanup(func() {
				common.AutomaticDisableChannelEnabled = oldDisable
				*operation_setting.GetMonitorSetting() = oldMonitor
				require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString(oldCodes))
				service.ClearChannelConsecutiveFailure(6701, failoverTestModel)
			})
			common.AutomaticDisableChannelEnabled = true
			operation_setting.GetMonitorSetting().CountAllRelayErrorsForDisable = true
			require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString("100-599"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/fallback/v1/responses" {
					writeFailoverSuccess(w)
					return
				}
				if scenario.streamError {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"type\":\"error\",\"error\":{\"code\":\"invalid_encrypted_content\",\"message\":\"The encrypted content for item rs_private could not be verified. Reason: Encrypted content item_id did not match the target item id.\"}}\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"code":"invalid_encrypted_content","message":"The encrypted content for item rs_private could not be verified. Reason: Encrypted content item_id did not match the target item id."}}`)
			}))
			t.Cleanup(server.Close)
			bad := recoveryIntegrationChannel(6701, "history", server.URL, 100, failoverTestModel)
			bad.AutoBan = common.GetPointer(1)
			require.NoError(t, db.Create(bad).Error)
			require.NoError(t, bad.AddAbilities(nil))
			if fallbackSuccess {
				good := recoveryIntegrationChannel(6702, "fallback", server.URL+"/fallback", 90, failoverTestModel)
				require.NoError(t, db.Create(good).Error)
				require.NoError(t, good.AddAbilities(nil))
			}
			model.InitChannelCache()
			// More than the disable threshold, exercising relay health callbacks.
			for i := 0; i < 4; i++ {
				requestID := fmt.Sprintf("encrypted-history-%t-%t-%d", fallbackSuccess, scenario.streamError, i)
				router, _, _ := recoveryIntegrationRouter(requestID)
				recorder := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"recovery-integration-model","input":[{"role":"user","content":"hi"},{"type":"reasoning","id":"rs_private","encrypted_content":"opaque"}],"stream":true}`))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, req)
				if fallbackSuccess {
					require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					require.Contains(t, recorder.Body.String(), "winner")
				} else {
					require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
					require.Equal(t, string(types.ErrorCodeInvalidEncryptedContent), gjson.Get(recorder.Body.String(), "error.code").String())
					require.Contains(t, gjson.Get(recorder.Body.String(), "error.message").String(), "Please start a new conversation")
				}
				require.NotContains(t, recorder.Body.String(), "rs_private")
				require.Zero(t, service.PeekChannelConsecutiveFailure(6701, failoverTestModel))
				var channel model.Channel
				require.NoError(t, db.First(&channel, 6701).Error)
				require.Equal(t, common.ChannelStatusEnabled, channel.Status)
				var healthRows int64
				require.NoError(t, db.Model(&model.ChannelModelStatus{}).Where("channel_id = ?", 6701).Count(&healthRows).Error)
				require.Zero(t, healthRows, "invalid history must not accumulate model failure counts")
				var logs []model.Log
				require.NoError(t, db.Where("request_id = ?", requestID).Find(&logs).Error)
				require.Len(t, logs, 1)
				require.Contains(t, logs[0].Other, "rs_private", "administrator attempts retain the diagnostic")
			}
			perfmetrics.WaitForPendingSamples()
		})
	}
}
