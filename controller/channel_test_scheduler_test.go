package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
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
