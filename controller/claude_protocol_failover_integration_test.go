package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestClaudeProtocolMismatchFailsOverWithoutLosingExtensions(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		stream    bool
		fallback  bool
		pinned    bool
		malformed bool
		explicit  bool
		native    bool
		setting   string
	}{
		{name: "stream_native_fallback", stream: true, fallback: true},
		{name: "json_native_fallback", fallback: true},
		{name: "no_compatible_channel", stream: true},
		{name: "pinned_route", stream: true, fallback: true, pinned: true},
		{name: "malformed_request", stream: true, fallback: true, malformed: true},
		{name: "explicit_responses_binding", stream: true, explicit: true},
		{name: "same_channel_native_fallback", stream: true, native: true},
		{name: "codex_profile_preserved", setting: `{"responses_request_profile":"codex"}`},
		{name: "codex_identity_preserved", setting: `{"requires_codex_identity":true}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := setupEmptyStreamRecoveryDB(t)
			service.ClearChannelConsecutiveFailure(6801, failoverTestModel)
			t.Cleanup(func() { service.ClearChannelConsecutiveFailure(6801, failoverTestModel) })
			seedRecoveryPrincipal(t, db)
			common.LogConsumeEnabled = true
			oldDefaults := operation_setting.ModelEndpointDefaults2JsonString()
			oldDisable := common.AutomaticDisableChannelEnabled
			oldMonitor := *operation_setting.GetMonitorSetting()
			oldCodes := operation_setting.AutomaticDisableStatusCodesToString()
			t.Cleanup(func() {
				require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(oldDefaults))
				common.AutomaticDisableChannelEnabled = oldDisable
				*operation_setting.GetMonitorSetting() = oldMonitor
				require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString(oldCodes))
				model.ReloadModelEndpointCache()
			})
			require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{"enabled":true,"normalize_text_endpoints":true,"entries":[{"match_type":"exact","pattern":"recovery-integration-model","channel_type":1,"default_endpoint":"openai-response","supported_endpoints":["openai","openai-response"],"fallback_endpoint":"openai-response","auto_correct":true}]}`))
			common.AutomaticDisableChannelEnabled = true
			operation_setting.GetMonitorSetting().CountAllRelayErrorsForDisable = true
			operation_setting.GetMonitorSetting().CountSkipRetryErrorsForDisable = true
			require.NoError(t, operation_setting.AutomaticDisableStatusCodesFromString("100-599"))
			badCalls, goodCalls := 0, 0
			var unexpectedRequests []string
			var forwarded []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/native/v1/messages" {
					badCalls++
					body, _ := io.ReadAll(r.Body)
					unexpectedRequests = append(unexpectedRequests, r.Method+" "+r.URL.Path+" "+string(body))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				goodCalls++
				forwarded, _ = io.ReadAll(r.Body)
				require.Equal(t, "sk-upstream-test", r.Header.Get("x-api-key"))
				if scenario.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_native\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"recovery-integration-model\",\"content\":[],\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"native-winner\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"msg_native","type":"message","role":"assistant","model":"recovery-integration-model","content":[{"type":"text","text":"native-winner"}],"stop_reason":"end_turn","usage":{"input_tokens":11,"output_tokens":3}}`)
				}
			}))
			t.Cleanup(server.Close)
			bad := recoveryIntegrationChannel(6801, "responses-only", server.URL+"/responses", 100, failoverTestModel)
			bad.AutoBan = common.GetPointer(1)
			if scenario.explicit {
				bad.Type = constant.ChannelTypeAnthropic
			}
			if scenario.native {
				bad.Type = constant.ChannelTypeAnthropic
				bad.BaseURL = common.GetPointer(server.URL + "/native")
			}
			if scenario.setting != "" {
				bad.Type = constant.ChannelTypeAnthropic
				bad.Setting = common.GetPointer(scenario.setting)
			}
			good := recoveryIntegrationChannel(6802, "native-messages", server.URL+"/native", 90, failoverTestModel)
			good.Type = constant.ChannelTypeAnthropic
			good.ParamOverride = common.GetPointer(`{"operations":[{"path":"tools.0.description","mode":"append","value":"-once"}]}`)
			if scenario.native {
				bad.ParamOverride = good.ParamOverride
			}
			for _, channel := range []*model.Channel{bad, good} {
				if channel == good && !scenario.fallback {
					continue
				}
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			model.InitChannelCache()
			if scenario.explicit {
				require.NoError(t, db.Create(&model.ModelEndpoint{ChannelId: bad.Id, Model: failoverTestModel, ChannelType: common.GetPointer(constant.ChannelTypeCodex), BaseURL: server.URL + "/responses"}).Error)
			}
			model.ReloadModelEndpointCache()
			requestID := "claude-protocol-" + scenario.name
			router, _, _ := recoveryIntegrationRouter(requestID)
			if scenario.pinned {
				router.Use(func(c *gin.Context) {
					common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "6801")
				})
			}
			var used []string
			router.POST("/v1/messages", middleware.Distribute(), func(c *gin.Context) {
				Relay(c, types.RelayFormatClaude)
				used = append([]string(nil), c.GetStringSlice("use_channel")...)
			})
			tool := `{"name":"lookup","description":"keep me","defer_loading":true,"input_schema":{"type":"object"}}`
			if scenario.malformed {
				tool = `{"input_schema":{"type":"object"}}`
			}
			extra := `,"thinking":{"type":"adaptive","display":"future-display"},"context_management":{"edits":[]}`
			if scenario.malformed {
				extra = ""
			}
			body := fmt.Sprintf(`{"model":"recovery-integration-model","max_tokens":512,"stream":%t,"messages":[{"role":"user","content":"hi"}],"tools":[%s]%s}`, scenario.stream, tool, extra)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)
			require.Zero(t, badCalls, "conversion failure must not send an upstream request: %v", unexpectedRequests)
			success := (scenario.fallback || scenario.native) && !scenario.pinned && !scenario.malformed
			if success {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), "native-winner")
				if scenario.native {
					require.Equal(t, []string{"6801"}, used)
				} else {
					require.Equal(t, []string{"6801", "6802"}, used)
				}
				require.Equal(t, 1, goodCalls)
				require.True(t, gjson.GetBytes(forwarded, "tools.0.defer_loading").Bool())
				require.Equal(t, "future-display", gjson.GetBytes(forwarded, "thinking.display").String())
				require.Equal(t, `{"edits":[]}`, gjson.GetBytes(forwarded, "context_management").Raw)
				require.Equal(t, "keep me-once", gjson.GetBytes(forwarded, "tools.0.description").String())
				require.Equal(t, int64(512), gjson.GetBytes(forwarded, "max_tokens").Int())
				require.Empty(t, recorder.Header().Get("X-NewAPI-Endpoint-Corrected"))
			} else {
				require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
				require.Zero(t, goodCalls)
				require.Equal(t, []string{"6801"}, used)
				if !scenario.malformed {
					require.Contains(t, recorder.Body.String(), types.PublicRequestProtocolUnsupportedMessage)
					require.NotContains(t, recorder.Body.String(), "tools.defer_loading")
				}
				require.NotContains(t, recorder.Body.String(), types.PublicModelCapacityMessage)
			}
			var logs []model.Log
			require.NoError(t, db.Where("request_id = ?", requestID).Find(&logs).Error)
			require.Len(t, logs, 1)
			if !scenario.malformed {
				if !scenario.native {
					require.Contains(t, logs[0].Other, "tools.defer_loading", "administrator retains the original conversion error")
				}
				require.Zero(t, service.PeekChannelConsecutiveFailure(6801, failoverTestModel))
				var healthCount int64
				require.NoError(t, db.Model(&model.ChannelModelStatus{}).Where("channel_id = ?", 6801).Count(&healthCount).Error)
				if !scenario.native {
					require.Zero(t, healthCount)
				}
			}
			var ledger model.BillingLedger
			require.NoError(t, db.Where("request_id = ?", requestID).First(&ledger).Error)
			if success {
				require.Equal(t, model.BillingLedgerStateSettled, ledger.State)
			} else {
				require.Equal(t, model.BillingLedgerStateRefunded, ledger.State)
			}
			perfmetrics.WaitForPendingSamples()
		})
	}
}
