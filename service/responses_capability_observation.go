package service

import (
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

var responsesCapabilityObservationLocks [64]sync.Mutex

func responsesCapabilityObservationLock(channelID int, modelName string) *sync.Mutex {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.ToLower(strings.TrimSpace(modelName))))
	index := (uint32(channelID) ^ hash.Sum32()) % uint32(len(responsesCapabilityObservationLocks))
	return &responsesCapabilityObservationLocks[index]
}

type ResponsesCapabilityAttempt struct {
	Kind         dto.ResponsesRequestKind
	ClientStream bool
	UsedLegacy   bool
	Source       string
}

type ResponsesCapabilityOutcome struct {
	Related          bool
	Unsupported      bool
	PersistenceError error
}

func (a ResponsesCapabilityAttempt) Related() bool {
	return a.Kind != dto.ResponsesNormal
}

func compactFailureExplicitlyUnsupported(attempt ResponsesCapabilityAttempt, err *types.NewAPIError) bool {
	if err == nil || !attempt.Related() {
		return false
	}
	text := strings.ToLower(err.Error())
	explicitHints := []string{
		"compact is not supported",
		"compaction is not supported",
		"unsupported compaction",
		"compaction_trigger is not supported",
		"unsupported input type: compaction_trigger",
		"unknown input type: compaction_trigger",
		"unsupported endpoint /v1/responses/compact",
		"unknown endpoint /v1/responses/compact",
		"compaction capability mismatch",
	}
	for _, hint := range explicitHints {
		if strings.Contains(text, hint) {
			return true
		}
	}
	if err.GetErrorCode() == types.ErrorCodeInvalidRequest {
		return false
	}
	if attempt.UsedLegacy || attempt.Kind == dto.ResponsesCompactEndpoint {
		modelNotFoundHints := []string{
			"model_not_found", "model not found", "unknown model",
			"deployment_not_found", "deployment not found",
		}
		for _, hint := range modelNotFoundHints {
			if strings.Contains(text, hint) {
				return false
			}
		}
		if err.StatusCode == 404 || err.StatusCode == 405 || err.StatusCode == 501 {
			return true
		}
	}
	return false
}

func capabilityValueFromObserved(record model.ChannelModelCapability) string {
	legacy := record.LegacyStatus == model.ChannelCapabilityStatusSupported
	native := record.NativeStatus == model.ChannelCapabilityStatusSupported
	switch {
	case legacy && native:
		return string(dto.CompactionNativeV2AndLegacy)
	case legacy:
		return string(dto.CompactionLegacy)
	case native:
		return string(dto.CompactionNativeV2)
	case record.LegacyStatus == model.ChannelCapabilityStatusUnsupported &&
		record.NativeStatus == model.ChannelCapabilityStatusUnsupported:
		return string(dto.CompactionDisabled)
	default:
		return string(dto.CompactionUnknown)
	}
}

func updateObservedCapabilityFacet(record *model.ChannelModelCapability, attempt ResponsesCapabilityAttempt, status int) {
	if record == nil {
		return
	}
	switch {
	case attempt.UsedLegacy || attempt.Kind == dto.ResponsesCompactEndpoint:
		record.LegacyStatus = status
	case attempt.Kind == dto.ResponsesCompactionTrigger:
		record.NativeStatus = status
		if attempt.ClientStream {
			record.NativeStreamStatus = status
		}
	case attempt.Kind == dto.ResponsesCompactedContextContinuation:
		record.ContinuationStatus = status
		if status == model.ChannelCapabilityStatusSupported {
			record.NativeStatus = status
		}
	}
	record.NativeStream = record.NativeStreamStatus == model.ChannelCapabilityStatusSupported
	record.Continuation = record.ContinuationStatus == model.ChannelCapabilityStatusSupported
	record.CapabilityValue = capabilityValueFromObserved(*record)
	if record.LegacyStatus == model.ChannelCapabilityStatusSupported || record.NativeStatus == model.ChannelCapabilityStatusSupported {
		record.Status = model.ChannelCapabilityStatusSupported
	} else if record.LegacyStatus == model.ChannelCapabilityStatusUnsupported && record.NativeStatus == model.ChannelCapabilityStatusUnsupported {
		record.Status = model.ChannelCapabilityStatusUnsupported
	} else {
		record.Status = model.ChannelCapabilityStatusUnknown
	}
}

