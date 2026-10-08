package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestDirectClaudeResponsesPreservesMixedContentAndExplicitValues(t *testing.T) {
	var req dto.ClaudeRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"gpt-test","max_tokens":0,"temperature":0,"thinking":{"type":"disabled"},"output_config":{"effort":"high"},"messages":[{"role":"assistant","content":[{"type":"text","text":"before"},{"type":"thinking","thinking":"visible summary"},{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"x"}},{"type":"text","text":"after"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"result"}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"},"strict":false}],"tool_choice":{"type":"auto","disable_parallel_tool_use":true}}`, &req))
	converted, err := ClaudeRequestToResponsesRequest(&req)
	require.NoError(t, err)
	require.NotNil(t, converted.MaxOutputTokens)
	require.Zero(t, *converted.MaxOutputTokens)
	require.NotNil(t, converted.Temperature)
	require.Zero(t, *converted.Temperature)
	require.Equal(t, "none", converted.Reasoning.Effort)
	require.Equal(t, "false", string(converted.ParallelToolCalls))
	var items []map[string]any
	require.NoError(t, common.Unmarshal(converted.Input, &items))
	require.Len(t, items, 4)
	require.Equal(t, "function_call", items[1]["type"])
	require.Equal(t, "call_1", items[1]["call_id"])
	require.Contains(t, string(converted.Input), "visible summary")
	require.Equal(t, "function_call_output", items[3]["type"])
	require.Contains(t, string(converted.Tools), `"strict":false`)
	restored, err := ResponsesRequestToClaudeRequest(converted)
	require.NoError(t, err)
	require.NotNil(t, restored.MaxTokens)
	require.Zero(t, *restored.MaxTokens)
	choice, err := common.Any2Type[dto.ClaudeToolChoice](restored.ToolChoice)
	require.NoError(t, err)
	require.True(t, choice.DisableParallelToolUse)
}

func TestDirectClaudeResponsesRejectsForeignSignedHistory(t *testing.T) {
	var req dto.ClaudeRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"gpt-test","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"summary","signature":"foreign-signature"}]}]}`, &req))
	_, err := ClaudeRequestToResponsesRequest(&req)
	require.ErrorContains(t, err, "signed thinking")
}
