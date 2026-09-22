package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestResponsesUsageIncludesAllGeneratedDeltas(t *testing.T) {
	for _, kind := range []string{"output_text", "function_call_arguments", "reasoning_summary_text", "reasoning_text", "refusal"} {
		t.Run(kind, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
			info.SetEstimatePromptTokens(37)
			a := NewResponsesUsageAccumulator(info)
			a.Observe(`{"type":"response.created"}`)
			a.Observe(`{"type":"response.` + kind + `.delta","delta":"hello world"}`)
			usage := a.Finish()
			require.Equal(t, 37, usage.PromptTokens)
			require.Equal(t, CountTextToken("hello world", "gpt-4o"), usage.CompletionTokens)
			require.Same(t, usage, a.Finish())
			require.Same(t, usage, info.ResponsesObservedUsage)
		})
	}
}

func TestResponsesUsagePreambleAndExplicitZero(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
	info.SetEstimatePromptTokens(37)
	a := NewResponsesUsageAccumulator(info)
	a.Observe(`{"type":"response.created"}`)
	require.Zero(t, a.Finish().TotalTokens)
	a = NewResponsesUsageAccumulator(info)
	a.Observe(`{"type":"response.output_text.delta","delta":"hello"}`)
	a.Observe(`{"type":"response.completed","response":{"usage":{"input_tokens":0,"output_tokens":0}}}`)
	require.Zero(t, a.Finish().TotalTokens)
}

func TestResponsesUsageTerminalOnlyAndNativeDetails(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
	a := NewResponsesUsageAccumulator(info)
	a.Observe(`{"type":"response.completed","response":{"output":[{"type":"function_call","arguments":"{\"city\":\"Taipei\"}"}],"usage":{"input_tokens":25,"input_tokens_details":{"cached_tokens":10,"cache_write_tokens":5},"output_tokens_details":{"reasoning_tokens":3}}}}`)
	usage := a.Finish()
	require.Positive(t, usage.CompletionTokens)
	require.Equal(t, 25, usage.PromptTokens)
	require.Equal(t, 10, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 5, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, 3, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestResponsesUsageTerminalCompletesPartialDeltasWithoutDuplication(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
	for _, delta := range []string{"hello", "hello world with more complete content"} {
		a := NewResponsesUsageAccumulator(info)
		a.Observe(`{"type":"response.output_text.delta","delta":"` + delta + `"}`)
		a.Observe(`{"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"hello world with more complete content"}]}]}}`)
		require.Equal(t, CountTextToken("hello world with more complete content", "gpt-4o"), a.Finish().CompletionTokens)
	}
}

func TestResponsesUsageNullCountersAreEstimated(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
	info.SetEstimatePromptTokens(17)
	a := NewResponsesUsageAccumulator(info)
	a.Observe(`{"type":"response.output_text.delta","delta":"hello world"}`)
	a.Observe(`{"type":"response.completed","response":{"usage":{"input_tokens":null,"output_tokens":null}}}`)
	require.Equal(t, 17, a.Finish().PromptTokens)
	require.Equal(t, CountTextToken("hello world", "gpt-4o"), a.Finish().CompletionTokens)
}