// ObserveResponsesCapabilityAttempt records only protocol evidence. Transient
// availability failures remain the responsibility of the ordinary channel
// health pipeline and never become capability state.
func ObserveResponsesCapabilityAttempt(
	channel *model.Channel,
	modelName string,
	attempt ResponsesCapabilityAttempt,
	err *types.NewAPIError,
) ResponsesCapabilityOutcome {
	outcome := ResponsesCapabilityOutcome{Related: attempt.Related()}
	if channel == nil || !outcome.Related || strings.TrimSpace(modelName) == "" {
		return outcome
	}
	lock := responsesCapabilityObservationLock(channel.Id, modelName)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now()
	record, _ := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction)
	previousFingerprint := record.RouteFingerprint
	currentFingerprint := ResponsesObservedRouteFingerprint(channel, modelName)
	if !strings.EqualFold(previousFingerprint, currentFingerprint) {
		// Facets are route-scoped. Never carry evidence from an old base URL,
		// endpoint, mapping, or config version into the first observation of the
		// new route.
		record = model.ChannelModelCapability{}
	}
	record.ChannelId = channel.Id
	record.ModelName = modelName
	record.Capability = model.ChannelCapabilityResponsesCompaction
	record.RouteFingerprint = currentFingerprint
	record.Source = strings.TrimSpace(attempt.Source)
	if record.Source == "" {
		record.Source = "runtime"
	}

	if err == nil {
		// Avoid a SQLite write on every successful request while still refreshing
		// long-lived evidence at least four times per day.
		if record.VerifiedAt > 0 && now.Unix()-record.VerifiedAt < int64(6*time.Hour/time.Second) {
			before := record
			updateObservedCapabilityFacet(&record, attempt, model.ChannelCapabilityStatusSupported)
			if before.Status == record.Status && before.CapabilityValue == record.CapabilityValue &&
				before.NativeStreamStatus == record.NativeStreamStatus && before.ContinuationStatus == record.ContinuationStatus &&
				strings.EqualFold(previousFingerprint, record.RouteFingerprint) {
				return outcome
			}
		} else {
			updateObservedCapabilityFacet(&record, attempt, model.ChannelCapabilityStatusSupported)
		}
		record.VerifiedAt = now.Unix()
		record.NextProbeAt = now.Add(24 * time.Hour).Unix()
		record.BlockedUntil = 0
		record.FailureCount = 0
		record.LastStatusCode = 0
		record.LastError = ""
		outcome.PersistenceError = persistObservedResponsesCapability(record)
		return outcome
	}

	if !compactFailureExplicitlyUnsupported(attempt, err) {
		if strings.EqualFold(record.Source, "probe") {
			record.NextProbeAt = now.Add(6 * time.Hour).Unix()
			record.LastStatusCode = err.StatusCode
			record.LastError = common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())
			outcome.PersistenceError = persistObservedResponsesCapability(record)
		}
		return outcome
	}
	outcome.Unsupported = true
	updateObservedCapabilityFacet(&record, attempt, model.ChannelCapabilityStatusUnsupported)
	record.VerifiedAt = now.Unix()
	record.NextProbeAt = now.Add(24 * time.Hour).Unix()
	record.FailureCount++
	record.LastStatusCode = err.StatusCode
	record.LastError = common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())
	if record.Status == model.ChannelCapabilityStatusUnsupported {
		record.BlockedUntil = record.NextProbeAt
	} else {
		record.BlockedUntil = 0
	}
	outcome.PersistenceError = persistObservedResponsesCapability(record)
	return outcome
}

func persistObservedResponsesCapability(record model.ChannelModelCapability) error {
	if err := model.UpsertChannelModelCapability(record); err != nil {
		common.SysError("failed to persist responses capability observation: " + err.Error())
		return err
	}
	return nil
}

func observedCapabilityFacetDecision(channel *model.Channel, modelName string, requirement *ResponsesRoutingRequirement) (bool, bool) {
	if channel == nil || requirement == nil || requirement.Kind == dto.ResponsesNormal {
		return false, false
	}
	record, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction)
	if !found || !strings.EqualFold(record.RouteFingerprint, ResponsesObservedRouteFingerprint(channel, modelName)) {
		return false, false
	}
	if record.NextProbeAt > 0 && record.NextProbeAt <= common.GetTimestamp() {
		return false, false
	}
	decide := func(status int) (bool, bool) {
		switch status {
		case model.ChannelCapabilityStatusSupported:
			return true, true
		case model.ChannelCapabilityStatusUnsupported:
			return true, false
		default:
			return false, false
		}
	}
	switch requirement.Kind {
	case dto.ResponsesCompactEndpoint:
		return decide(record.LegacyStatus)
	case dto.ResponsesCompactedContextContinuation:
		return decide(record.ContinuationStatus)
	case dto.ResponsesCompactionTrigger:
		if record.LegacyStatus == model.ChannelCapabilityStatusSupported {
			return true, true
		}
		if record.NativeStatus == model.ChannelCapabilityStatusSupported {
			if !requirement.ClientStream {
				return true, true
			}
			return decide(record.NativeStreamStatus)
		}
		if record.LegacyStatus == model.ChannelCapabilityStatusUnsupported &&
			record.NativeStatus == model.ChannelCapabilityStatusUnsupported {
			return true, false
		}
	}
	return false, false
}
