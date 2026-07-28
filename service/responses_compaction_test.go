package service

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChannelMatchesResponsesRequirement(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	trigger := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger}
	streamTrigger := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactionTrigger, ClientStream: true}
	continuation := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactedContextContinuation}
	legacyEndpoint := &ResponsesRoutingRequirement{Kind: dto.ResponsesCompactEndpoint}

	channel := func(channelType int, setting string) *model.Channel {
		return &model.Channel{Type: channelType, Setting: common.GetPointer(setting)}
	}
	require.False(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{}`), "gpt-5.4", trigger, nil))
	require.False(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"disabled"}}}`), "gpt-5.4", trigger, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2"}}}`), "gpt-5.4", trigger, nil))
	require.False(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2"}}}`), "gpt-5.4", streamTrigger, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2","native_stream":true,"continuation":true}}}`), "gpt-5.4", streamTrigger, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2_and_legacy"}}}`), "gpt-5.4", streamTrigger, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2","continuation":true}}}`), "gpt-5.4", continuation, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"legacy"}}}`), "gpt-5.4-openai-compact", legacyEndpoint, nil))
	require.False(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeAnthropic, `{"responses_compaction":{"default_capability":{"capability":"native_v2_and_legacy","native_stream":true,"continuation":true}}}`), "gpt-5.4", trigger, nil))

	routedTrigger := &ResponsesRoutingRequirement{
		Kind:                      dto.ResponsesCompactionTrigger,
		RequiredContinuationModel: "gpt-5.5",
	}
	routedChannel := channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2"}}}`)
	routedChannel.Models = "gpt-5.6-sol"
	require.False(t, ChannelMatchesResponsesRequirement(routedChannel, "gpt-5.6-sol", routedTrigger, nil))
	routedChannel.Models = "gpt-5.5,gpt-5.6-sol"
	require.False(t, ChannelMatchesResponsesRequirement(routedChannel, "gpt-5.6-sol", routedTrigger, nil))
	routedChannel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		DefaultCapability: &dto.ResponsesCompactionCapabilityRecord{
			Capability:   dto.CompactionNativeV2,
			Continuation: true,
		},
	}})
	require.True(t, ChannelMatchesResponsesRequirement(routedChannel, "gpt-5.6-sol", routedTrigger, nil))
}

func TestResponsesRouteCompatibilityForCompactionRouting(t *testing.T) {
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: common.GetPointer("HTTPS://API.EXAMPLE.COM/v1/"),
	}

	left := responsesRouteCompatibilityForModel(channel, "gpt-5.6-sol")
	right := responsesRouteCompatibilityForModel(channel, "gpt-5.5")
	require.Equal(t, left, right)
	require.Equal(t, "https://api.example.com/v1", left.BaseURL)
}

func TestRoutedCompactionRejectsDifferentEffectiveRoutes(t *testing.T) {
	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [
			{"match_type":"exact","pattern":"gpt-5.6-sol","channel_type":1,"default_endpoint":"openai-response"},
			{"match_type":"exact","pattern":"gpt-5.5","channel_type":14,"default_endpoint":"anthropic"}
		]
	}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})

	channel := &model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Models: "gpt-5.5,gpt-5.6-sol",
		Setting: common.GetPointer(`{
			"allow_model_protocol_override":true,
			"model_protocol_override_targets":["anthropic","openai-response"],
			"responses_compaction":{"default_capability":{"capability":"native_v2","continuation":true}}
		}`),
	}
	require.NotEqual(
		t,
		responsesRouteCompatibilityForModel(channel, "gpt-5.6-sol"),
		responsesRouteCompatibilityForModel(channel, "gpt-5.5"),
	)
	require.False(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.6-sol", &ResponsesRoutingRequirement{
		Kind:                      dto.ResponsesCompactionTrigger,
		RequiredContinuationModel: "gpt-5.5",
	}, nil))
}

