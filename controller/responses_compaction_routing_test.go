package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResponsesRequirementForRelayKeepsCompactionClientModel(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeResponses,
		ResponsesRequestKind: dto.ResponsesCompactionTrigger,
		OriginModelName:      "gpt-5.6-sol",
		ClientModelName:      "gpt-5.5",
		IsStream:             true,
	}

	requirement := responsesRequirementForRelay(info)
	require.NotNil(t, requirement)
	require.Equal(t, dto.ResponsesCompactionTrigger, requirement.Kind)
	require.True(t, requirement.ClientStream)
	require.Equal(t, "gpt-5.5", requirement.RequiredContinuationModel)
}

func TestResponsesRequirementForRelayDoesNotConstrainNormalRequests(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeResponses,
		ResponsesRequestKind: dto.ResponsesNormal,
		OriginModelName:      "gpt-5.5",
		ClientModelName:      "gpt-5.5",
	}

	requirement := responsesRequirementForRelay(info)
	require.NotNil(t, requirement)
	require.Empty(t, requirement.RequiredContinuationModel)
}

func TestPrepareDistributorResponsesRoutePlanAdvancesBeforePricingWhenFirstRouteBecomesInvalid(t *testing.T) {
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

	capability := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	first := controllerRouteTestChannel(71, "first", 20, capability)
	second := controllerRouteTestChannel(89, "second", 10, capability)
	for _, channel := range []*model.Channel{first, second} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	model.InitChannelCache()
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")

	request := &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: []byte(`[{"type":"compaction_trigger"}]`)}
	plan, err := service.BuildResponsesRelayRoutePlan(service.ResponsesRelayRoutePlanParams{
		Group:        "default",
		ClientModel:  "gpt-5.5",
		PrimaryModel: "gpt-5.5",
		Requirement:  &service.ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		Request:      request,
	})
	require.NoError(t, err)

	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyResponsesRelayRoutePlan, plan)
	require.Nil(t, middleware.SetupContextForSelectedChannel(ctx, first, "gpt-5.5"))

	first.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionDisabled},
	}})
	require.NoError(t, db.Save(first).Error)
	model.InitChannelCache()

	info := &relaycommon.RelayInfo{
		Request:              request,
		RelayMode:            relayconstant.RelayModeResponses,
		ResponsesRequestKind: dto.ResponsesCompactionTrigger,
		OriginModelName:      "gpt-5.5",
		ClientModelName:      "gpt-5.5",
		UsingGroup:           "default",
		TokenGroup:           "default",
	}
	require.Nil(t, prepareDistributorResponsesRoutePlan(ctx, info))
	require.Equal(t, 89, common.GetContextKeyInt(ctx, constant.ContextKeyChannelId))
	require.Equal(t, "gpt-5.5", info.OriginModelName)
}

func controllerRouteTestChannel(id int, name string, priority int64, capability dto.ResponsesCompactionCapabilityRecord) *model.Channel {
	channel := &model.Channel{
		Id:            id,
		ConfigVersion: 1,
		Type:          constant.ChannelTypeOpenAI,
		Key:           "test-key",
		Status:        common.ChannelStatusEnabled,
		Name:          name,
		Models:        "gpt-5.5",
		Group:         "default",
		Priority:      common.GetPointer(priority),
		BaseURL:       common.GetPointer("https://api.example.com/v1"),
	}
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		DefaultCapability: &capability,
	}})
	return channel
}
