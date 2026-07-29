package controller

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResponsesEmptyStreamRecoversAcrossChannelsExactlyOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		modelName  = "recovery-integration-model"
		requestID  = "req-empty-stream-recovery-integration"
		sessionKey = "session-empty-stream-recovery-integration"
	)
	requestBody := []byte(`{"model":"recovery-integration-model","input":"hello","stream":true,"prompt_cache_key":"session-empty-stream-recovery-integration"}`)

	type upstreamObservation struct {
		sync.Mutex
		calls int
		body  []byte
	}
	var upstreamA, upstreamB upstreamObservation
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamA.Lock()
		upstreamA.calls++
		upstreamA.body = append([]byte(nil), body...)
		upstreamA.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		time.Sleep(6 * time.Millisecond)
	}))
	t.Cleanup(serverA.Close)
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamB.Lock()
		upstreamB.calls++
		upstreamB.body = append([]byte(nil), body...)
		upstreamB.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_b\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"recovery-integration-model\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"from-b\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_b\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"recovery-integration-model\",\"output\":[],\"usage\":{\"input_tokens\":11,\"output_tokens\":3,\"total_tokens\":14}}}\n\n")
	}))
	t.Cleanup(serverB.Close)

	db := setupEmptyStreamRecoveryDB(t)
	channelA := recoveryIntegrationChannel(197, "empty-a", serverA.URL, 20, modelName)
	channelB := recoveryIntegrationChannel(201, "complete-b", serverB.URL, 10, modelName)
	for _, channel := range []*model.Channel{channelA, channelB} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "recovery-user", Password: "test-password", Status: common.UserStatusEnabled, Quota: 1_000_000, Group: "default"}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "sk-recovery-test", Status: common.TokenStatusEnabled, Name: "recovery-token", RemainQuota: 1_000_000}).Error)
	model.InitChannelCache()

	seedRecoveryAffinity(t, requestBody, modelName, sessionKey, channelA.Id)

	var usedChannels []string
	var finalChannelID int
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, 1)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-recovery-test")
		c.Set("token_name", "recovery-token")
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, false)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
		common.SetContextKey(c, common.RequestIdKey, requestID)
		c.Next()
	})
	router.POST("/v1/responses", middleware.Distribute(), func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAIResponses)
		usedChannels = append([]string(nil), c.GetStringSlice("use_channel")...)
		finalChannelID = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "response.completed")
	require.Contains(t, recorder.Body.String(), "from-b")
	require.NotContains(t, recorder.Body.String(), "empty-a")
	require.Equal(t, []string{"197", "201"}, usedChannels)
	require.Equal(t, channelB.Id, finalChannelID)

	upstreamA.Lock()
	require.Equal(t, 1, upstreamA.calls)
	bodyA := append([]byte(nil), upstreamA.body...)
	upstreamA.Unlock()
	upstreamB.Lock()
	require.Equal(t, 1, upstreamB.calls)
	bodyB := append([]byte(nil), upstreamB.body...)
	upstreamB.Unlock()
	require.JSONEq(t, string(bodyA), string(bodyB))
	require.Contains(t, string(bodyB), sessionKey)

	var ledgers []model.BillingLedger
	require.NoError(t, db.Where("request_id = ?", requestID).Find(&ledgers).Error)
	require.Len(t, ledgers, 1)
	require.Equal(t, model.BillingLedgerStateSettled, ledgers[0].State)
	require.True(t, ledgers[0].RequestCounted)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1, user.RequestCount)

	var storedA model.Channel
	require.NoError(t, db.First(&storedA, channelA.Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, storedA.Status)

	affinityCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	affinityCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
	affinityCtx.Request.Header.Set("Content-Type", "application/json")
	preferred, found := service.GetPreferredChannelByAffinity(affinityCtx, modelName, "default")
	require.True(t, found)
	require.Equal(t, channelB.Id, preferred)
	perfmetrics.WaitForPendingSamples()
}

