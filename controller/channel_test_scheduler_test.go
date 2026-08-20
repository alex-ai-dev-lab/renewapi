package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetEffectiveTestConfigUsesChannelIntervalMinutes(t *testing.T) {
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{
		AutoTestInterval:        10,
		AutoTestRetryCount:      3,
		AutoTestRetryThreshold:  2,
		AutoTestTimeWindowStart: "23:00",
		AutoTestTimeWindowEnd:   "07:00",
		AutoTestTimezone:        "Asia/Taipei",
	})

	interval, retryCount, retryThreshold, start, end, timezone := getEffectiveTestConfig(channel)

	require.Equal(t, 10*time.Minute, interval)
	require.Equal(t, 3, retryCount)
	require.Equal(t, 2, retryThreshold)
	require.Equal(t, "23:00", start)
	require.Equal(t, "07:00", end)
	require.Equal(t, "Asia/Taipei", timezone)
}

func TestIsClockInTestWindowSupportsCrossDayWindow(t *testing.T) {
	require.True(t, isClockInTestWindow("23:30", "23:00", "07:00"))
	require.True(t, isClockInTestWindow("06:59", "23:00", "07:00"))
	require.False(t, isClockInTestWindow("12:00", "23:00", "07:00"))
}

func TestIsClockInTestWindowSupportsSameDayWindow(t *testing.T) {
	require.False(t, isClockInTestWindow("08:59", "09:00", "18:00"))
	require.True(t, isClockInTestWindow("09:00", "09:00", "18:00"))
	require.True(t, isClockInTestWindow("17:59", "09:00", "18:00"))
	require.False(t, isClockInTestWindow("18:01", "09:00", "18:00"))
}

func TestChannelTestTrackerUsesPersistedTestTimeAfterRestart(t *testing.T) {
	tracker := &channelTestTracker{channelLastTest: make(map[int]time.Time)}
	persisted := time.Now().Add(-5 * time.Minute).Unix()

	since := tracker.lastTestSince(42, persisted)

	require.GreaterOrEqual(t, since, 4*time.Minute)
	require.Less(t, since, 6*time.Minute)
}

func TestChannelTestTrackerPrefersInMemoryTestTime(t *testing.T) {
	tracker := &channelTestTracker{channelLastTest: make(map[int]time.Time)}
	tracker.recordTest(42)

	since := tracker.lastTestSince(42, time.Now().Add(-time.Hour).Unix())

	require.Less(t, since, time.Second)
}

func TestResponsesCompactionProbeModelsStayExplicitlyScoped(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_MODEL", "gpt-5.7")
	t.Setenv("RESPONSES_COMPACTION_PROBE_MAX_MODELS", "10")
	channel := &model.Channel{
		Models:    "unrelated-model,gpt-5.4-openai-compact,gpt-5.7",
		TestModel: common.GetPointer("gpt-5.3"),
	}
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown},
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6": {Capability: dto.CompactionNativeV2},
		},
	}})

	require.Equal(t, []string{"gpt-5.3", "gpt-5.4", "gpt-5.7"}, responsesCompactionProbeModels(channel))
}

func TestResponsesCompactionProbeModelsSkipConcreteManualDeclarations(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_PROBE_MAX_MODELS", "10")
	channel := &model.Channel{Models: "gpt-5.4-openai-compact,gpt-5.5-openai-compact,gpt-5.6-openai-compact"}
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.4": {Capability: dto.CompactionDisabled},
			"gpt-5.5": {Capability: dto.CompactionLegacy},
			"gpt-5.6": {Capability: dto.CompactionUnknown},
		},
	}})
	require.Equal(t, []string{"gpt-5.6"}, responsesCompactionProbeModels(channel))
}

func TestResponsesCompactionProbeModelsRevalidateMeasuredDeclarations(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_PROBE_MAX_MODELS", "10")
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": {Capability: dto.CompactionNativeV2, VerifiedAt: 123},
		},
	}})
	require.Equal(t, []string{"gpt-5.5"}, responsesCompactionProbeModels(channel))
}

