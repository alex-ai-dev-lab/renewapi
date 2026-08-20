package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	ContextKeyResponsesRequestKind       = "responses_request_kind"
	ContextKeyResponsesHasCompactedState = "responses_has_compacted_context"
	ContextKeyResponsesCompactedHashes   = "responses_compacted_content_hashes"
)

type ResponsesInputSignals struct {
	HasTrigger             bool
	HasCompactedContext    bool
	CompactedContentHashes []string
}

type ResponsesRoutingRequirement struct {
	Kind                      dto.ResponsesRequestKind
	ClientStream              bool
	RequiredContinuationModel string
}

// ResponsesRequirementReason is a stable, redacted reason used when a
// channel/model candidate is excluded from a Responses compaction route.
// These values are intentionally protocol/capability-only: they must never
// contain upstream response bodies, credentials, or URLs.
type ResponsesRequirementReason string

const (
	ResponsesRequirementReasonCapabilityUnknownStrict   ResponsesRequirementReason = "capability_unknown_strict"
	ResponsesRequirementReasonCapabilityDisabled        ResponsesRequirementReason = "capability_disabled"
	ResponsesRequirementReasonLegacyNotVerified         ResponsesRequirementReason = "legacy_not_verified"
	ResponsesRequirementReasonNativeNotVerified         ResponsesRequirementReason = "native_not_verified"
	ResponsesRequirementReasonNativeStreamNotVerified   ResponsesRequirementReason = "native_stream_not_verified"
	ResponsesRequirementReasonContinuationNotVerified   ResponsesRequirementReason = "continuation_not_verified"
	ResponsesRequirementReasonUnsupported               ResponsesRequirementReason = "unsupported"
	ResponsesRequirementReasonStaleRouteFingerprint     ResponsesRequirementReason = "stale_route_fingerprint"
	ResponsesRequirementReasonProtocolUnsupported       ResponsesRequirementReason = "protocol_unsupported"
	ResponsesRequirementReasonProtocolLossy             ResponsesRequirementReason = "protocol_lossy"
	ResponsesRequirementReasonContinuationRouteMismatch ResponsesRequirementReason = "continuation_route_incompatible"
	ResponsesRequirementReasonTemporarilyBlocked        ResponsesRequirementReason = "temporarily_blocked"
)

type ResponsesRequirementDecision struct {
	Allowed bool
	Reason  ResponsesRequirementReason
}

type ResponsesExecutionPlan struct {
	Kind              dto.ResponsesRequestKind
	UpstreamPath      string
	UpstreamModel     string
	UpstreamStream    bool
	BridgeJSONToSSE   bool
	RemoveTriggerItem bool
	StripTopLevel     []string
}

func InspectResponsesInput(body []byte) ResponsesInputSignals {
	var out ResponsesInputSignals
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return out
	}
	input.ForEach(func(_, item gjson.Result) bool {
		switch item.Get("type").String() {
		case "compaction_trigger":
			out.HasTrigger = true
		case "compaction", "context_compaction", "compaction_summary":
			out.HasCompactedContext = true
			if encrypted := item.Get("encrypted_content"); encrypted.Type == gjson.String && encrypted.String() != "" {
				sum := sha256.Sum256([]byte(encrypted.String()))
				out.CompactedContentHashes = append(out.CompactedContentHashes, hex.EncodeToString(sum[:]))
			}
		}
		return true
	})
	return out
}

func ClassifyResponsesRequest(path string, signals ResponsesInputSignals) dto.ResponsesRequestKind {
	if strings.TrimSuffix(path, "/") == "/v1/responses/compact" {
		return dto.ResponsesCompactEndpoint
	}
	if signals.HasTrigger {
		return dto.ResponsesCompactionTrigger
	}
	if signals.HasCompactedContext {
		return dto.ResponsesCompactedContextContinuation
	}
	return dto.ResponsesNormal
}

// RequiresResponsesCompactionCapability reports whether the wire-level
// Responses request needs a channel with verified compaction support.
//
// Ordinary Responses requests, including requests whose context was compacted
// locally by the client, remain ordinary requests as long as they do not carry
// the compaction endpoint or compaction item types. Keeping this predicate
// centralized prevents normal traffic from accidentally inheriting the
// compaction route constraint.
func RequiresResponsesCompactionCapability(kind dto.ResponsesRequestKind) bool {
	return kind != dto.ResponsesNormal
}

