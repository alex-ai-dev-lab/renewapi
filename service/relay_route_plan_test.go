package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayRoutePlanTestDB(t *testing.T, channels ...*model.Channel) {
	t.Helper()
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRetryTimes := common.RetryTimes
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RetryTimes = oldRetryTimes
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
	))
	model.DB = db
	common.MemoryCacheEnabled = true
	for _, channel := range channels {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	model.InitChannelCache()
}

func routePlanTestChannel(id int, name, models string, priority int64, capability *dto.ResponsesCompactionCapabilityRecord) *model.Channel {
	channel := &model.Channel{
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
	if capability != nil {
		channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
			DefaultCapability: capability,
		}})
	}
	return channel
}

func routePlanEntries(plan *RelayRoutePlan) []RelayModelRoles {
	if plan == nil {
		return nil
	}
	return append([]RelayModelRoles(nil), plan.routes...)
}

func TestRelayRoutePlanKeepsModelRolesSeparate(t *testing.T) {
	plan := NewRelayRoutePlan("gpt-5.5", "gpt-5.6-sol", "gpt-5.5", []string{"gpt-5.6-sol", "gpt-5.4"})
	require.Equal(t, 2, plan.Len())

	first, ok := plan.Current()
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", first.ClientModel)
	require.Equal(t, "gpt-5.6-sol", first.RoutingModel)
	require.Equal(t, "gpt-5.6-sol", first.BillingModel)
	require.Equal(t, "gpt-5.5", first.RequiredModel)

	require.True(t, plan.Advance())
	second, ok := plan.Current()
	require.True(t, ok)
	require.Equal(t, "gpt-5.4", second.RoutingModel)
	require.False(t, plan.Advance())
}

func TestRelayRoutePlanDefaultsClientAndDeduplicatesCaseInsensitively(t *testing.T) {
	plan := NewRelayRoutePlan("", "gpt-5.5", "", []string{"GPT-5.5", "", "gpt-5.4"})
	require.Equal(t, 2, plan.Len())
	first, ok := plan.Current()
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", first.ClientModel)
}

func TestBuildResponsesRelayRoutePlanOrdersInitialExactThenVersionsAndChannels(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{
		Capability:   dto.CompactionNativeV2,
		NativeStream: true,
		Continuation: true,
	}
	channelA := routePlanTestChannel(71, "provider-a", "gpt-5.4,gpt-5.5,gpt-5.6", 10, capability)
	channelB := routePlanTestChannel(89, "provider-b", "gpt-5.5", 20, capability)
	setupRelayRoutePlanTestDB(t, channelA, channelB)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	t.Setenv("RESPONSES_COMPACTION_MAX_ROUTE_CANDIDATES", "12")

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:            "default",
		ClientModel:      "gpt-5.5",
		PrimaryModel:     "gpt-5.5",
		InitialChannelId: 71,
		Requirement:      &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger, ClientStream: true},
	})
	require.NoError(t, err)
	entries := routePlanEntries(plan)
	require.Len(t, entries, 4)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.6", "gpt-5.4", "gpt-5.5"}, []string{
		entries[0].RoutingModel,
		entries[1].RoutingModel,
		entries[2].RoutingModel,
		entries[3].RoutingModel,
	})
	require.Equal(t, []int{71, 71, 71, 89}, []int{
		entries[0].PreferredChannelId,
		entries[1].PreferredChannelId,
		entries[2].PreferredChannelId,
		entries[3].PreferredChannelId,
	})
}

func TestBuildResponsesRelayRoutePlanRequiresInitialContextRoute(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	initial := routePlanTestChannel(71, "initial", "gpt-5.4", 10, capability)
	fallback := routePlanTestChannel(89, "fallback", "gpt-5.5", 20, capability)
	setupRelayRoutePlanTestDB(t, initial, fallback)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")

	_, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:            "default",
		ClientModel:      "gpt-5.5",
		PrimaryModel:     "gpt-5.5",
		InitialChannelId: 71,
		Requirement:      &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
	})
	require.EqualError(t, err, "initial channel/model no longer satisfies responses compaction requirements")
}

