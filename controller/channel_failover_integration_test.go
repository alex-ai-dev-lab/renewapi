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
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const failoverTestModel = "recovery-integration-model"

func writeFailoverSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_winner\",\"status\":\"in_progress\"}}\n\n")
	_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"winner\"}\n\n")
	_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_winner\",\"status\":\"completed\",\"usage\":{\"input_tokens\":11,\"output_tokens\":3,\"total_tokens\":14}}}\n\n")
}

func TestChannelFailoverUsesSixDistinctChannelsAndRemainingPriority(t *testing.T) {
	for _, successAtSix := range []bool{false, true} {
		t.Run(fmt.Sprintf("success_at_six_%t", successAtSix), func(t *testing.T) {
			idBase := 6000
			if successAtSix {
				idBase = 6100
			}
			db := setupEmptyStreamRecoveryDB(t)
			seedRecoveryPrincipal(t, db)
			common.LogConsumeEnabled = true
			// 即使旧配置为零，也必须保留请求级的五次渠道切换预算。
			common.RetryTimes = 0
			var mu sync.Mutex
			var calls []int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var id int
				_, _ = fmt.Sscanf(r.URL.Path, "/channel/%d/", &id)
				mu.Lock()
				calls = append(calls, id-idBase)
				attempt := len(calls)
				mu.Unlock()
				w.Header().Set("X-Request-Id", fmt.Sprintf("upstream-%d", id))
				if successAtSix && attempt == 6 {
					writeFailoverSuccess(w)
					return
				}
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":{"type":"upstream_private","message":"secret-provider-host.internal rejected key"}}`)
			}))
			t.Cleanup(server.Close)
			priorities := []int64{100, 100, 90, 80, 70, 60, 50, 200, 300}
			for i, priority := range priorities {
				id := idBase + i + 1
				channel := recoveryIntegrationChannel(id, fmt.Sprintf("channel-%d", id), server.URL, priority, failoverTestModel)
				// 不同路径用于识别真实 adaptor 发起的渠道请求。
				channel.BaseURL = common.GetPointer(fmt.Sprintf("%s/channel/%d", server.URL, id))
				if i == 7 {
					channel.Status = common.ChannelStatusAutoDisabled
				}
				if i == 8 {
					channel.Status = common.ChannelStatusManuallyDisabled
				}
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			model.InitChannelCache()
			router, used, _ := recoveryIntegrationRouter(fmt.Sprintf("failover-six-%t", successAtSix))
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"recovery-integration-model","input":"hi","stream":true}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)
			mu.Lock()
			got := append([]int(nil), calls...)
			mu.Unlock()
			require.Len(t, got, 6)
			require.Len(t, *used, 6)
			require.ElementsMatch(t, []int{1, 2}, got[:2])
			require.Equal(t, []int{3, 4, 5, 6}, got[2:])
			if successAtSix {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), "winner")
				require.NotContains(t, recorder.Body.String(), "secret-provider")
			} else {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				require.Equal(t, types.PublicModelCapacityMessage, gjson.Get(recorder.Body.String(), "error.message").String())
				require.Equal(t, "model_capacity", gjson.Get(recorder.Body.String(), "error.code").String())
			}
			var logs []model.Log
			require.NoError(t, db.Where("request_id = ?", fmt.Sprintf("failover-six-%t", successAtSix)).Find(&logs).Error)
			require.Len(t, logs, 1)
			attempts := gjson.Get(logs[0].Other, "admin_info.attempts").Array()
			require.Len(t, attempts, 6)
			for i, attempt := range attempts {
				require.EqualValues(t, i+1, attempt.Get("attempt").Int())
				require.NotEmpty(t, attempt.Get("upstream_request_id").String())
				if i < 5 || !successAtSix {
					require.Contains(t, attempt.Get("real_error").String(), "secret-provider-host.internal")
					health, healthErr := model.GetChannelModelStatus(int(attempt.Get("channel_id").Int()), "default", failoverTestModel)
					require.NoError(t, healthErr)
					require.Equal(t, 1, health.FailureCount)
					require.Equal(t, http.StatusTooManyRequests, health.LastStatusCode)
				}
			}
			selfLogs, err := model.GetLogByTokenId(1)
			require.NoError(t, err)
			selfJSON, err := common.Marshal(selfLogs)
			require.NoError(t, err)
			require.NotContains(t, string(selfJSON), "secret-provider-host")
			require.NotContains(t, string(selfJSON), "admin_info")
			require.NotContains(t, string(selfJSON), "upstream_private")
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			if successAtSix {
				require.Equal(t, 1, user.RequestCount)
				var ledger model.BillingLedger
				require.NoError(t, db.Where("request_id = ?", fmt.Sprintf("failover-six-%t", successAtSix)).First(&ledger).Error)
				require.Equal(t, idBase+6, ledger.ChannelID)
				var channels []model.Channel
				require.NoError(t, db.Find(&channels).Error)
				for _, channel := range channels {
					if channel.Id == ledger.ChannelID {
						require.EqualValues(t, ledger.ActualQuota, channel.UsedQuota)
					} else {
						require.Zero(t, channel.UsedQuota)
					}
				}
			} else {
				require.Zero(t, user.RequestCount)
				require.Equal(t, 1_000_000, user.Quota)
			}
			perfmetrics.WaitForPendingSamples()
		})
	}
}