func ResponsesRequestKindName(kind dto.ResponsesRequestKind) string {
	switch kind {
	case dto.ResponsesNormal:
		return "normal"
	case dto.ResponsesCompactionTrigger:
		return "compaction_trigger"
	case dto.ResponsesCompactEndpoint:
		return "compact_endpoint"
	case dto.ResponsesCompactedContextContinuation:
		return "compacted_context_continuation"
	default:
		return fmt.Sprintf("unknown_%d", kind)
	}
}

func ResponsesCompactionEnforcementStrict() bool {
	return !strings.EqualFold(strings.TrimSpace(common.GetEnvOrDefaultString("RESPONSES_COMPACTION_ENFORCEMENT", "strict")), "observe")
}

// ResolveResponsesCompactionRoutingModel returns the model used to select the
// channel and build the upstream request. Only native compaction triggers are
// redirected; ordinary Responses requests, compacted-context continuations,
// and the legacy /v1/responses/compact endpoint keep their requested model.
func ResolveResponsesCompactionRoutingModel(kind dto.ResponsesRequestKind, requestedModel string) (string, bool) {
	if kind != dto.ResponsesCompactionTrigger || strings.TrimSpace(requestedModel) == "" {
		return requestedModel, false
	}
	targetModel := strings.TrimSpace(common.GetEnvOrDefaultString("RESPONSES_COMPACTION_MODEL", ""))
	if targetModel == "" {
		return requestedModel, false
	}
	targetModel = ResolveLatestModelAlias(targetModel)
	if strings.EqualFold(targetModel, requestedModel) {
		return requestedModel, false
	}
	return targetModel, true
}

func channelAdvertisesRoutingModel(channel *model.Channel, modelName string) bool {
	if channel == nil || strings.TrimSpace(modelName) == "" {
		return false
	}
	for _, candidate := range channel.GetRoutingModels() {
		if strings.EqualFold(candidate, modelName) {
			return true
		}
	}
	return false
}

type responsesRouteCompatibility struct {
	ChannelType int
	BaseURL     string
	Endpoint    string
}

func normalizeResponsesRouteBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if parsed, err := url.Parse(base); err == nil {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		base = parsed.String()
	}
	return base
}

func responsesRouteCompatibilityForModel(channel *model.Channel, modelName string) responsesRouteCompatibility {
	decision := model.ResolveModelRouteDecision(channel, modelName)
	return responsesRouteCompatibility{
		ChannelType: decision.ChannelType,
		BaseURL:     normalizeResponsesRouteBaseURL(decision.BaseURL),
		Endpoint:    string(decision.Endpoint),
	}
}

func requiredContinuationDecision(channel *model.Channel, compactionModel string, requirement *ResponsesRoutingRequirement, request dto.Request) ResponsesRequirementDecision {
	if requirement == nil || requirement.Kind != dto.ResponsesCompactionTrigger {
		return ResponsesRequirementDecision{Allowed: true}
	}
	continuationModel := strings.TrimSpace(requirement.RequiredContinuationModel)
	if continuationModel == "" {
		return ResponsesRequirementDecision{Allowed: true}
	}
	if !strings.EqualFold(continuationModel, compactionModel) {
		if !channelAdvertisesRoutingModel(channel, continuationModel) {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonContinuationNotVerified}
		}
		if responsesRouteCompatibilityForModel(channel, compactionModel) != responsesRouteCompatibilityForModel(channel, continuationModel) {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonContinuationRouteMismatch}
		}
	}
	continuationRequirement := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactedContextContinuation}
	return responsesRequirementDecision(channel, continuationModel, continuationRequirement, request)
}

func isConcreteCompactionCapability(capability dto.CompactionCapability) bool {
	capability = dto.CompactionCapability(strings.ToLower(strings.TrimSpace(string(capability))))
	return capability != "" && capability != dto.CompactionUnknown
}

func responseCapabilityRecord(channel *model.Channel, modelName string) (dto.ResponsesCompactionCapabilityRecord, bool) {
	if channel == nil {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	settings := channel.GetSetting().ResponsesCompaction
	if settings == nil {
		return dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown}, false
	}
	clientModel := strings.TrimSuffix(modelName, "-openai-compact")
	if record, ok := settings.ModelCapabilities[clientModel]; ok {
		if record.Capability == "" {
			record.Capability = dto.CompactionUnknown
		}
		// An explicit unknown is a request to learn, not an authoritative
		// capability declaration. Concrete operator declarations still win.
		return record, isConcreteCompactionCapability(record.Capability)
	}
	if settings.DefaultCapability != nil {
		record := *settings.DefaultCapability
		if record.Capability == "" {
			record.Capability = dto.CompactionUnknown
		}
		return record, isConcreteCompactionCapability(record.Capability)
	}
	return dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown}, false
}