func TestRoutedCompactionRequiresClientModelContinuationCapability(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := &model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Models: "gpt-5.5,gpt-5.6-sol",
	}
	requirement := &ResponsesRoutingRequirement{
		Kind:                      dto.ResponsesCompactionTrigger,
		RequiredContinuationModel: "gpt-5.5",
	}

	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6-sol": {Capability: dto.CompactionNativeV2},
		},
	}})
	require.False(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.6-sol", requirement, nil))

	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.6-sol": {Capability: dto.CompactionNativeV2},
			"gpt-5.5":     {Capability: dto.CompactionNativeV2, Continuation: true},
		},
	}})
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.6-sol", requirement, nil))
}

func TestRoutedCompactionRequiresContinuationWhenCandidateMatchesClientModel(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_ENFORCEMENT", "strict")
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Models: "gpt-5.5"}
	requirement := &ResponsesRoutingRequirement{
		Kind:                      dto.ResponsesCompactionTrigger,
		RequiredContinuationModel: "gpt-5.5",
	}
	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": {Capability: dto.CompactionNativeV2},
		},
	}})
	require.False(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", requirement, nil))

	channel.SetSetting(dto.ChannelSettings{ResponsesCompaction: &dto.ResponsesCompactionSettings{
		ModelCapabilities: map[string]dto.ResponsesCompactionCapabilityRecord{
			"gpt-5.5": {Capability: dto.CompactionNativeV2, Continuation: true},
		},
	}})
	require.True(t, ChannelMatchesResponsesRequirement(channel, "gpt-5.5", requirement, nil))
}

func TestResolveResponsesCompactionRoutingModel(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_MODEL", "gpt-5.6-sol")

	modelName, routed := ResolveResponsesCompactionRoutingModel(dto.ResponsesCompactionTrigger, "gpt-5.5")
	require.True(t, routed)
	require.Equal(t, "gpt-5.6-sol", modelName)

	for _, kind := range []dto.ResponsesRequestKind{
		dto.ResponsesNormal,
		dto.ResponsesCompactedContextContinuation,
		dto.ResponsesCompactEndpoint,
	} {
		modelName, routed = ResolveResponsesCompactionRoutingModel(kind, "gpt-5.5")
		require.False(t, routed)
		require.Equal(t, "gpt-5.5", modelName)
	}
}

func TestResolveResponsesCompactionRoutingModelDisabledByDefault(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_MODEL", "")
	modelName, routed := ResolveResponsesCompactionRoutingModel(dto.ResponsesCompactionTrigger, "gpt-5.5")
	require.False(t, routed)
	require.Equal(t, "gpt-5.5", modelName)
}

func TestResolveResponsesCompactionRoutingModelDoesNotFillMissingClientModel(t *testing.T) {
	t.Setenv("RESPONSES_COMPACTION_MODEL", "gpt-5.6-sol")
	modelName, routed := ResolveResponsesCompactionRoutingModel(dto.ResponsesCompactionTrigger, "")
	require.False(t, routed)
	require.Empty(t, modelName)
}

func TestInspectAndClassifyResponsesInput(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want dto.ResponsesRequestKind
	}{
		{"trigger", "/v1/responses", `{"input":[{"type":"compaction_trigger"}]}`, dto.ResponsesCompactionTrigger},
		{"compaction", "/v1/responses", `{"input":[{"type":"compaction","encrypted_content":"x"}]}`, dto.ResponsesCompactedContextContinuation},
		{"context compaction", "/v1/responses", `{"input":[{"type":"context_compaction","encrypted_content":"x"}]}`, dto.ResponsesCompactedContextContinuation},
		{"summary", "/v1/responses", `{"input":[{"type":"compaction_summary","encrypted_content":"x"}]}`, dto.ResponsesCompactedContextContinuation},
		{"nested tool schema ignored", "/v1/responses", `{"input":[{"type":"message","content":[{"type":"input_text","text":"x"}]}],"tools":[{"schema":{"type":"compaction_trigger"}}]}`, dto.ResponsesNormal},
		{"compact endpoint wins", "/v1/responses/compact", `{"input":[{"type":"compaction_trigger"}]}`, dto.ResponsesCompactEndpoint},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signals := InspectResponsesInput([]byte(test.body))
			require.Equal(t, test.want, ClassifyResponsesRequest(test.path, signals))
		})
	}
}

