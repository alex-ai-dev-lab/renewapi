package dto

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativeRequestExtensionRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		target     any
	}{
		{"claude", `{"model":"claude","future_execution":{"enabled":false},"thinking":{"type":"adaptive","display":"summarized","future_budget":0},"messages":[{"role":"assistant","future_message":true,"content":[{"type":"thinking","thinking":"visible","signature":"opaque-signature"},{"type":"redacted_thinking","data":"opaque-data"},{"type":"tool_result","tool_use_id":"call_a","is_error":false,"content":"done","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`, &ClaudeRequest{}},
		{"responses", `{"model":"gpt","future_execution":{"enabled":false},"reasoning":{"effort":"high","future_budget":0},"input":[{"role":"user","content":[{"type":"input_text","text":"hi","future":0}]}]}`, &OpenAIResponsesRequest{}},
		{"chat", `{"model":"gpt","future_execution":{"enabled":false},"messages":[{"role":"user","content":"hi","future_message":0}]}`, &GeneralOpenAIRequest{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, common.Unmarshal([]byte(tc.body), tc.target))
			out, err := common.Marshal(tc.target)
			require.NoError(t, err)
			require.JSONEq(t, tc.body, string(out))
		})
	}
	var block ClaudeMediaMessage
	body := `{"type":"image","source":{"type":"url","url":"https://example.test/image","future":false},"future_block":0}`
	require.NoError(t, common.Unmarshal([]byte(body), &block))
	out, err := common.Marshal(block)
	require.NoError(t, err)
	require.JSONEq(t, body, string(out))
}

func TestExtensionsCannotRestoreRemovedDeclaredFields(t *testing.T) {
	req := OpenAIResponsesRequest{Model: "mapped", RequestExtensions: RequestExtensions{ExtraFields: map[string]common.RawMessage{"model": []byte(`"unmapped"`), "service_tier": []byte(`"priority"`), "future": []byte(`false`)}}}
	out, err := common.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"mapped","future":false}`, string(out))
	req.ExtraFields["MODEL"] = []byte(`"unmapped"`)
	out, err = common.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"mapped","future":false}`, string(out))
}
