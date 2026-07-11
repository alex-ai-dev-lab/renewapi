package service

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
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
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"native_v2","continuation":true}}}`), "gpt-5.4", continuation, nil))
	require.True(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeOpenAI, `{"responses_compaction":{"default_capability":{"capability":"legacy"}}}`), "gpt-5.4-openai-compact", legacyEndpoint, nil))
	require.False(t, ChannelMatchesResponsesRequirement(channel(constant.ChannelTypeAnthropic, `{"responses_compaction":{"default_capability":{"capability":"native_v2_and_legacy","native_stream":true,"continuation":true}}}`), "gpt-5.4", trigger, nil))
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
