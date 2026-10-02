package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Real distributor, Relay loop, adapters, HTTP bodies and billing ledger. No
// replacement of relaySelectChannel or relayDispatchUpstream is used here.
func TestRelayCompactMappingExplicitSourceWire(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"recovery-integration-model":1,"recovery-integration-model-openai-compact":1}`))
	var body map[string]interface{}
	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = common.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"compact-ok","object":"response.compaction","model":"explicit-target","output":[{"type":"compaction","encrypted_content":"preserve-result"}],"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}}`)
	}))
	defer upstream.Close()
	channel := recoveryIntegrationChannel(14001, "compact-mapped", upstream.URL, 20, "recovery-integration-model")
	channel.ModelMapping = common.GetPointer(`{"version":2,"rules":[{"id":"base","from":"recovery-integration-model","to":"WRONG","priority":999},{"id":"explicit","from":"recovery-integration-model-openai-compact","to":"explicit-target","priority":0}]}`)
	channel.SetSetting(dto.ChannelSettings{PassThroughBodyEnabled: true, ResponsesCompaction: &dto.ResponsesCompactionSettings{DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionLegacy}}})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	seedRecoveryPrincipal(t, db)
	model.InitChannelCache()
	router, _, _ := recoveryIntegrationRouter("mapping-compact-explicit")
	var state *relaycommon.ChannelFailoverState
	router.POST("/v1/responses/compact", middleware.Distribute(), func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		raw, _ := c.Get(relaycommon.ChannelFailoverContextKey)
		state, _ = raw.(*relaycommon.ChannelFailoverState)
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/responses/compact", bytes.NewBufferString(`{"model":"recovery-integration-model-openai-compact","input":[{"role":"user","content":"hello"}],"metadata":{"keep":"yes"}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	perfmetrics.WaitForPendingSamples()
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	require.Equal(t, "/v1/responses/compact", upstreamPath)
	require.Equal(t, "explicit-target", body["model"])
	require.Equal(t, map[string]interface{}{"keep": "yes"}, body["metadata"])
	require.Len(t, state.AttemptRecords, 1)
	require.Equal(t, "explicit", state.AttemptRecords[0].MappingRuleID)
}