func TestResponsesAllEmptyStreamsRefundExactlyOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		modelName = "recovery-integration-model"
		requestID = "req-all-empty-streams-refund-integration"
	)
	requestBody := []byte(`{"model":"recovery-integration-model","input":"hello","stream":true,"prompt_cache_key":"session-all-empty-streams"}`)

	var callsA, callsB int
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callsA++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(serverA.Close)
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callsB++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(serverB.Close)

	db := setupEmptyStreamRecoveryDB(t)
	for _, channel := range []*model.Channel{
		recoveryIntegrationChannel(197, "empty-a", serverA.URL, 20, modelName),
		recoveryIntegrationChannel(201, "empty-b", serverB.URL, 10, modelName),
	} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	seedRecoveryPrincipal(t, db)
	model.InitChannelCache()

	router, usedChannels, _ := recoveryIntegrationRouter(requestID)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, callsA)
	require.Equal(t, 1, callsB)
	require.Equal(t, []string{"197", "201"}, *usedChannels)

	var ledgers []model.BillingLedger
	require.NoError(t, db.Where("request_id = ?", requestID).Find(&ledgers).Error)
	require.Len(t, ledgers, 1)
	require.Equal(t, model.BillingLedgerStateRefunded, ledgers[0].State)
	require.False(t, ledgers[0].RequestCounted)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 1_000_000, user.Quota)
	require.Zero(t, user.RequestCount)
	var token model.Token
	require.NoError(t, db.First(&token, 1).Error)
	require.Equal(t, 1_000_000, token.RemainQuota)
	perfmetrics.WaitForPendingSamples()
}

func TestResponsesCommittedIncompleteStreamDoesNotAppendJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		modelName = "recovery-integration-model"
		requestID = "req-committed-incomplete-stream-integration"
	)
	requestBody := []byte(`{"model":"recovery-integration-model","input":"hello","stream":true,"prompt_cache_key":"session-committed-incomplete"}`)

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_partial\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"recovery-integration-model\"}}\n\n")
	}))
	t.Cleanup(server.Close)

	db := setupEmptyStreamRecoveryDB(t)
	channel := recoveryIntegrationChannel(197, "partial-a", server.URL, 20, modelName)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	seedRecoveryPrincipal(t, db)
	model.InitChannelCache()

	router, usedChannels, _ := recoveryIntegrationRouter(requestID)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, calls)
	require.Equal(t, []string{"197"}, *usedChannels)
	require.Contains(t, recorder.Body.String(), "response.created")
	require.NotContains(t, recorder.Body.String(), `{"error":`)
	require.NotContains(t, recorder.Body.String(), `"code":"bad_response_body"`)
	perfmetrics.WaitForPendingSamples()
}

func seedRecoveryPrincipal(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "recovery-user", Password: "test-password", Status: common.UserStatusEnabled, Quota: 1_000_000, Group: "default"}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "sk-recovery-test", Status: common.TokenStatusEnabled, Name: "recovery-token", RemainQuota: 1_000_000}).Error)
}

func recoveryIntegrationRouter(requestID string) (*gin.Engine, *[]string, *int) {
	usedChannels := make([]string, 0, 2)
	finalChannelID := 0
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, 1)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-recovery-test")
		c.Set("token_name", "recovery-token")
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, false)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
		common.SetContextKey(c, common.RequestIdKey, requestID)
		c.Next()
	})
	router.POST("/v1/responses", middleware.Distribute(), func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAIResponses)
		usedChannels = append(usedChannels[:0], c.GetStringSlice("use_channel")...)
		finalChannelID = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	})
	return router, &usedChannels, &finalChannelID
}

func setupEmptyStreamRecoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldRetryTimes := common.RetryTimes
	oldAffinity := *operation_setting.GetChannelAffinitySetting()
	oldRecovery := *operation_setting.GetStreamRecoverySetting()
	oldRatios := ratio_setting.ModelRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	oldOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		oldOptionMap[key] = value
	}
	common.OptionMapRWMutex.RUnlock()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{},
		&model.ChannelModelStatus{}, &model.ChannelModelCapability{}, &model.ModelEndpoint{},
		&model.BillingLedger{}, &model.BillingOutbox{}, &model.Option{}, &model.Task{}, &model.Midjourney{},
		&model.Log{},
	))
	model.DB, model.LOG_DB = db, db
	model.InitOptionMap()
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.LogConsumeEnabled = false
	common.RetryTimes = 0
	t.Setenv("BILLING_LEDGER_MODE", service.BillingLedgerModeEnforce)
	t.Setenv("CHANNEL_AFFINITY_FALLBACK_ONLY", "false")

	recovery := operation_setting.GetStreamRecoverySetting()
	recovery.Enabled = true
	recovery.PreCommitRetryEnabled = true
	recovery.EmptyStreamRetryLimit = 1
	recovery.SessionRouteRepairEnabled = true
	recovery.PostCommitRouteRepairEnabled = true
	recovery.SessionNegativeTTLSeconds = 90
	recovery.UnknownFailureNegativeTTLSeconds = 30
	recovery.KeyNegativeTTLSeconds = 90
	recovery.MaxCrossRequestRouteChanges = 2
	recovery.RecoveryChainWindowSeconds = 60
	recovery.ChannelModelEscalationEnabled = false
	affinity := operation_setting.GetChannelAffinitySetting()
	affinity.Enabled = true
	affinity.SwitchOnSuccess = true
	affinity.DefaultTTLSeconds = 3600
	affinity.Rules = []operation_setting.ChannelAffinityRule{{
		Name: "recovery-test", ModelRegex: []string{"^recovery-integration-model$"}, PathRegex: []string{"^/v1/responses$"},
		KeySources:         []operation_setting.ChannelAffinityKeySource{{Type: "gjson", Path: "prompt_cache_key"}},
		SkipRetryOnFailure: false, IncludeUsingGroup: true, IncludeRuleName: true,
	}}

	var ratios map[string]float64
	require.NoError(t, common.Unmarshal([]byte(oldRatios), &ratios))
	ratios["recovery-integration-model"] = 1
	ratioBytes, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratioBytes)))
	service.ClearChannelAffinityCacheAll()

	t.Cleanup(func() {
		service.ClearChannelAffinityCacheAll()
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatios))
		*operation_setting.GetStreamRecoverySetting() = oldRecovery
		*operation_setting.GetChannelAffinitySetting() = oldAffinity
		common.RetryTimes = oldRetryTimes
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
		if oldDB != nil && oldMemoryCacheEnabled {
			model.InitChannelCache()
		}
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func recoveryIntegrationChannel(id int, name, baseURL string, priority int64, modelName string) *model.Channel {
	return &model.Channel{
		Id: id, Type: constant.ChannelTypeOpenAI, Key: "sk-upstream-test", Status: common.ChannelStatusEnabled,
		Name: name, Models: modelName, Group: "default", Priority: common.GetPointer(priority), Weight: common.GetPointer(uint(1)),
		BaseURL: common.GetPointer(baseURL),
	}
}

func seedRecoveryAffinity(t *testing.T, body []byte, modelName, sessionKey string, channelID int) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	preferred, found := service.GetPreferredChannelByAffinity(c, modelName, "default")
	require.False(t, found)
	require.Zero(t, preferred)
	service.RecordChannelAffinity(c, channelID)

	verify, _ := gin.CreateTestContext(httptest.NewRecorder())
	verify.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	verify.Request.Header.Set("Content-Type", "application/json")
	preferred, found = service.GetPreferredChannelByAffinity(verify, modelName, "default")
	require.True(t, found, sessionKey)
	require.Equal(t, channelID, preferred)
}
