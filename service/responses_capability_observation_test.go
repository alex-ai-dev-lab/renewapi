package service

import (
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResponsesCapabilityTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		model.ReloadChannelModelCapabilityCache()
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelCapability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	model.ReloadChannelModelCapabilityCache()
}

func compactTestChannel() *model.Channel {
	return &model.Channel{
		Id:            71,
		Type:          constant.ChannelTypeOpenAI,
		Models:        "gpt-5.5",
		BaseURL:       common.GetPointer("https://api.example.com/v1"),
		ConfigVersion: 1,
	}
}

func TestObservedNativeCapabilityEnablesStrictRouting(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := compactTestChannel()

	outcome := ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind:         dto.ResponsesCompactionTrigger,
		ClientStream: true,
	}, nil)
	require.True(t, outcome.Related)
	require.False(t, outcome.Unsupported)
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{
		Kind:         dto.ResponsesCompactionTrigger,
		ClientStream: true,
	}, nil))
	record := EffectiveResponsesCompactionRecord(channel, "gpt-5.5")
	require.Equal(t, dto.CompactionNativeV2, record.Capability)
	require.True(t, record.NativeStream)
}

func TestObservedLegacyUnsupportedDoesNotBecomeChannelFailure(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := compactTestChannel()
	err := types.NewErrorWithStatusCode(
		errors.New("unknown endpoint /v1/responses/compact"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusNotFound,
	)
	outcome := ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind:       dto.ResponsesCompactEndpoint,
		UsedLegacy: true,
	}, err)
	require.True(t, outcome.Unsupported)
	require.False(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{
		Kind: dto.ResponsesCompactEndpoint,
	}, nil))

	transient := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.False(t, ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactionTrigger,
	}, transient).Unsupported)
}

func TestLegacyModelNotFoundIsNotEndpointUnsupported(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	channel := compactTestChannel()
	for _, message := range []string{
		`{"error":{"code":"model_not_found","message":"model not found"}}`,
		"unknown model gpt-5.5",
		"deployment_not_found: deployment not found",
	} {
		err := types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeBadResponseStatusCode, http.StatusNotFound)
		outcome := ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
			Kind:       dto.ResponsesCompactEndpoint,
			UsedLegacy: true,
		}, err)
		require.False(t, outcome.Unsupported)
	}
	_, found := model.GetChannelModelCapability(channel.Id, "gpt-5.5", model.ChannelCapabilityResponsesCompaction)
	require.False(t, found)
}

func TestExplicitUnsupportedInvalidRequestIsCapabilityEvidence(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	channel := compactTestChannel()
	err := types.NewErrorWithStatusCode(
		errors.New("unsupported input type: compaction_trigger"),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
	)
	outcome := ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactionTrigger,
	}, err)
	require.True(t, outcome.Unsupported)
	record, found := model.GetChannelModelCapability(channel.Id, "gpt-5.5", model.ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, model.ChannelCapabilityStatusUnsupported, record.NativeStatus)
}

func TestManualCapabilityOverridesObservedEvidence(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := compactTestChannel()
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": {Capability: dto.CompactionLegacy},
		},
	}})
	err := types.NewErrorWithStatusCode(errors.New("unknown endpoint /v1/responses/compact"), types.ErrorCodeBadResponseStatusCode, http.StatusNotFound)
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind:       dto.ResponsesCompactEndpoint,
		UsedLegacy: true,
	}, err)
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{
		Kind: dto.ResponsesCompactEndpoint,
	}, nil))
}

func TestManualUnknownFallsBackToObservedEvidence(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := compactTestChannel()
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown},
	}})
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactionTrigger,
	}, nil)
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{
		Kind: dto.ResponsesCompactionTrigger,
	}, nil))
}

func TestObservedCapabilityInvalidatesOnChannelConfigChange(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := compactTestChannel()
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{Kind: dto.ResponsesCompactionTrigger}, nil)
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger}, nil))
	channel.ConfigVersion++
	require.False(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger}, nil))

	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{Kind: dto.ResponsesCompactionTrigger}, nil)
	record, found := model.GetChannelModelCapability(channel.Id, "gpt-5.5", model.ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, model.ChannelCapabilityStatusSupported, record.NativeStatus)
	require.Equal(t, model.ChannelCapabilityStatusUnknown, record.LegacyStatus)
	require.Equal(t, model.ChannelCapabilityStatusUnknown, record.ContinuationStatus)
}

func TestObservedCapabilityConfigChangeDoesNotReviveOldFacets(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	channel := compactTestChannel()
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind:       dto.ResponsesCompactEndpoint,
		UsedLegacy: true,
	}, nil)
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactedContextContinuation,
	}, nil)

	channel.ConfigVersion++
	ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactionTrigger,
	}, nil)
	record, found := model.GetChannelModelCapability(channel.Id, "gpt-5.5", model.ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, model.ChannelCapabilityStatusSupported, record.NativeStatus)
	require.Equal(t, model.ChannelCapabilityStatusUnknown, record.LegacyStatus)
	require.Equal(t, model.ChannelCapabilityStatusUnknown, record.ContinuationStatus)
}

func TestObservedCapabilitySerializesConcurrentFacetUpdates(t *testing.T) {
	setupResponsesCapabilityTestDB(t)
	channel := compactTestChannel()
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, attempt := range []ResponsesCapabilityAttempt{
		{Kind: dto.ResponsesCompactEndpoint, UsedLegacy: true},
		{Kind: dto.ResponsesCompactionTrigger},
	} {
		attempt := attempt
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ObserveResponsesCapabilityAttempt(channel, "gpt-5.5", attempt, nil)
		}()
	}
	close(start)
	wg.Wait()
	record, found := model.GetChannelModelCapability(channel.Id, "gpt-5.5", model.ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, model.ChannelCapabilityStatusSupported, record.LegacyStatus)
	require.Equal(t, model.ChannelCapabilityStatusSupported, record.NativeStatus)
}
