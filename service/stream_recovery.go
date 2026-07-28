package service

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var streamRecoveryEvents = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "newapi_stream_recovery_events_total",
		Help: "Responses stream recovery outcomes and actions.",
	},
	[]string{"channel_id", "model", "outcome", "action"},
)

const (
	maxStreamRecoveryModelLabels = 256
	maxStreamRecoveryEvidence    = 4096
)

var streamRecoveryModelLabels = struct {
	sync.Mutex
	values map[string]struct{}
}{values: make(map[string]struct{})}

func StreamRecoveryCollectors() []prometheus.Collector {
	return []prometheus.Collector{streamRecoveryEvents}
}

func RecordStreamRecoveryEvent(c *gin.Context, info *relaycommon.RelayInfo, channelID int, action string) {
	if info == nil || info.StreamStatus == nil || channelID <= 0 {
		return
	}
	outcome := info.StreamStatus.Outcome()
	modelName := streamRecoveryModelLabel(info.OriginModelName)
	action = strings.TrimSpace(action)
	if action == "" {
		action = "detected"
	}
	streamRecoveryEvents.WithLabelValues(strconv.Itoa(channelID), modelName, string(outcome.Code), action).Inc()
	keyFingerprint := ""
	if meta, ok := getChannelAffinityMeta(c); ok {
		keyFingerprint = meta.KeyFingerprint
	}
	logger.LogInfo(c, fmt.Sprintf(
		"stream recovery: channel=%d model=%s outcome=%s action=%s key_fp=%s committed=%t retryable=%t",
		channelID, modelName, outcome.Code, action, keyFingerprint,
		outcome.ClientCommitted, outcome.RetryableBeforeCommit,
	))
}

func streamRecoveryModelLabel(modelName string) string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if modelName == "" {
		return "unknown"
	}
	if len(modelName) > 128 {
		return "other"
	}
	streamRecoveryModelLabels.Lock()
	defer streamRecoveryModelLabels.Unlock()
	if _, ok := streamRecoveryModelLabels.values[modelName]; ok {
		return modelName
	}
	if len(streamRecoveryModelLabels.values) >= maxStreamRecoveryModelLabels {
		return "other"
	}
	streamRecoveryModelLabels.values[modelName] = struct{}{}
	return modelName
}

type streamRecoveryEvidenceState struct {
	fingerprints map[string]time.Time
	escalated    bool
	lastObserved time.Time
}

var streamRecoveryEvidence = struct {
	sync.Mutex
	items     map[string]streamRecoveryEvidenceState
	lastSweep time.Time
}{items: make(map[string]streamRecoveryEvidenceState)}

// RecordDistinctStreamFailure returns true only when a channel/model crosses
// the configured distinct-session threshold in the current evidence window.
func RecordDistinctStreamFailure(c *gin.Context, channelID int, modelName string) bool {
	setting := operation_setting.GetStreamRecoverySetting()
	if setting == nil || !setting.ChannelModelEscalationEnabled || channelID <= 0 {
		return false
	}
	meta, ok := getChannelAffinityMeta(c)
	fingerprint := strings.TrimSpace(meta.KeyFingerprint)
	if !ok || fingerprint == "" {
		return false
	}
	threshold := setting.DistinctSessionThreshold
	if threshold < 2 {
		threshold = 3
	}
	window := time.Duration(setting.EvidenceWindowSeconds) * time.Second
	if window <= 0 {
		window = time.Minute
	}

	key := strconv.Itoa(channelID) + "\n" + strings.ToLower(strings.TrimSpace(modelName))
	now := time.Now()
	streamRecoveryEvidence.Lock()
	defer streamRecoveryEvidence.Unlock()
	sweepStreamRecoveryEvidenceLocked(now, window)
	if _, exists := streamRecoveryEvidence.items[key]; !exists && len(streamRecoveryEvidence.items) >= maxStreamRecoveryEvidence {
		evictOldestStreamRecoveryEvidenceLocked()
	}
	state := streamRecoveryEvidence.items[key]
	if state.fingerprints == nil {
		state.fingerprints = make(map[string]time.Time)
	}
	for fp, observedAt := range state.fingerprints {
		if now.Sub(observedAt) > window {
			delete(state.fingerprints, fp)
		}
	}
	if len(state.fingerprints) < threshold {
		state.escalated = false
	}
	state.fingerprints[fingerprint] = now
	state.lastObserved = now
	crossed := len(state.fingerprints) >= threshold && !state.escalated
	if crossed {
		state.escalated = true
	}
	streamRecoveryEvidence.items[key] = state
	return crossed
}

func sweepStreamRecoveryEvidenceLocked(now time.Time, window time.Duration) {
	if !streamRecoveryEvidence.lastSweep.IsZero() && now.Sub(streamRecoveryEvidence.lastSweep) < window {
		return
	}
	for key, state := range streamRecoveryEvidence.items {
		for fingerprint, observedAt := range state.fingerprints {
			if now.Sub(observedAt) > window {
				delete(state.fingerprints, fingerprint)
			}
		}
		if len(state.fingerprints) == 0 || now.Sub(state.lastObserved) > window {
			delete(streamRecoveryEvidence.items, key)
			continue
		}
		streamRecoveryEvidence.items[key] = state
	}
	streamRecoveryEvidence.lastSweep = now
}

func evictOldestStreamRecoveryEvidenceLocked() {
	oldestKey := ""
	var oldest time.Time
	for key, state := range streamRecoveryEvidence.items {
		if oldestKey == "" || state.lastObserved.Before(oldest) {
			oldestKey = key
			oldest = state.lastObserved
		}
	}
	if oldestKey != "" {
		delete(streamRecoveryEvidence.items, oldestKey)
	}
}