func ResponsesRouteFingerprint(channel *model.Channel, modelName string) string {
	if channel == nil {
		return ""
	}
	clientModel := strings.TrimSuffix(modelName, "-openai-compact")
	decision := model.ResolveModelRouteDecision(channel, clientModel)
	base := normalizeResponsesRouteBaseURL(decision.BaseURL)
	raw := fmt.Sprintf("%d\n%s\n%s\n%s", decision.ChannelType, base, decision.Endpoint, clientModel)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ResponsesObservedRouteFingerprint invalidates runtime/probe evidence when
// any channel configuration changes. Manual fingerprints keep the narrower
// historical route fingerprint semantics for backward compatibility.
func ResponsesObservedRouteFingerprint(channel *model.Channel, modelName string) string {
	if channel == nil {
		return ""
	}
	raw := fmt.Sprintf("%s\n%d\n%s", ResponsesRouteFingerprint(channel, modelName), channel.ConfigVersion, channel.GetModelMapping())
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func observedCompactionCapability(channel *model.Channel, modelName string) (dto.ResponsesCompactionCapabilityRecord, bool) {
	if channel == nil {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	observed, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction)
	if !found || observed.RouteFingerprint == "" ||
		!strings.EqualFold(observed.RouteFingerprint, ResponsesObservedRouteFingerprint(channel, modelName)) {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	if observed.BlockedUntil > 0 && observed.BlockedUntil <= common.GetTimestamp() {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	legacySupported := observed.LegacyStatus == model.ChannelCapabilityStatusSupported
	nativeSupported := observed.NativeStatus == model.ChannelCapabilityStatusSupported
	value := dto.CompactionUnknown
	switch {
	case legacySupported && nativeSupported:
		value = dto.CompactionNativeV2AndLegacy
	case legacySupported:
		value = dto.CompactionLegacy
	case nativeSupported:
		value = dto.CompactionNativeV2
	case observed.LegacyStatus == model.ChannelCapabilityStatusUnsupported &&
		observed.NativeStatus == model.ChannelCapabilityStatusUnsupported:
		value = dto.CompactionDisabled
	case observed.CapabilityValue != "":
		// Backward-compatible fallback for rows written before facet statuses
		// were introduced.
		value = dto.CompactionCapability(observed.CapabilityValue)
	}
	if value == dto.CompactionUnknown && observed.Status != model.ChannelCapabilityStatusSupported {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	record := dto.ResponsesCompactionCapabilityRecord{
		Capability:       value,
		NativeStream:     observed.NativeStreamStatus == model.ChannelCapabilityStatusSupported || observed.NativeStream,
		Continuation:     observed.ContinuationStatus == model.ChannelCapabilityStatusSupported || observed.Continuation,
		RouteFingerprint: observed.RouteFingerprint,
		VerifiedAt:       observed.VerifiedAt,
	}
	if record.Capability == "" {
		record.Capability = dto.CompactionUnknown
	}
	return record, true
}

func mergeMeasuredCompactionCapability(
	channel *model.Channel,
	modelName string,
	measured dto.ResponsesCompactionCapabilityRecord,
) (dto.ResponsesCompactionCapabilityRecord, bool) {
	if channel == nil || measured.VerifiedAt <= 0 {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}
	observed, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction)
	if !found || observed.VerifiedAt < measured.VerifiedAt || observed.RouteFingerprint == "" ||
		!strings.EqualFold(observed.RouteFingerprint, ResponsesObservedRouteFingerprint(channel, modelName)) {
		return dto.ResponsesCompactionCapabilityRecord{}, false
	}

	legacy := measured.Capability == dto.CompactionLegacy || measured.Capability == dto.CompactionNativeV2AndLegacy
	native := measured.Capability == dto.CompactionNativeV2 || measured.Capability == dto.CompactionNativeV2AndLegacy
	applyFacet := func(status int, value *bool) {
		switch status {
		case model.ChannelCapabilityStatusSupported:
			*value = true
		case model.ChannelCapabilityStatusUnsupported:
			*value = false
		}
	}
	applyFacet(observed.LegacyStatus, &legacy)
	applyFacet(observed.NativeStatus, &native)
	applyFacet(observed.NativeStreamStatus, &measured.NativeStream)
	applyFacet(observed.ContinuationStatus, &measured.Continuation)

	switch {
	case legacy && native:
		measured.Capability = dto.CompactionNativeV2AndLegacy
	case legacy:
		measured.Capability = dto.CompactionLegacy
	case native:
		measured.Capability = dto.CompactionNativeV2
	default:
		measured.Capability = dto.CompactionDisabled
	}
	if native {
		measured.PreferredTransport = "native_v2"
	} else if legacy {
		measured.PreferredTransport = "legacy"
	} else {
		measured.PreferredTransport = ""
	}
	measured.RouteFingerprint = observed.RouteFingerprint
	measured.VerifiedAt = observed.VerifiedAt
	return measured, true
}

func effectiveCompactionCapability(channel *model.Channel, modelName string) dto.ResponsesCompactionCapabilityRecord {
	record, configured := responseCapabilityRecord(channel, modelName)
	if configured {
		if record.RouteFingerprint != "" && !strings.EqualFold(record.RouteFingerprint, ResponsesRouteFingerprint(channel, modelName)) {
			// A stale manual assertion is no longer authoritative. Fall through to
			// route-scoped observed evidence before returning unknown.
			configured = false
		} else {
			if merged, ok := mergeMeasuredCompactionCapability(channel, modelName, record); ok {
				return merged
			}
			return record
		}
	}
	if observed, found := observedCompactionCapability(channel, modelName); found {
		return observed
	}
	return dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown}
}

func responsesRequirementDecision(channel *model.Channel, modelName string, requirement *ResponsesRoutingRequirement, request dto.Request) ResponsesRequirementDecision {
	if requirement == nil || !RequiresResponsesCompactionCapability(requirement.Kind) {
		return ResponsesRequirementDecision{Allowed: true}
	}
	protocol := EvaluateChannelProtocolCapability(channel, modelName, types.RelayFormatOpenAIResponses, request)
	if !protocol.Supported {
		return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonProtocolUnsupported}
	}
	if protocol.Lossy {
		return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonProtocolLossy}
	}
	_, manuallyConfigured := responseCapabilityRecord(channel, modelName)
	if !manuallyConfigured {
		if observed, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction); found {
			currentFingerprint := ResponsesObservedRouteFingerprint(channel, modelName)
			if observed.RouteFingerprint != "" && !strings.EqualFold(observed.RouteFingerprint, currentFingerprint) {
				return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonStaleRouteFingerprint}
			}
		}
		if decided, allowed := observedCapabilityFacetDecision(channel, modelName, requirement); decided {
			if allowed {
				return requiredContinuationDecision(channel, modelName, requirement, request)
			}
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonUnsupported}
		}
		if observed, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction); found && observed.BlockedUntil > common.GetTimestamp() {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonTemporarilyBlocked}
		}
	}
	record := effectiveCompactionCapability(channel, modelName)
	if record.Capability == dto.CompactionDisabled {
		return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonCapabilityDisabled}
	}
	if record.Capability == dto.CompactionUnknown {
		return ResponsesRequirementDecision{Allowed: !ResponsesCompactionEnforcementStrict(), Reason: ResponsesRequirementReasonCapabilityUnknownStrict}
	}
	switch requirement.Kind {
	case dto.ResponsesCompactionTrigger:
		if record.Capability == dto.CompactionLegacy {
			return requiredContinuationDecision(channel, modelName, requirement, request)
		}
		if record.Capability != dto.CompactionNativeV2 && record.Capability != dto.CompactionNativeV2AndLegacy {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonNativeNotVerified}
		}
		if requirement.ClientStream && !record.NativeStream && record.Capability != dto.CompactionNativeV2AndLegacy {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonNativeStreamNotVerified}
		}
		return requiredContinuationDecision(channel, modelName, requirement, request)
	case dto.ResponsesCompactEndpoint:
		if record.Capability == dto.CompactionLegacy || record.Capability == dto.CompactionNativeV2AndLegacy {
			return ResponsesRequirementDecision{Allowed: true}
		}
		return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonLegacyNotVerified}
	case dto.ResponsesCompactedContextContinuation:
		if record.Capability == dto.CompactionUnknown {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonContinuationNotVerified}
		}
		if record.Capability == dto.CompactionDisabled {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonUnsupported}
		}
		if !record.Continuation {
			return ResponsesRequirementDecision{Reason: ResponsesRequirementReasonContinuationNotVerified}
		}
		return ResponsesRequirementDecision{Allowed: true}
	default:
		return ResponsesRequirementDecision{Allowed: true}
	}
}

