package openaicompat

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"testing"
)

func TestChatResponsesPreservesCacheAndReasoning(t *testing.T) {
	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt","messages":[{"role":"user","content":"hi"}],"include":["reasoning.encrypted_content"],"client_metadata":{"installation":"caller"},"prompt_cache_key":"caller","prompt_cache_retention":"24h","safety_identifier":"safety","reasoning":{"effort":"high","summary":"auto","future":0},"verbosity":"low","parallel_tool_calls":false}`), &req))
	out, err := ChatCompletionsRequestToResponsesRequest(&req)
	require.NoError(t, err)
	b, err := common.Marshal(out)
	require.NoError(t, err)
	for path, value := range map[string]string{"prompt_cache_key": "caller", "prompt_cache_retention": "24h", "reasoning.summary": "auto", "reasoning.future": "0", "text.verbosity": "low", "parallel_tool_calls": "false", "client_metadata.installation": "caller"} {
		require.Equal(t, value, gjson.GetBytes(b, path).String(), path)
	}
}

func TestClaudeResponsesPreservesToolFailureAndFormat(t *testing.T) {
	var req dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt","max_tokens":64,"thinking":{"type":"enabled","budget_tokens":2048,"display":"summarized"},"output_config":{"format":{"type":"json_schema","schema":{"type":"object"},"strict":false}},"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_a","is_error":true,"content":[{"type":"text","text":"failed"},{"type":"image","source":{"type":"url","url":"https://example.test/image"}}]}]}]}`), &req))
	out, err := ClaudeRequestToResponsesRequest(&req)
	require.NoError(t, err)
	require.Equal(t, "auto", out.Reasoning.Summary)
	require.Equal(t, "low", out.Reasoning.Effort)
	item := gjson.ParseBytes(out.Input).Array()[0]
	require.Equal(t, "call_a", item.Get("call_id").String())
	result := gjson.Parse(item.Get("output").String())
	require.True(t, result.Get("is_error").Bool())
	require.Equal(t, "failed", result.Get("content.0.text").String())
	require.Equal(t, "https://example.test/image", result.Get("content.1.image_url").String())
	require.Equal(t, "false", gjson.GetBytes(out.Text, "format.strict").Raw)
}

func TestClaudeResponsesRejectsUnknownExecutionFields(t *testing.T) {
	for _, body := range []string{
		`{"model":"gpt","container":{"id":"old"}}`,
		`{"model":"gpt","thinking":{"type":"adaptive","future_execution":true}}`,
		`{"model":"gpt","output_config":{"future_execution":true}}`,
		`{"model":"gpt","tools":[{"name":"a","defer_loading":true,"input_schema":{}}]}`,
		`{"model":"gpt","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"hi","signature":"opaque"}]}]}`,
		`{"model":"gpt","messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"a","content":[{"type":"text","text":"ok"},{"type":"future_execution"}]}]}]}`,
		`{"model":"gpt","messages":[{"role":"user","content":[{"type":"image"}]}]}`,
	} {
		var req dto.ClaudeRequest
		require.NoError(t, common.Unmarshal([]byte(body), &req))
		_, err := ClaudeRequestToResponsesRequest(&req)
		require.Error(t, err, body)
	}
}
