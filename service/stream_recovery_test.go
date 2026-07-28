package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	metricdto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func resetStreamRecoveryStateForTest() {
	streamRecoveryEvidence.Lock()
	streamRecoveryEvidence.items = make(map[string]streamRecoveryEvidenceState)
	streamRecoveryEvidence.lastSweep = time.Time{}
	streamRecoveryEvidence.Unlock()
	streamRecoveryModelLabels.Lock()
	streamRecoveryModelLabels.values = make(map[string]struct{})
	streamRecoveryModelLabels.Unlock()
}

func streamRecoveryContextForTest(fingerprint string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ginKeyChannelAffinityMeta, channelAffinityMeta{KeyFingerprint: fingerprint})
	return c
}

func TestRecordDistinctStreamFailureRequiresDistinctSessions(t *testing.T) {
	resetStreamRecoveryStateForTest()
	setting := operation_setting.GetStreamRecoverySetting()
	old := *setting
	t.Cleanup(func() { *setting = old })
	setting.ChannelModelEscalationEnabled = true
	setting.DistinctSessionThreshold = 2
	setting.EvidenceWindowSeconds = 60

	require.False(t, RecordDistinctStreamFailure(streamRecoveryContextForTest("session-a"), 197, "gpt-5.6-sol"))
	require.False(t, RecordDistinctStreamFailure(streamRecoveryContextForTest("session-a"), 197, "gpt-5.6-sol"))
	require.True(t, RecordDistinctStreamFailure(streamRecoveryContextForTest("session-b"), 197, "gpt-5.6-sol"))
	require.False(t, RecordDistinctStreamFailure(streamRecoveryContextForTest("session-b"), 197, "gpt-5.6-sol"))
}

func TestStreamRecoveryEvidenceIsBounded(t *testing.T) {
	resetStreamRecoveryStateForTest()
	now := time.Now()
	streamRecoveryEvidence.Lock()
	for i := 0; i < maxStreamRecoveryEvidence; i++ {
		streamRecoveryEvidence.items[fmt.Sprintf("channel-%d", i)] = streamRecoveryEvidenceState{
			fingerprints: map[string]time.Time{"fp": now},
			lastObserved: now.Add(time.Duration(i) * time.Nanosecond),
		}
	}
	evictOldestStreamRecoveryEvidenceLocked()
	require.Len(t, streamRecoveryEvidence.items, maxStreamRecoveryEvidence-1)
	_, retainedOldest := streamRecoveryEvidence.items["channel-0"]
	streamRecoveryEvidence.Unlock()
	require.False(t, retainedOldest)
}

func TestStreamRecoveryModelLabelsAreBounded(t *testing.T) {
	resetStreamRecoveryStateForTest()
	for i := 0; i < maxStreamRecoveryModelLabels; i++ {
		require.Equal(t, fmt.Sprintf("model-%d", i), streamRecoveryModelLabel(fmt.Sprintf("model-%d", i)))
	}
	require.Equal(t, "other", streamRecoveryModelLabel("model-overflow"))
	require.Equal(t, "other", streamRecoveryModelLabel(string(make([]byte, 129))))
}

func TestStreamRecoveryCollectorRegistersAndCounts(t *testing.T) {
	resetStreamRecoveryStateForTest()
	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(streamRecoveryEvents))

	status := relaycommon.NewStreamStatus()
	status.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	status.SetTransportEnd(relaycommon.StreamEndReasonEOF, nil)
	status.Finalize()
	info := &relaycommon.RelayInfo{OriginModelName: "metric-test-model", StreamStatus: status}
	counter := streamRecoveryEvents.WithLabelValues("197", "metric-test-model", "empty_response", "detected")
	beforeMetric := &metricdto.Metric{}
	require.NoError(t, counter.Write(beforeMetric))
	before := beforeMetric.GetCounter().GetValue()
	RecordStreamRecoveryEvent(streamRecoveryContextForTest("safe-fingerprint"), info, 197, "detected")
	afterMetric := &metricdto.Metric{}
	require.NoError(t, counter.Write(afterMetric))
	after := afterMetric.GetCounter().GetValue()
	require.Equal(t, before+1, after)

	families, err := registry.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, families)
}
