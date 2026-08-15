package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateResolvedChannelRouteRejectsCodexThroughOtherAdaptor(t *testing.T) {
	err := validateResolvedChannelRoute(
		&model.Channel{Id: 151, Type: constant.ChannelTypeCodex},
		constant.ChannelTypeAnthropic,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "codex channel #151")
}

func TestValidateResolvedChannelRouteAllowsNonCodexProtocolOverride(t *testing.T) {
	err := validateResolvedChannelRoute(
		&model.Channel{Id: 71, Type: constant.ChannelTypeOpenAI},
		constant.ChannelTypeAnthropic,
	)

	require.NoError(t, err)
}

func TestGetModelFromJSONBodyKeepsFullResponsesRequestForRouting(t *testing.T) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	body := `{"model":"gpt-5.5","input":"hello","previous_response_id":"resp_1","conversation":{"id":"conv_1"},"tools":[{"type":"mcp","server_label":"x"}]}`
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

	modelRequest, err := getModelFromJSONBody(ctx)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", modelRequest.Model)
	routingRequest, ok := modelRequest.RoutingRequest.(*dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Equal(t, "resp_1", routingRequest.PreviousResponseID)
	require.JSONEq(t, `{"id":"conv_1"}`, string(routingRequest.Conversation))
	require.JSONEq(t, `[{"type":"mcp","server_label":"x"}]`, string(routingRequest.Tools))

	raw, found := common.GetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest)
	require.True(t, found)
	require.Same(t, routingRequest, raw)
}

func TestGetModelFromJSONBodyUsesCompactionDTOForCompactEndpoint(t *testing.T) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	body := `{"model":"gpt-5.5","input":[{"role":"user","content":"hello"}],"previous_response_id":"resp_1"}`
	ctx.Request = httptest.NewRequest("POST", "/v1/responses/compact", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

	modelRequest, err := getModelFromJSONBody(ctx)
	require.NoError(t, err)
	routingRequest, ok := modelRequest.RoutingRequest.(*dto.OpenAIResponsesCompactionRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", routingRequest.Model)
	require.Equal(t, "resp_1", routingRequest.PreviousResponseID)
}

func TestResponsesRoutingRequirementLeavesNormalResponsesUnconstrained(t *testing.T) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hello"}`))

	ctx.Set(service.ContextKeyResponsesRequestKind, dto.ResponsesNormal)
	require.Nil(t, responsesRoutingRequirement(ctx))

	ctx.Set(service.ContextKeyResponsesRequestKind, dto.ResponsesCompactionTrigger)
	require.NotNil(t, responsesRoutingRequirement(ctx))
	require.Equal(t, dto.ResponsesCompactionTrigger, responsesRoutingRequirement(ctx).Kind)
}

func TestDistributeNormalResponsesDoesNotRequireCompactionCapability(t *testing.T) {
	db := setupDistributorFallbackTestDB(t)
	channel := distributorRouteTestChannel(71, "normal-only", "gpt-5.5", 20)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()

	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	t.Setenv("RESPONSES_COMPACTION_ROUTE_PLAN_ENABLED", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/v1/responses",
		distributorRouteAuthContext(),
		Distribute(),
		func(c *gin.Context) {
			require.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
			_, found := common.GetContextKey(c, constant.ContextKeyResponsesRelayRoutePlan)
			require.False(t, found)
			c.Status(http.StatusNoContent)
		},
	)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
}

