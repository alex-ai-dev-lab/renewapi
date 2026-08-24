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
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func requestGuardControllerSetting(endpoints ...operation_setting.RequestGuardEndpoint) operation_setting.RequestGuardSetting {
	return operation_setting.RequestGuardSetting{
		Enabled:             true,
		Mode:                operation_setting.RequestGuardModeEnforce,
		FailurePolicy:       operation_setting.RequestGuardFailureClosed,
		InputMode:           operation_setting.RequestGuardInputFullClientControlled,
		MaxInputRunes:       16000,
		EvaluationTimeoutMs: 2500,
		Scope: operation_setting.RequestGuardScope{
			AllGroups: true,
			Models:    []string{"*"},
			Protocols: []string{"openai_chat"},
		},
		Bulkhead:  operation_setting.RequestGuardBulkhead{MaxConcurrent: 64, MaxPerEndpoint: 16},
		Observe:   operation_setting.RequestGuardObserve{WorkerCount: 4, QueueCapacity: 4096},
		Endpoints: endpoints,
	}
}

func requestGuardControllerEndpoint(id string, priority int) operation_setting.RequestGuardEndpoint {
	return operation_setting.RequestGuardEndpoint{
		ID: id, Enabled: true, Priority: priority,
		BaseURL: "https://guard.example/v1", Model: "guard",
		Codec: operation_setting.RequestGuardCodecJSONPolicy,
		TimeoutMs: 1500, InputLimitRunes: 16000,
		ProxyPolicy: operation_setting.RequestGuardProxyDisabled,
	}
}

func installRequestGuardControllerState(
	t *testing.T,
	current operation_setting.RequestGuardSetting,
	secrets map[string]string,
) string {
	t.Helper()

	oldDB := model.DB
	oldSetting := operation_setting.GetRequestGuardSetting()
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model.DB = oldDB
		operation_setting.ApplyRequestGuardSetting(oldSetting)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	model.DB = db
	operation_setting.ApplyRequestGuardSetting(current)

	encoded, err := common.Marshal(current)
	require.NoError(t, err)
	baseline := string(encoded)
	options := []model.Option{{Key: "request_guard_setting", Value: baseline}}
	common.OptionMapRWMutex.Lock()
	common.OptionMap["request_guard_setting"] = baseline
	for endpointID, secret := range secrets {
		key := requestguard.EndpointSecretOptionKey(endpointID)
		options = append(options, model.Option{Key: key, Value: secret})
		common.OptionMap[key] = secret
	}
	common.OptionMapRWMutex.Unlock()
	require.NoError(t, db.Create(&options).Error)
	return baseline
}

func requestGuardUpdatePayload(setting operation_setting.RequestGuardSetting) requestGuardConfigUpdate {
	endpoints := make([]requestGuardEndpointUpdate, 0, len(setting.Endpoints))
	for _, endpoint := range setting.Endpoints {
		endpoints = append(endpoints, requestGuardEndpointUpdate{RequestGuardEndpoint: endpoint})
	}
	return requestGuardConfigUpdate{
		Enabled: setting.Enabled, Mode: setting.Mode, FailurePolicy: setting.FailurePolicy,
		InputMode: setting.InputMode, MaxInputRunes: setting.MaxInputRunes,
		EvaluationTimeoutMs: setting.EvaluationTimeoutMs, Scope: setting.Scope,
		Bulkhead: setting.Bulkhead, Observe: setting.Observe,
		StorePassEvents: setting.StorePassEvents,
		StoreRedactedPreview: setting.StoreRedactedPreview,
		Endpoints: endpoints,
	}
}

func performRequestGuardConfigUpdate(t *testing.T, payload requestGuardConfigUpdate) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/request-guard/config", bytes.NewReader(encoded))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateRequestGuardConfig(c)
	return recorder
}

func TestUpdateRequestGuardConfigClearsRemovedEndpointSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	primary := requestGuardControllerEndpoint("primary", 100)
	secondary := requestGuardControllerEndpoint("secondary", 50)
	current := requestGuardControllerSetting(primary, secondary)
	installRequestGuardControllerState(t, current, map[string]string{
		"primary":   "primary-secret",
		"secondary": "secondary-secret",
	})

	next := requestGuardControllerSetting(secondary)
	recorder := performRequestGuardConfigUpdate(t, requestGuardUpdatePayload(next))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var configOption model.Option
	require.NoError(t, model.DB.First(&configOption, "key = ?", "request_guard_setting").Error)
	var stored operation_setting.RequestGuardSetting
	require.NoError(t, common.Unmarshal([]byte(configOption.Value), &stored))
	require.Len(t, stored.Endpoints, 1)
	require.Equal(t, "secondary", stored.Endpoints[0].ID)

	primarySecretKey := requestguard.EndpointSecretOptionKey("primary")
	var primarySecret model.Option
	require.NoError(t, model.DB.First(&primarySecret, "key = ?", primarySecretKey).Error)
	require.Empty(t, primarySecret.Value)

	secondarySecretKey := requestguard.EndpointSecretOptionKey("secondary")
	var secondarySecret model.Option
	require.NoError(t, model.DB.First(&secondarySecret, "key = ?", secondarySecretKey).Error)
	require.Equal(t, "secondary-secret", secondarySecret.Value)

	common.OptionMapRWMutex.RLock()
	require.Empty(t, common.OptionMap[primarySecretKey])
	require.Equal(t, "secondary-secret", common.OptionMap[secondarySecretKey])
	common.OptionMapRWMutex.RUnlock()

	runtimeSetting := operation_setting.GetRequestGuardSetting()
	require.Len(t, runtimeSetting.Endpoints, 1)
	require.Equal(t, "secondary", runtimeSetting.Endpoints[0].ID)
}

func TestUpdateRequestGuardConfigRejectsInvalidPayloadWithoutMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	primary := requestGuardControllerEndpoint("primary", 100)
	current := requestGuardControllerSetting(primary)
	baseline := installRequestGuardControllerState(t, current, map[string]string{"primary": "primary-secret"})

	invalid := requestGuardControllerSetting(primary)
	invalid.Endpoints[0].BaseURL = "not-an-absolute-url"
	recorder := performRequestGuardConfigUpdate(t, requestGuardUpdatePayload(invalid))
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())

	var configOption model.Option
	require.NoError(t, model.DB.First(&configOption, "key = ?", "request_guard_setting").Error)
	require.Equal(t, baseline, configOption.Value)

	secretKey := requestguard.EndpointSecretOptionKey("primary")
	var secretOption model.Option
	require.NoError(t, model.DB.First(&secretOption, "key = ?", secretKey).Error)
	require.Equal(t, "primary-secret", secretOption.Value)

	common.OptionMapRWMutex.RLock()
	require.Equal(t, baseline, common.OptionMap["request_guard_setting"])
	require.Equal(t, "primary-secret", common.OptionMap[secretKey])
	common.OptionMapRWMutex.RUnlock()

	runtimeSetting := operation_setting.GetRequestGuardSetting()
	require.Len(t, runtimeSetting.Endpoints, 1)
	require.Equal(t, "https://guard.example/v1", runtimeSetting.Endpoints[0].BaseURL)
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
