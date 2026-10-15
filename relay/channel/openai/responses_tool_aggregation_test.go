package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateResponsesToolIdentityAndMixedOutput(t *testing.T) {
	for _, text := range []string{"", "Checking now."} {
		t.Run("text="+text, func(t *testing.T) {
			body := `data: {"type":"response.output_text.delta","delta":"` + text + `"}` + "\n\n" +
				`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}` + "\n\n" +
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{}"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n"
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}
			info.AntiPoisonToolsDeclared = true
			info.AntiPoisonAllowedTools = map[string]bool{"lookup": true}
			resp, _, apiErr := aggregateResponsesStreamToResponse(ctx, info, &http.Response{Body: io.NopCloser(strings.NewReader(body))})
			require.Nil(t, apiErr)
			calls := 0
			var returnedText strings.Builder
			for _, item := range resp.Output {
				for _, part := range item.Content {
					returnedText.WriteString(part.Text)
				}
				if item.Type != "function_call" {
					continue
				}
				calls++
				require.Equal(t, "call_1", item.CallId)
				require.Equal(t, "lookup", item.Name)
				require.Equal(t, "{}", item.ArgumentsString())
			}
			require.Equal(t, 1, calls)
			require.Equal(t, text, returnedText.String())
		})
	}
}

func TestAggregateResponsesPreservesCustomToolInput(t *testing.T) {
	body := "event: response.completed\ndata: {\ndata: " + `"response":{"output":[{"type":"custom_tool_call","id":"ct_1","call_id":"call_1","name":"emit","input":"hello"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n"
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}
	resp, _, apiErr := aggregateResponsesStreamToResponse(ctx, info, &http.Response{Body: io.NopCloser(strings.NewReader(body))})
	require.Nil(t, apiErr)
	encoded, err := common.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"input":"hello"`)
}

func TestAggregateResponsesToolDoneFallback(t *testing.T) {
	for _, test := range []struct{ name, item, events, field, value string }{
		{"function_done", `{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}`,
			`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{}"}`, "arguments", "{}"},
		{"custom_delta", `{"type":"custom_tool_call","id":"fc_1","call_id":"call_1","name":"emit","input":""}`,
			`data: {"type":"response.custom_tool_call_input.delta","item_id":"fc_1","delta":"hello"}`, "input", "hello"},
		{"custom_done", `{"type":"custom_tool_call","id":"fc_1","call_id":"call_1","name":"emit","input":""}`,
			`data: {"type":"response.custom_tool_call_input.done","item_id":"fc_1","input":"hello"}`, "input", "hello"},
		{"custom_empty", `{"type":"custom_tool_call","id":"fc_1","call_id":"call_1","name":"emit","input":""}`,
			`data: {"type":"response.custom_tool_call_input.done","item_id":"fc_1","input":""}`, "input", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := `data: {"type":"response.output_item.added","item":` + test.item + "}\n\n" + test.events + "\n\n" +
				`data: {"type":"response.completed","response":{"output":[],"usage":{"total_tokens":2}}}` + "\n\n"
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}, AntiPoisonToolsDeclared: true, AntiPoisonAllowedTools: map[string]bool{"lookup": true, "emit": true}}
			resp, _, apiErr := aggregateResponsesStreamToResponse(ctx, info, &http.Response{Body: io.NopCloser(strings.NewReader(body))})
			require.Nil(t, apiErr)
			require.Len(t, resp.Output, 1)
			require.Equal(t, "call_1", resp.Output[0].CallId)
			encoded, err := common.Marshal(resp.Output[0])
			require.NoError(t, err)
			var item map[string]any
			require.NoError(t, common.Unmarshal(encoded, &item))
			require.Equal(t, test.value, item[test.field])
		})
	}
}
