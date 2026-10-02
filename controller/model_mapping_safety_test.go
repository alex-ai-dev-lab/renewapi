package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mappingBillingState struct {
	recordingBillingSettler
	retryable bool
}

func (s *mappingBillingState) CanRetryRequest() bool { return s.retryable }

func TestMappedCandidateSafetyGates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*gin.Context, *relaycommon.RelayInfo, *service.RetryParam, *types.NewAPIError)
		want   bool
	}{
		{name: "active_preconsume_is_not_settlement", want: true},
		{name: "active_zero_preconsume", want: true, mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.Billing = &mappingBillingState{retryable: true}
		}},
		{name: "terminal_billing", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.Billing = &mappingBillingState{recordingBillingSettler: recordingBillingSettler{reserved: 10}}
		}},
		{name: "skip_retry", mutate: func(_ *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, e *types.NewAPIError) {
			*e = *types.NewOpenAIError(errors.New("model not found"), types.ErrorCodeModelNotFound, 404, types.ErrOptionWithSkipRetry())
		}},
		{name: "client_output", mutate: func(c *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			_, _ = c.Writer.Write([]byte("started"))
		}},
		{name: "usage_observed", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.ResponsesObservedUsage = &dto.Usage{TotalTokens: 1}
		}},
		{name: "stream_received", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.ReceivedResponseCount = 1
		}},
		{name: "stream_attempt", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.StreamStatus = &relaycommon.StreamStatus{}
		}},
		{name: "unknown_billing_implementation", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.Billing = &recordingBillingSettler{}
		}},
		{name: "cancelled", mutate: func(c *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			ctx, cancel := context.WithCancel(c.Request.Context())
			cancel()
			c.Request = c.Request.WithContext(ctx)
		}},
		{name: "strict_plan", mutate: func(_ *gin.Context, _ *relaycommon.RelayInfo, p *service.RetryParam, _ *types.NewAPIError) {
			p.StrictPreferredChannel = true
		}},
		{name: "specific_binding", mutate: func(c *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			c.Set("specific_channel_id", 91)
		}},
		{name: "global_budget", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.Failover.AttemptCount = 6
		}},
		{name: "async_accepted", mutate: func(_ *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, e *types.NewAPIError) {
			e.StatusCode = http.StatusAccepted
		}},
		{name: "task_mode", mutate: func(_ *gin.Context, i *relaycommon.RelayInfo, _ *service.RetryParam, _ *types.NewAPIError) {
			i.RelayMode = relayconstant.RelayModeImagesGenerations
		}},
		{name: "unknown_timeout", mutate: func(_ *gin.Context, _ *relaycommon.RelayInfo, _ *service.RetryParam, e *types.NewAPIError) {
			*e = *types.NewOpenAIError(errors.New("context deadline exceeded"), types.ErrorCodeDoRequestFailed, 504)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 91}, ModelMappingRoute: relaycommon.ModelMappingRouteCursor{ChannelId: 91, Source: "alias", Candidates: []string{"A", "B"}}, Billing: &mappingBillingState{recordingBillingSettler: recordingBillingSettler{reserved: 10}, retryable: true}, Failover: relaycommon.NewChannelFailoverState()}
			require.True(t, info.Failover.Begin(91, "test", 0))
			p := &service.RetryParam{Retry: common.GetPointer(1), ExcludedChannelIds: map[int]bool{91: true}}
			e := types.NewOpenAIError(errors.New("model not found"), types.ErrorCodeModelNotFound, 404)
			if tc.mutate != nil {
				tc.mutate(c, info, p, e)
			}
			attempts := info.Failover.AttemptCount
			require.Equal(t, tc.want, retryNextMappedModelCandidate(c, info, p, &model.Channel{Id: 91}, e))
			require.True(t, p.ExcludedChannelIds[91])
			require.Equal(t, attempts, info.Failover.AttemptCount)
			require.Equal(t, 1, p.GetRetry())
			if !tc.want {
				require.Zero(t, info.ModelMappingRoute.Index)
			}
		})
	}
}

func TestMappedCandidateErrorScope(t *testing.T) {
	for _, tc := range []struct {
		status  int
		code    string
		message string
		safe    bool
	}{
		{404, "model_not_found", "model not found", true},
		{400, "invalid_request_error", "model is not supported", true},
		{422, "invalid_request_error", "unknown model", true},
		{400, "invalid_request_error", "invalid tools", false},
		{401, "invalid_api_key", "model not found", false},
		{403, "permission_denied", "model not found", false},
		{402, "insufficient_quota", "model not found", false},
		{429, "rate_limit_exceeded", "rate limit exceeded", false},
		{429, "insufficient_quota", "model quota exhausted", false},
		{429, "model_rate_limit_exceeded", "rate limit exceeded", true},
		{503, "model_overloaded", "capacity", true},
		{503, "model_capacity_exceeded", "capacity", true},
		{503, "server_error", "capacity", false},
		{500, "server_error", "model not found", false},
		{408, "request_timeout", "model not found", false},
		{504, "timeout", "model not found", false},
	} {
		t.Run(tc.code+http.StatusText(tc.status), func(t *testing.T) {
			e := types.NewOpenAIError(errors.New(tc.message), types.ErrorCode(tc.code), tc.status)
			require.Equal(t, tc.safe, isSafeMappedModelRejection(e))
		})
	}
}