func TestRelayMappedCandidatesMainLoop(t *testing.T) {
	tests := []struct {
		name         string
		mapping      string
		rejectAll    bool
		unmappedNext bool
		override     string
		status       int
		code         string
		message      string
		want         []string
		switches     int
		failed       bool
	}{
		{name: "fixed_override_does_not_replay_target", override: `{"model":"X"}`, want: []string{"X", "C"}, switches: 1},
		{name: "fixed_override_equal_first_target", override: `{"model":"A"}`, want: []string{"A", "C"}, switches: 1},
		{name: "unrelated_legacy_override_keeps_candidates", override: `{"temperature":0}`, want: []string{"A", "B"}},
		{name: "operations_override_fails_closed", override: `{"operations":[{"mode":"set","path":"model","value":"X"}]}`, want: []string{"X", "C"}, switches: 1},
		{name: "same_channel_success", want: []string{"A", "B"}},
		{name: "exhaust_then_next_channel", rejectAll: true, want: []string{"A", "B", "C"}, switches: 1},
		{name: "restore_unmapped_channel", rejectAll: true, unmappedNext: true, want: []string{"A", "B", "recovery-integration-model"}, switches: 1},
		{name: "v2_direct_priority", mapping: `{"version":2,"rules":[{"id":"b","from":"recovery-integration-model","to":"B","priority":1},{"id":"a","from":"recovery-integration-model","to":"A","priority":9},{"id":"b-to-other","from":"B","to":"WRONG"}]}`, want: []string{"A", "B"}},
		{name: "global_budget", mapping: `{"recovery-integration-model":["A","B","D","E","F","G","H"]}`, rejectAll: true, want: []string{"A", "B", "D", "E", "F", "G"}, failed: true},
		{name: "auth_skips_candidates", status: 401, code: "invalid_api_key", message: "invalid api key", want: []string{"A", "C"}, switches: 1},
		{name: "generic_rate_limit_skips_candidates", status: 429, code: "rate_limit_exceeded", message: "rate limit exceeded", want: []string{"A", "C"}, switches: 1},
		{name: "model_rate_limit", status: 429, code: "model_rate_limit_exceeded", message: "model rate limit exceeded", want: []string{"A", "B"}},
		{name: "model_capacity", status: 503, code: "model_capacity_exceeded", message: "model capacity exceeded", want: []string{"A", "B"}},
	}
	for protocolIndex, protocol := range []string{"chat", "responses"} {
		for caseIndex, tc := range tests {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				db := setupEmptyStreamRecoveryDB(t)
				// Isolate channel health/capability caches from other integration fixtures.
				firstID := 12000 + protocolIndex*100 + caseIndex*2
				var mu sync.Mutex
				var targets []string
				var bodies []map[string]interface{}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					raw, _ := io.ReadAll(r.Body)
					var body map[string]interface{}
					_ = common.Unmarshal(raw, &body)
					target, _ := body["model"].(string)
					mu.Lock()
					targets = append(targets, target)
					bodies = append(bodies, body)
					mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					if target == "A" || target == "X" || (tc.rejectAll && target != "C" && target != "recovery-integration-model") {
						status, code, message := tc.status, tc.code, tc.message
						if status == 0 {
							status = 404
							code = "model_not_found"
							message = "model not found"
						}
						w.WriteHeader(status)
						_, _ = fmt.Fprintf(w, `{"error":{"message":%q,"type":"invalid_request_error","code":%q}}`, message, code)
						return
					}
					if protocol == "chat" {
						_, _ = io.WriteString(w, `{"id":"ok","object":"chat.completion","model":"B","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`)
					} else {
						_, _ = io.WriteString(w, `{"id":"ok","object":"response","status":"completed","model":"B","output":[],"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}}`)
					}
				}))
				defer upstream.Close()
				for idx, id := range []int{firstID, firstID + 1} {
					channel := recoveryIntegrationChannel(id, "mapped", upstream.URL, int64(20-idx*10), "recovery-integration-model")
					channel.Type = constant.ChannelTypeOpenAI
					mapping := tc.mapping
					if mapping == "" {
						mapping = `{"recovery-integration-model":["A","B"]}`
					}
					if idx == 1 {
						mapping = `{"recovery-integration-model":"C"}`
						if tc.unmappedNext {
							mapping = `{"version":2,"rules":[]}`
						}
					}
					channel.ModelMapping = common.GetPointer(mapping)
					channel.SetSetting(dto.ChannelSettings{PassThroughBodyEnabled: tc.override == ""})
					if tc.override != "" && idx == 0 {
						channel.ParamOverride = common.GetPointer(tc.override)
					}
					require.NoError(t, db.Create(channel).Error)
					require.NoError(t, channel.AddAbilities(nil))
				}
				seedRecoveryPrincipal(t, db)
				model.InitChannelCache()
				name := protocol + "-" + tc.name
				router, _, _ := recoveryIntegrationRouter("mapping-" + name)
				var state *relaycommon.ChannelFailoverState
				path := "/v1/mapping-test"
				format := types.RelayFormat(types.RelayFormatOpenAIResponses)
				payload := `{"model":"recovery-integration-model","input":"hello","metadata":{"keep":"yes"},"temperature":0}`
				if protocol == "chat" {
					format = types.RelayFormatOpenAI
					payload = `{"model":"recovery-integration-model","messages":[{"role":"user","content":"hello"}],"metadata":{"keep":"yes"},"temperature":0}`
				}
				router.POST(path, func(c *gin.Context) {
					if protocol == "chat" {
						c.Request.URL.Path = "/v1/chat/completions"
					} else {
						c.Request.URL.Path = "/v1/responses"
					}
				}, middleware.Distribute(), func(c *gin.Context) {
					Relay(c, format)
					raw, _ := c.Get(relaycommon.ChannelFailoverContextKey)
					state, _ = raw.(*relaycommon.ChannelFailoverState)
					require.Equal(t, "recovery-integration-model", common.GetContextKeyString(c, constant.ContextKeyClientModel))
					require.Equal(t, "recovery-integration-model", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
				})
				recorder := httptest.NewRecorder()
				req := httptest.NewRequest("POST", path, bytes.NewBufferString(payload))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, req)
				perfmetrics.WaitForPendingSamples()
				if tc.failed {
					require.Equal(t, 503, recorder.Code, recorder.Body.String())
				} else {
					require.Equal(t, 200, recorder.Code, recorder.Body.String())
				}
				mu.Lock()
				defer mu.Unlock()
				require.Equal(t, tc.want, targets)
				for _, body := range bodies {
					require.Equal(t, map[string]interface{}{"keep": "yes"}, body["metadata"])
					require.Equal(t, float64(0), body["temperature"])
				}
				require.NotNil(t, state)
				require.Equal(t, len(tc.want), state.AttemptCount)
				require.Len(t, state.AttemptRecords, len(tc.want))
				require.Equal(t, tc.switches, state.SwitchCount)
				for i, attempt := range state.AttemptRecords {
					require.Equal(t, tc.want[i], attempt.UpstreamModel)
					require.Equal(t, i+1, attempt.Attempt)
				}
				if tc.name == "v2_direct_priority" {
					require.Equal(t, "a", state.AttemptRecords[0].MappingRuleID)
					require.Equal(t, "b", state.AttemptRecords[1].MappingRuleID)
				}
				require.True(t, state.AttemptedChannelIDs[firstID])
				var ledgers []model.BillingLedger
				require.NoError(t, db.Find(&ledgers).Error)
				require.Len(t, ledgers, 1)
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				if tc.failed {
					require.Equal(t, model.BillingLedgerStateRefunded, ledgers[0].State)
					require.Zero(t, user.RequestCount)
					require.Equal(t, 1_000_000, user.Quota)
				} else {
					require.Equal(t, model.BillingLedgerStateSettled, ledgers[0].State)
					require.Equal(t, 1, user.RequestCount)
				}
				var channel model.Channel
				require.NoError(t, db.First(&channel, firstID).Error)
				require.Equal(t, common.ChannelStatusEnabled, channel.Status)
			})
		}
	}
}