func TestDistributeInstallsHighestPriorityChannelModelRouteBeforeRelay(t *testing.T) {
	require.NoError(t, appI18n.Init())
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		if oldDB != nil && oldMemoryCacheEnabled {
			model.InitChannelCache()
		}
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelModelStatus{},
		&model.ChannelModelCapability{},
		&model.ModelEndpoint{},
	))
	model.DB = db
	common.MemoryCacheEnabled = true

	supported := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	highPriority := distributorRouteTestChannel(71, "high", "gpt-5.5,gpt-5.6", 20)
	highPriority.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6": supported,
		},
	}})
	lowerPriority := distributorRouteTestChannel(89, "low", "gpt-5.5", 10)
	lowerPriority.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{"gpt-5.5": supported},
	}})
	for _, channel := range []*model.Channel{highPriority, lowerPriority} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	model.InitChannelCache()
	require.NoError(t, model.UpsertChannelModelCapability(model.ChannelModelCapability{
		ChannelId:          highPriority.Id,
		ModelName:          "gpt-5.5",
		Capability:         model.ChannelCapabilityResponsesCompaction,
		Status:             model.ChannelCapabilityStatusSupported,
		LegacyStatus:       model.ChannelCapabilityStatusUnsupported,
		NativeStatus:       model.ChannelCapabilityStatusUnsupported,
		ContinuationStatus: model.ChannelCapabilityStatusSupported,
		RouteFingerprint:   service.ResponsesObservedRouteFingerprint(highPriority, "gpt-5.5"),
		Source:             "test",
	}))

	t.Setenv("RESPONSES_COMPACTION_ROUTE_PLAN_ENABLED", "true")
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	t.Setenv("RESPONSES_COMPACTION_MAX_MODELS_PER_CHANNEL", "3")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/responses",
		func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
			c.Next()
		},
		Distribute(),
		func(c *gin.Context) {
			require.Equal(t, 71, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
			require.Equal(t, "gpt-5.6", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
			plan, found := common.GetContextKey(c, constant.ContextKeyResponsesRelayRoutePlan)
			require.True(t, found)
			require.NotNil(t, plan)
			c.Status(http.StatusNoContent)
		},
	)
	body := `{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
}

func TestDistributeInitialSelectionUsesOnlyExplicitFallbackModels(t *testing.T) {
	require.NoError(t, appI18n.Init())
	db := setupDistributorFallbackTestDB(t)
	fallback := distributorRouteTestChannel(121, "fallback", "gpt-fallback", 10)
	require.NoError(t, db.Create(fallback).Error)
	require.NoError(t, fallback.AddAbilities(nil))
	model.InitChannelCache()

	t.Setenv("REQUEST_MODELS_FALLBACK_ENABLED", "true")
	t.Setenv("REQUEST_MODELS_FALLBACK_MAX", "4")
	gin.SetMode(gin.TestMode)

	t.Run("explicit fallback advances initial selection", func(t *testing.T) {
		router := gin.New()
		router.POST("/v1/chat/completions", distributorRouteAuthContext(), Distribute(), func(c *gin.Context) {
			require.Equal(t, fallback.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
			require.Equal(t, "gpt-primary", common.GetContextKeyString(c, constant.ContextKeyClientModel))
			require.Equal(t, "gpt-fallback", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
			require.Equal(t, []string{"gpt-fallback"}, common.GetContextKeyStringSlice(c, constant.ContextKeyFallbackModels))
			c.Status(http.StatusNoContent)
		})
		body := `{"model":"gpt-primary","models":["gpt-fallback"]}`
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	})

	t.Run("missing explicit fallback never changes model", func(t *testing.T) {
		router := gin.New()
		router.POST("/v1/chat/completions", distributorRouteAuthContext(), Distribute(), func(c *gin.Context) {
			t.Fatal("request without a primary channel must not reach relay")
		})
		body := `{"model":"gpt-primary"}`
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.Contains(t, response.Body.String(), `"code":"model_not_found"`)
		require.Contains(t, response.Body.String(), "gpt-primary")
	})

	t.Run("token whitelist still filters explicit fallback", func(t *testing.T) {
		router := gin.New()
		router.POST("/v1/chat/completions", func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-primary": true})
			c.Next()
		}, Distribute(), func(c *gin.Context) {
			t.Fatal("token-forbidden fallback must not reach relay")
		})
		body := `{"model":"gpt-primary","models":["gpt-fallback"]}`
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
	})

	t.Run("provider policy still filters explicit fallback", func(t *testing.T) {
		t.Setenv("PROVIDER_ROUTING_CONTROL_ENABLED", "true")
		router := gin.New()
		router.POST("/v1/chat/completions", distributorRouteAuthContext(), Distribute(), func(c *gin.Context) {
			t.Fatal("provider-forbidden fallback must not reach relay")
		})
		body := `{"model":"gpt-primary","models":["gpt-fallback"],"provider":{"only":["anthropic"]}}`
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
	})
}

func TestSelectDistributorInitialChannelReturnsRealSelectionError(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyFallbackModels, []string{"gpt-primary", "gpt-fallback"})

	selectorCalls := make([]string, 0, 2)
	channel, _, selectErr := selectDistributorInitialChannelWithSelector(ctx, &ModelRequest{Model: "gpt-primary"}, "default", nil, "", func(param *service.RetryParam) (*model.Channel, string, error) {
		selectorCalls = append(selectorCalls, param.ModelName)
		return nil, "default", errors.New("database unavailable")
	})
	require.Nil(t, channel)
	require.Error(t, selectErr)
	require.Equal(t, []string{"gpt-primary"}, selectorCalls)
	require.Contains(t, strings.ToLower(selectErr.Error()), "database unavailable")
}

func setupDistributorFallbackTestDB(t *testing.T) *gorm.DB {
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		if oldDB != nil && oldMemoryCacheEnabled {
			model.InitChannelCache()
		}
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelModelStatus{},
		&model.ChannelModelCapability{},
		&model.ModelEndpoint{},
	))
	model.DB = db
	common.MemoryCacheEnabled = true
	return db
}

func distributorRouteAuthContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		c.Next()
	}
}

func distributorRouteTestChannel(id int, name, models string, priority int64) *model.Channel {
	return &model.Channel{
		Id:            id,
		ConfigVersion: 1,
		Type:          constant.ChannelTypeOpenAI,
		Key:           "test-key",
		Status:        common.ChannelStatusEnabled,
		Name:          name,
		Models:        models,
		Group:         "default",
		Priority:      common.GetPointer(priority),
		BaseURL:       common.GetPointer("https://api.example.com/v1"),
	}
}