func TestPlanResponsesExecution(t *testing.T) {
	legacy, err := PlanResponsesExecution(dto.ResponsesCompactionTrigger, dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionLegacy}, "gpt-5.4", true)
	require.NoError(t, err)
	require.Equal(t, "/v1/responses/compact", legacy.UpstreamPath)
	require.False(t, legacy.UpstreamStream)
	require.True(t, legacy.BridgeJSONToSSE)
	require.True(t, legacy.RemoveTriggerItem)

	native, err := PlanResponsesExecution(dto.ResponsesCompactionTrigger, dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2, NativeStream: true}, "gpt-5.4", true)
	require.NoError(t, err)
	require.Equal(t, "/v1/responses", native.UpstreamPath)
	require.True(t, native.UpstreamStream)
	require.False(t, native.RemoveTriggerItem)

	combinedLegacyStream, err := PlanResponsesExecution(dto.ResponsesCompactionTrigger, dto.ResponsesCompactionCapabilityRecord{
		Capability: dto.CompactionNativeV2AndLegacy,
	}, "gpt-5.4", true)
	require.NoError(t, err)
	require.Equal(t, "/v1/responses/compact", combinedLegacyStream.UpstreamPath)
	require.True(t, combinedLegacyStream.BridgeJSONToSSE)
	require.True(t, combinedLegacyStream.RemoveTriggerItem)

	combinedNative, err := PlanResponsesExecution(dto.ResponsesCompactionTrigger, dto.ResponsesCompactionCapabilityRecord{
		Capability:   dto.CompactionNativeV2AndLegacy,
		NativeStream: true,
	}, "gpt-5.4", true)
	require.NoError(t, err)
	require.Equal(t, "/v1/responses", combinedNative.UpstreamPath)
	require.False(t, combinedNative.BridgeJSONToSSE)

	_, err = PlanResponsesExecution(dto.ResponsesCompactionTrigger, dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}, "gpt-5.4", true)
	require.Error(t, err)
	_, err = PlanResponsesExecution(dto.ResponsesCompactEndpoint, dto.ResponsesCompactionCapabilityRecord{Capability: dto.CompactionNativeV2}, "gpt-5.4-openai-compact", false)
	require.Error(t, err)
}

func TestBuildResponsesCompactionRequestBodyPreservesRawItems(t *testing.T) {
	originalItem := `{"type":"compaction_summary","encrypted_content":"opaque","future":{"a":[1,2,3]}}`
	body := []byte(`{"model":"gpt-5.4-openai-compact","stream":true,"stream_options":{"x":1},"input":[{"type":"compaction_trigger"},` + originalItem + `]}`)
	plan := ResponsesExecutionPlan{UpstreamModel: "gpt-5.4", RemoveTriggerItem: true, StripTopLevel: []string{"stream", "stream_options"}}
	got, err := BuildResponsesCompactionRequestBody(body, plan)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(got, "model").String())
	require.False(t, gjson.GetBytes(got, "stream").Exists())
	require.Equal(t, originalItem, gjson.GetBytes(got, "input.0").Raw)
	require.Len(t, gjson.GetBytes(got, "input").Array(), 1)
}

func TestValidateCompactionResponse(t *testing.T) {
	valid := []byte(`{"id":"resp_1","object":"response.compaction","output":[{"type":"compaction_summary","encrypted_content":"opaque","future":true}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`)
	require.NoError(t, ValidateCompactionResponse(valid))
	require.NotEmpty(t, CompactionResponseContentHashes(valid))

	invalid := [][]byte{
		[]byte(`{"object":"response","output":[],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`),
		[]byte(`{"object":"response.compaction","output":[{"type":"compaction_summary","encrypted_content":""}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`),
		[]byte(`{"object":"response.compaction","output":[{"type":"compaction_summary","encrypted_content":42}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`),
		[]byte(`{"object":"response.compaction","output":[{"type":"message","encrypted_content":"opaque"}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`),
		[]byte(`{"object":"response.compaction","output":[{"type":"compaction_summary","encrypted_content":"opaque"}]}`),
	}
	for _, body := range invalid {
		require.Error(t, ValidateCompactionResponse(body), string(bytes.Clone(body)))
	}
}
