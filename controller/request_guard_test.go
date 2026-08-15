package controller

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/requestguard"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestRequestGuardConfigViewNeverReturnsSecret(t *testing.T) {
	key := requestguard.EndpointSecretOptionKey("primary")
	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[key]
	common.OptionMap[key] = "super-secret-value"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if optionMapWasNil {
			common.OptionMap = nil
			return
		}
		if existed {
			common.OptionMap[key] = previous
		} else {
			delete(common.OptionMap, key)
		}
	})

	view := buildRequestGuardConfigView(operation_setting.RequestGuardSetting{Endpoints: []operation_setting.RequestGuardEndpoint{{ID: "primary"}}})
	encoded, err := common.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "super-secret-value")
	require.Contains(t, string(encoded), `"has_secret":true`)
	require.Contains(t, string(encoded), `"secret_status":"configured"`)
}

func TestRequestGuardDefaultCollectionsEncodeAsArrays(t *testing.T) {
	view := buildRequestGuardConfigView(operation_setting.GetRequestGuardSetting())
	encoded, err := common.Marshal(view)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"groups":[]`)
	require.Contains(t, string(encoded), `"endpoints":[]`)
	require.NotContains(t, string(encoded), `"groups":null`)
	require.NotContains(t, string(encoded), `"endpoints":null`)
}

type relayAdmissionCounters struct {
	routePlan atomic.Int64
	price     atomic.Int64
	pre       atomic.Int64
	channel   atomic.Int64
	upstream  atomic.Int64
}

func installRelayAdmissionSeams(t *testing.T, guardErr *types.NewAPIError) *relayAdmissionCounters {
	t.Helper()
	previousPreflight := relayRequestPreflight
	previousRoutePlan := relayPrepareRoutePlan
	previousPrice := relayPriceHelper
	previousPreConsume := relayPreConsume
	previousChannel := relaySelectChannel
	previousDispatch := relayDispatchUpstream
	counters := &relayAdmissionCounters{}

	relayRequestPreflight = func(*gin.Context, *relaycommon.RelayInfo, dto.Request) *types.NewAPIError {
		return guardErr
	}
	relayPrepareRoutePlan = func(*gin.Context, *relaycommon.RelayInfo) *types.NewAPIError {
		counters.routePlan.Add(1)
		return nil
	}
	relayPriceHelper = func(_ *gin.Context, info *relaycommon.RelayInfo, _ int, _ *types.TokenCountMeta) (types.PriceData, error) {
		counters.price.Add(1)
		price := types.PriceData{QuotaToPreConsume: 1}
		info.PriceData = price
		return price, nil
	}
	relayPreConsume = func(*gin.Context, int, *relaycommon.RelayInfo) *types.NewAPIError {
		counters.pre.Add(1)
		return nil
	}
	relaySelectChannel = func(*gin.Context, *relaycommon.RelayInfo, *service.RetryParam) (*model.Channel, *types.NewAPIError) {
		counters.channel.Add(1)
		autoBan := 0
		return &model.Channel{Type: constant.ChannelTypeOpenAI, AutoBan: &autoBan}, nil
	}
	relayDispatchUpstream = func(*gin.Context, *relaycommon.RelayInfo, types.RelayFormat, *websocket.Conn) *types.NewAPIError {
		counters.upstream.Add(1)
		return nil
	}

	t.Cleanup(func() {
		relayRequestPreflight = previousPreflight
		relayPrepareRoutePlan = previousRoutePlan
		relayPriceHelper = previousPrice
		relayPreConsume = previousPreConsume
		relaySelectChannel = previousChannel
		relayDispatchUpstream = previousDispatch
	})
	return counters
}

func performRelayAdmissionRequest(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	previousCountToken := constant.CountToken
	previousSensitive := setting.CheckSensitiveEnabled
	constant.CountToken = false
	setting.CheckSensitiveEnabled = false
	t.Cleanup(func() {
		constant.CountToken = previousCountToken
		setting.CheckSensitiveEnabled = previousSensitive
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewBufferString(`{"model":"guard-order-model","messages":[{"role":"user","content":"hello"}]}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "guard-order-model")
	common.SetContextKey(c, constant.ContextKeyClientModel, "guard-order-model")
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	Relay(c, types.RelayFormatOpenAI)
	return recorder
}

func TestRequestGuardPreflightStopsAllDownstreamRelayWork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		guardErr   *types.NewAPIError
		wantStatus int
		wantCalls  int64
	}{
		{
			name: "blocked",
			guardErr: types.NewErrorWithStatusCode(
				errors.New("request blocked by RequestGuard policy"),
				types.ErrorCodeRequestGuardBlocked,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			),
			wantStatus: http.StatusForbidden,
		},
		{
			name: "fail closed unavailable",
			guardErr: types.NewErrorWithStatusCode(
				errors.New("RequestGuard is temporarily unavailable"),
				types.ErrorCodeRequestGuardUnavailable,
				http.StatusServiceUnavailable,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			),
			wantStatus: http.StatusServiceUnavailable,
		},
		{name: "allow", wantStatus: http.StatusOK, wantCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			counters := installRelayAdmissionSeams(t, test.guardErr)
			recorder := performRelayAdmissionRequest(t)
			require.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, test.wantCalls, counters.routePlan.Load(), "route plan")
			require.Equal(t, test.wantCalls, counters.price.Load(), "price helper")
			require.Equal(t, test.wantCalls, counters.pre.Load(), "preconsume")
			require.Equal(t, test.wantCalls, counters.channel.Load(), "channel selection")
			require.Equal(t, test.wantCalls, counters.upstream.Load(), "upstream dispatch")
		})
	}
}