func TestBuildResponsesRelayRoutePlanFiltersRequiredContinuationChannel(t *testing.T) {
	compaction := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	continuation := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2, Continuation: true}
	compatible := routePlanTestChannel(71, "compatible", "gpt-5.5,gpt-5.6", 10, nil)
	compatible.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": continuation,
			"gpt-5.6": compaction,
		},
	}})
	missingContinuationModel := routePlanTestChannel(89, "missing-continuation", "gpt-5.6", 20, &compaction)
	setupRelayRoutePlanTestDB(t, compatible, missingContinuationModel)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:         "default",
		ClientModel:   "gpt-5.6",
		PrimaryModel:  "gpt-5.6",
		RequiredModel: "gpt-5.5",
		Requirement: &ResponsesRoutingRequirement{
			Kind:                      dto.ResponsesCompactionTrigger,
			RequiredContinuationModel: "gpt-5.5",
		},
	})
	require.NoError(t, err)
	for _, entry := range routePlanEntries(plan) {
		require.Equal(t, 71, entry.PreferredChannelId)
		require.Equal(t, "gpt-5.5", entry.RequiredModel)
	}
}

func TestBuildResponsesRelayRoutePlanStrictExcludesUnknownObserveIncludesIt(t *testing.T) {
	known := routePlanTestChannel(71, "known", "gpt-5.5", 20, &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2})
	unknown := routePlanTestChannel(89, "unknown", "gpt-5.5", 10, nil)
	setupRelayRoutePlanTestDB(t, known, unknown)
	params := ResponsesRelayRoutePlanParams{
		Group:        "default",
		ClientModel:  "gpt-5.5",
		PrimaryModel: "gpt-5.5",
		Requirement:  &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
	}

	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	strictPlan, err := BuildResponsesRelayRoutePlan(params)
	require.NoError(t, err)
	require.Len(t, routePlanEntries(strictPlan), 1)

	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "observe")
	observePlan, err := BuildResponsesRelayRoutePlan(params)
	require.NoError(t, err)
	require.Len(t, routePlanEntries(observePlan), 2)
}

func TestBuildResponsesRelayRoutePlanHonorsLimitRetryBudgetAndProviderOrder(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	channelA := routePlanTestChannel(71, "provider-a", "gpt-5.4,gpt-5.5,gpt-5.6", 10, capability)
	channelA.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: 5}
	channelB := routePlanTestChannel(89, "provider-b", "gpt-5.5", 10, capability)
	setupRelayRoutePlanTestDB(t, channelA, channelB)
	common.RetryTimes = 2
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	t.Setenv("RESPONSES_COMPACTION_MAX_ROUTE_CANDIDATES", "2")

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:        "default",
		ClientModel:  "gpt-5.5",
		PrimaryModel: "gpt-5.5",
		Requirement:  &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		ProviderPolicy: &ProviderRoutingPolicy{
			Order: []string{"provider-b", "provider-a"},
		},
	})
	require.NoError(t, err)
	entries := routePlanEntries(plan)
	require.Len(t, entries, 2)
	require.Equal(t, 89, entries[0].PreferredChannelId)
	require.Equal(t, 0, entries[0].RetryBudget)
	require.Equal(t, 71, entries[1].PreferredChannelId)
	require.Equal(t, 2, entries[1].RetryBudget)
}

func TestStrictPreferredChannelCannotEscapeToOrdinaryPool(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	preferred := routePlanTestChannel(71, "preferred", "gpt-5.5", 10, capability)
	alternative := routePlanTestChannel(89, "alternative", "gpt-5.5", 20, capability)
	setupRelayRoutePlanTestDB(t, preferred, alternative)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)

	channel, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:                    ctx,
		TokenGroup:             "default",
		ModelName:              "gpt-5.5",
		Retry:                  common.GetPointer(0),
		ExcludedChannelIds:     map[int]bool{71: true},
		PreferredChannelId:     71,
		StrictPreferredChannel: true,
		ResponsesRequirement:   &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
	})
	require.NoError(t, err)
	require.Equal(t, "default", group)
	require.Nil(t, channel)
}