func ChannelMatchesResponsesRequirement(channel *model.Channel, modelName string, requirement *ResponsesRoutingRequirement, request dto.Request) bool {
	return responsesRequirementDecision(channel, modelName, requirement, request).Allowed
}

// ResponsesRequirementDecisionForChannel exposes the same strict routing
// decision as ChannelMatchesResponsesRequirement together with a stable,
// redacted reason suitable for distributor diagnostics.
func ResponsesRequirementDecisionForChannel(channel *model.Channel, modelName string, requirement *ResponsesRoutingRequirement, request dto.Request) ResponsesRequirementDecision {
	return responsesRequirementDecision(channel, modelName, requirement, request)
}

func PlanResponsesExecution(kind dto.ResponsesRequestKind, record dto.ResponsesCompactionCapabilityRecord, upstreamModel string, clientStream bool) (ResponsesExecutionPlan, error) {
	plan := ResponsesExecutionPlan{Kind: kind, UpstreamPath: "/v1/responses", UpstreamModel: strings.TrimSuffix(upstreamModel, "-openai-compact"), UpstreamStream: clientStream}
	if record.Capability == dto.CompactionUnknown && !ResponsesCompactionEnforcementStrict() {
		// Observe mode performs a conservative real request so capability can be
		// learned. The legacy endpoint remains legacy; trigger requests default to
		// the native Responses transport.
		if kind == dto.ResponsesCompactEndpoint {
			record.Capability = dto.CompactionLegacy
		} else {
			record.Capability = dto.CompactionNativeV2
			record.NativeStream = clientStream
		}
	}
	switch kind {
	case dto.ResponsesNormal, dto.ResponsesCompactedContextContinuation:
		return plan, nil
	case dto.ResponsesCompactEndpoint:
		if record.Capability != dto.CompactionLegacy && record.Capability != dto.CompactionNativeV2AndLegacy {
			return plan, fmt.Errorf("channel does not support legacy responses compaction")
		}
		plan.UpstreamPath = "/v1/responses/compact"
		plan.UpstreamStream = false
		plan.StripTopLevel = []string{"stream", "stream_options"}
		return plan, nil
	case dto.ResponsesCompactionTrigger:
		useLegacy := record.Capability == dto.CompactionLegacy ||
			(record.Capability == dto.CompactionNativeV2AndLegacy && clientStream && !record.NativeStream)
		if useLegacy {
			plan.UpstreamPath = "/v1/responses/compact"
			plan.UpstreamStream = false
			plan.BridgeJSONToSSE = clientStream
			plan.RemoveTriggerItem = true
			plan.StripTopLevel = []string{"stream", "stream_options", "prompt_cache_retention", "prompt_cache_options"}
			return plan, nil
		}
		if record.Capability != dto.CompactionNativeV2 && record.Capability != dto.CompactionNativeV2AndLegacy {
			return plan, fmt.Errorf("channel does not support native responses compaction")
		}
		if clientStream && !record.NativeStream {
			return plan, fmt.Errorf("channel native compaction stream is not verified")
		}
		return plan, nil
	default:
		return plan, fmt.Errorf("unknown responses request kind %d", kind)
	}
}

