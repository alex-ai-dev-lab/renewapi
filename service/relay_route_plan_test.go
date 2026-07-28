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
		&model.ModelEndpoint{},
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
	require.Equal(t, "gpt-5.5", first.BillingModel)
	require.Equal(t, "gpt-5.5", first.RequiredModel)

	require.True(t, plan.Advance())
	second, ok := plan.Current()
	require.True(t, ok)
	require.Equal(t, "gpt-5.4", second.RoutingModel)
	require.Equal(t, "gpt-5.5", second.BillingModel)
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

func TestBuildResponsesRelayRoutePlanSelectsHigherPriorityChannelAlternativeBeforeLowerPriorityExact(t *testing.T) {
	supported := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	highPriority := routePlanTestChannel(71, "high-priority", "gpt-5.5,gpt-5.6", 20, nil)
	highPriority.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6": supported,
		},
	}})
	lowerPriority := routePlanTestChannel(89, "lower-priority", "gpt-5.5", 10, nil)
	lowerPriority.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": supported,
		},
	}})
	setupRelayRoutePlanTestDB(t, highPriority, lowerPriority)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	require.NoError(t, model.UpsertChannelModelCapability(model.ChannelModelCapability{
		ChannelId:          highPriority.Id,
		ModelName:          "gpt-5.5",
		Capability:         model.ChannelCapabilityResponsesCompaction,
		Status:             model.ChannelCapabilityStatusSupported,
		LegacyStatus:       model.ChannelCapabilityStatusUnsupported,
		NativeStatus:       model.ChannelCapabilityStatusUnsupported,
		ContinuationStatus: model.ChannelCapabilityStatusSupported,
		RouteFingerprint:   ResponsesObservedRouteFingerprint(highPriority, "gpt-5.5"),
		Source:             "test",
	}))

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:        "default",
		ClientModel:  "gpt-5.5",
		PrimaryModel: "gpt-5.5",
		Requirement:  &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		Request:      &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: []byte(`"test"`)},
	})
	require.NoError(t, err)
	entries := routePlanEntries(plan)
	require.GreaterOrEqual(t, len(entries), 2)
	require.Equal(t, 71, entries[0].PreferredChannelId)
	require.Equal(t, "gpt-5.6", entries[0].RoutingModel)
	require.Equal(t, 89, entries[1].PreferredChannelId)
	require.Equal(t, "gpt-5.5", entries[1].RoutingModel)
}

func TestBuildResponsesRelayRoutePlanHonorsPinnedChannelAndTokenModelLimit(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2, Continuation: true}
	pinned := routePlanTestChannel(71, "pinned", "gpt-5.5,gpt-5.6,gpt-5.7", 10, capability)
	other := routePlanTestChannel(89, "other", "gpt-5.5,gpt-5.6,gpt-5.7", 20, capability)
	setupRelayRoutePlanTestDB(t, pinned, other)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Group:           "default",
		ClientModel:     "gpt-5.5",
		PrimaryModel:    "gpt-5.5",
		PinnedChannelId: 71,
		Requirement:     &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		Request:         &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: []byte(`"test"`)},
		TokenModelAllowed: func(modelName string) bool {
			return modelName != "gpt-5.7"
		},
	})
	require.NoError(t, err)
	entries := routePlanEntries(plan)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		require.Equal(t, 71, entry.PreferredChannelId)
		require.NotEqual(t, "gpt-5.7", entry.RoutingModel)
	}
}

func TestBuildResponsesRelayRoutePlanPreservesAutoGroupOrder(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	firstGroup := routePlanTestChannel(71, "first-group", "gpt-5.5", 1, capability)
	firstGroup.Group = "first"
	secondGroup := routePlanTestChannel(89, "second-group", "gpt-5.5", 100, capability)
	secondGroup.Group = "second"
	setupRelayRoutePlanTestDB(t, firstGroup, secondGroup)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")

	plan, err := BuildResponsesRelayRoutePlan(ResponsesRelayRoutePlanParams{
		Groups:       []string{"first", "second"},
		ClientModel:  "gpt-5.5",
		PrimaryModel: "gpt-5.5",
		Requirement:  &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		Request:      &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: []byte(`"test"`)},
	})
	require.NoError(t, err)
	entries := routePlanEntries(plan)
	require.Len(t, entries, 2)
	require.Equal(t, "first", entries[0].Group)
	require.Equal(t, 71, entries[0].PreferredChannelId)
	require.Equal(t, "second", entries[1].Group)
	require.Equal(t, 89, entries[1].PreferredChannelId)
}

func TestBuildResponsesRelayRoutePlanDistinguishesStrictAndFallbackAffinity(t *testing.T) {
	capability := &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}
	highPriority := routePlanTestChannel(71, "high-priority", "gpt-5.5", 20, capability)
	affinity := routePlanTestChannel(89, "affinity", "gpt-5.5", 10, capability)
	setupRelayRoutePlanTestDB(t, highPriority, affinity)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	params := ResponsesRelayRoutePlanParams{
		Group:              "default",
		ClientModel:        "gpt-5.5",
		PrimaryModel:       "gpt-5.5",
		PreferredChannelId: 89,
		Requirement:        &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger},
		Request:            &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: []byte(`"test"`)},
	}

	fallbackOnly, err := BuildResponsesRelayRoutePlan(params)
	require.NoError(t, err)
	require.Equal(t, 71, routePlanEntries(fallbackOnly)[0].PreferredChannelId)

	params.PreferChannelFirst = true
	strictAffinity, err := BuildResponsesRelayRoutePlan(params)
	require.NoError(t, err)
	require.Equal(t, 89, routePlanEntries(strictAffinity)[0].PreferredChannelId)
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
