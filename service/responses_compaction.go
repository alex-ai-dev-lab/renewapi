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

func channelSupportsRequiredContinuation(channel *model.Channel, compactionModel string, requirement *ResponsesRoutingRequirement, request dto.Request) bool {
	if requirement == nil || requirement.Kind != dto.ResponsesCompactionTrigger {
		return true
	}
	continuationModel := strings.TrimSpace(requirement.RequiredContinuationModel)
	if continuationModel == "" || strings.EqualFold(continuationModel, compactionModel) {
		return true
	}
	if !channelAdvertisesRoutingModel(channel, continuationModel) {
		return false
	}
	if responsesRouteCompatibilityForModel(channel, compactionModel) != responsesRouteCompatibilityForModel(channel, continuationModel) {
		return false
	}
	continuationRequirement := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactedContextContinuation}
	return ChannelMatchesResponsesRequirement(channel, continuationModel, continuationRequirement, request)
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
		return record, true
	}
	if settings.DefaultCapability != nil {
		return *settings.DefaultCapability, true
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

func effectiveCompactionCapability(channel *model.Channel, modelName string) dto.ResponsesCompactionCapabilityRecord {
	record, configured := responseCapabilityRecord(channel, modelName)
	if !configured || record.Capability == "" {
		record.Capability = dto.CompactionUnknown
	}
	if record.RouteFingerprint != "" && !strings.EqualFold(record.RouteFingerprint, ResponsesRouteFingerprint(channel, modelName)) {
		return dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionUnknown}
	}
	return record
}

func ChannelMatchesResponsesRequirement(channel *model.Channel, modelName string, requirement *ResponsesRoutingRequirement, request dto.Request) bool {
	if requirement == nil || requirement.Kind == dto.ResponsesNormal {
		return true
	}
	protocol := EvaluateChannelProtocolCapability(channel, modelName, types.RelayFormatOpenAIResponses, request)
	if !protocol.Supported || protocol.Lossy {
		return false
	}
	if !channelSupportsRequiredContinuation(channel, modelName, requirement, request) {
		return false
	}
	record := effectiveCompactionCapability(channel, modelName)
	if record.Capability == dto.CompactionDisabled {
		return false
	}
	if record.Capability == dto.CompactionUnknown {
		return !ResponsesCompactionEnforcementStrict()
	}
	switch requirement.Kind {
	case dto.ResponsesCompactionTrigger:
		if record.Capability == dto.CompactionLegacy {
			return true
		}
		if record.Capability != dto.CompactionNativeV2 && record.Capability != dto.CompactionNativeV2AndLegacy {
			return false
		}
		return !requirement.ClientStream || record.NativeStream
	case dto.ResponsesCompactEndpoint:
		return record.Capability == dto.CompactionLegacy || record.Capability == dto.CompactionNativeV2AndLegacy
	case dto.ResponsesCompactedContextContinuation:
		return (record.Capability == dto.CompactionNativeV2 || record.Capability == dto.CompactionNativeV2AndLegacy) && record.Continuation
	default:
		return true
	}
}

func PlanResponsesExecution(kind dto.ResponsesRequestKind, record dto.ResponsesCompactionCapabilityRecord, upstreamModel string, clientStream bool) (ResponsesExecutionPlan, error) {
	plan := ResponsesExecutionPlan{Kind: kind, UpstreamPath: "/v1/responses", UpstreamModel: strings.TrimSuffix(upstreamModel, "-openai-compact"), UpstreamStream: clientStream}
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
		if record.Capability == dto.CompactionLegacy {
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
	validItems := 0
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