func EffectiveResponsesCompactionRecord(channel *model.Channel, modelName string) dto.ResponsesCompactionCapabilityRecord {
	return effectiveCompactionCapability(channel, modelName)
}

func EffectiveResponsesCompactionRecordByID(channelID int, modelName string, fallbackSettings dto.ChannelSettings) dto.ResponsesCompactionCapabilityRecord {
	if channel, err := model.CacheGetChannel(channelID); err == nil && channel != nil {
		return effectiveCompactionCapability(channel, modelName)
	}
	return ResponsesCompactionRecordFromSettings(fallbackSettings, modelName)
}

// ResponsesCompactionRecordFromSettings is retained for callers that do not
// have a channel identity. Production routing should use the effective
// resolver so route fingerprints and observed capabilities are honored.
func ResponsesCompactionRecordFromSettings(settings dto.ChannelSettings, modelName string) dto.ResponsesCompactionCapabilityRecord {
	result := dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown}
	if settings.ResponsesCompaction == nil {
		return result
	}
	clientModel := strings.TrimSuffix(modelName, "-openai-compact")
	if record, ok := settings.ResponsesCompaction.ModelCapabilities[clientModel]; ok {
		if record.Capability == "" {
			record.Capability = dto.CompactionUnknown
		}
		return record
	}
	if settings.ResponsesCompaction.DefaultCapability != nil {
		result = *settings.ResponsesCompaction.DefaultCapability
		if result.Capability == "" {
			result.Capability = dto.CompactionUnknown
		}
	}
	return result
}

