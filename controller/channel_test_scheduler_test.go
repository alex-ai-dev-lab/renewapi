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
		DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionLegacy},
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6": {Capability: dto.CompactionNativeV2},
		},
	}})

	require.Equal(t, []string{"gpt-5.3", "gpt-5.4", "gpt-5.6", "gpt-5.7"}, responsesCompactionProbeModels(channel))
}