func TestBuildResponsesCompactionProbeRequestCoversTriggerAndContinuation(t *testing.T) {
	trigger, err := buildResponsesCompactionProbeRequest("gpt-5.5", true, nil)
	require.NoError(t, err)
	require.JSONEq(t, `[{"role":"user","content":[{"type":"input_text","text":"Compress this channel capability probe state."}]},{"type":"compaction_trigger"}]`, string(trigger.Input))
	require.NotNil(t, trigger.Stream)
	require.True(t, *trigger.Stream)

	item := []byte(`{"type":"compaction_summary","encrypted_content":"opaque"}`)
	continuation, err := buildResponsesCompactionProbeRequest("gpt-5.5", false, item)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"compaction_summary","encrypted_content":"opaque"},{"role":"user","content":[{"type":"input_text","text":"Reply with CONTINUE_OK."}]}]`, string(continuation.Input))
	require.NotNil(t, continuation.Stream)
	require.False(t, *continuation.Stream)
}

func TestResponsesCompactionObservationCompleteRequiresNativeFacets(t *testing.T) {
	record := model.ChannelModelCapability{
		LegacyStatus: model.ChannelCapabilityStatusSupported,
		NativeStatus: model.ChannelCapabilityStatusSupported,
	}
	require.False(t, responsesCompactionObservationComplete(record))
	record.NativeStreamStatus = model.ChannelCapabilityStatusSupported
	record.ContinuationStatus = model.ChannelCapabilityStatusSupported
	require.True(t, responsesCompactionObservationComplete(record))

	record = model.ChannelModelCapability{
		LegacyStatus: model.ChannelCapabilityStatusSupported,
		NativeStatus: model.ChannelCapabilityStatusUnsupported,
	}
	require.True(t, responsesCompactionObservationComplete(record))
}

func TestShouldSkipResponsesCompactionProbeRequiresCurrentFingerprint(t *testing.T) {
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ModelEndpoint{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	channel := &model.Channel{
		Id:            71,
		Type:          1,
		BaseURL:       common.GetPointer("https://api.example.com/v1"),
		ConfigVersion: 1,
	}
	now := time.Now().Unix()
	current := service.ResponsesObservedRouteFingerprint(channel, "gpt-5.6-sol")

	record := model.ChannelModelCapability{Source: "probe", NextProbeAt: now + 3600, RouteFingerprint: current}
	require.True(t, shouldSkipResponsesCompactionProbe(channel, "gpt-5.6-sol", record, true, now))

	record.RouteFingerprint = "stale-fingerprint"
	require.False(t, shouldSkipResponsesCompactionProbe(channel, "gpt-5.6-sol", record, true, now))

	record.RouteFingerprint = ""
	require.False(t, shouldSkipResponsesCompactionProbe(channel, "gpt-5.6-sol", record, true, now))

	record.RouteFingerprint = current
	record.NextProbeAt = now - 1
	require.False(t, shouldSkipResponsesCompactionProbe(channel, "gpt-5.6-sol", record, true, now))
	require.False(t, shouldSkipResponsesCompactionProbe(channel, "gpt-5.6-sol", record, false, now))
}

func TestObserveResponsesCapabilityProbeResultClassifiesUpstreamStatus(t *testing.T) {
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelCapability{}, &model.ModelEndpoint{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	channel := &model.Channel{
		Id: 71, Type: constant.ChannelTypeOpenAI, Models: "gpt-5.6-sol",
		BaseURL: common.GetPointer("https://api.example.com/v1"), ConfigVersion: 1,
	}
	apiErr := types.NewErrorWithStatusCode(errors.New("method not allowed"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	err = observeResponsesCapabilityProbeResult(channel, "gpt-5.6-sol", service.ResponsesCapabilityAttempt{
		Kind: dto.ResponsesCompactEndpoint, UsedLegacy: true, Source: "probe",
	}, testResult{localErr: apiErr, newAPIError: apiErr, httpStatus: http.StatusMethodNotAllowed})
	require.Error(t, err)
	record, found := model.GetChannelModelCapability(channel.Id, "gpt-5.6-sol", model.ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, model.ChannelCapabilityStatusUnsupported, record.LegacyStatus)
	require.Equal(t, http.StatusMethodNotAllowed, record.LastStatusCode)
}

func TestResponsesCapabilityProbeSemaphoreHonorsCancellation(t *testing.T) {
	for i := 0; i < cap(responsesCompactionProbeSemaphore); i++ {
		responsesCompactionProbeSemaphore <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(responsesCompactionProbeSemaphore); i++ {
			<-responsesCompactionProbeSemaphore
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := runResponsesCapabilityProbe(ctx, func() testResult {
		t.Fatal("probe callback must not run after cancellation")
		return testResult{}
	})
	require.ErrorIs(t, result.localErr, context.Canceled)
}