func BuildResponsesCompactionRequestBody(body []byte, plan ResponsesExecutionPlan) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("invalid responses JSON body")
	}
	result := append([]byte(nil), body...)
	var err error
	if plan.UpstreamModel != "" {
		result, err = sjson.SetBytes(result, "model", plan.UpstreamModel)
		if err != nil {
			return nil, err
		}
	}
	if plan.UpstreamStream {
		result, err = sjson.SetBytes(result, "stream", true)
		if err != nil {
			return nil, err
		}
	}
	for _, field := range plan.StripTopLevel {
		result, err = sjson.DeleteBytes(result, field)
		if err != nil {
			return nil, err
		}
	}
	if !plan.RemoveTriggerItem {
		return result, nil
	}
	input := gjson.GetBytes(result, "input")
	if !input.IsArray() {
		return nil, fmt.Errorf("legacy compaction downgrade requires top-level input array")
	}
	var raw bytes.Buffer
	raw.WriteByte('[')
	kept := 0
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "compaction_trigger" {
			return true
		}
		if kept > 0 {
			raw.WriteByte(',')
		}
		raw.WriteString(item.Raw)
		kept++
		return true
	})
	raw.WriteByte(']')
	return sjson.SetRawBytes(result, "input", raw.Bytes())
}

func ValidateCompactionResponse(body []byte) error {
	maxBodyBytes := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_BODY_BYTES", 16*1024*1024)
	if maxBodyBytes > 0 && len(body) > maxBodyBytes {
		return fmt.Errorf("compaction response body size limit exceeded")
	}
	if !gjson.ValidBytes(body) {
		return fmt.Errorf("invalid compaction response JSON")
	}
	if object := gjson.GetBytes(body, "object").String(); object != "response.compaction" {
		return fmt.Errorf("compaction capability mismatch: object=%q", object)
	}
	usage := gjson.GetBytes(body, "usage")
	if !usage.IsObject() {
		return fmt.Errorf("compaction response missing usage")
	}
	for _, field := range []string{"input_tokens", "output_tokens", "total_tokens"} {
		value := usage.Get(field)
		if !value.Exists() || value.Type != gjson.Number || value.Int() < 0 {
			return fmt.Errorf("compaction response has invalid usage.%s", field)
		}
	}
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return fmt.Errorf("compaction response output is not an array")
	}
	maxItems := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_ITEMS", 64)
	maxEncrypted := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_ENCRYPTED_BYTES", 8*1024*1024)
	maxTotalEncrypted := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_TOTAL_ENCRYPTED_BYTES", 12*1024*1024)
	validItems := 0
	totalEncryptedBytes := 0
	items := output.Array()
	if len(items) > maxItems {
		return fmt.Errorf("compaction response output item limit exceeded")
	}
	for _, item := range items {
		typ := item.Get("type").String()
		encrypted := item.Get("encrypted_content")
		if typ != "compaction" && typ != "context_compaction" && typ != "compaction_summary" {
			if encrypted.Exists() {
				return fmt.Errorf("encrypted_content on invalid output item type %q", typ)
			}
			continue
		}
		if encrypted.Type != gjson.String || encrypted.String() == "" {
			return fmt.Errorf("compaction output item has empty or non-string encrypted_content")
		}
		if len(encrypted.String()) > maxEncrypted {
			return fmt.Errorf("compaction encrypted_content size limit exceeded")
		}
		totalEncryptedBytes += len(encrypted.String())
		if maxTotalEncrypted > 0 && totalEncryptedBytes > maxTotalEncrypted {
			return fmt.Errorf("compaction total encrypted_content size limit exceeded")
		}
		validItems++
	}
	if validItems == 0 {
		return fmt.Errorf("compaction response missing compaction output item")
	}
	return nil
}

func CompactionResponseContentHashes(body []byte) []string {
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return nil
	}
	hashes := make([]string, 0, len(output.Array()))
	output.ForEach(func(_, item gjson.Result) bool {
		typ := item.Get("type").String()
		if typ != "compaction" && typ != "context_compaction" && typ != "compaction_summary" {
			return true
		}
		if encrypted := item.Get("encrypted_content"); encrypted.Type == gjson.String && encrypted.String() != "" {
			sum := sha256.Sum256([]byte(encrypted.String()))
			hashes = append(hashes, hex.EncodeToString(sum[:]))
		}
		return true
	})
	return hashes
}
