package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesChatRoundTripPreservesCacheOptions(t *testing.T) {
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt","input":"hello","prompt_cache_options":{"enabled":false,"ttl":0}}`), &req))
	chat, err := ResponsesRequestToChatCompletionsRequest(&req)
	require.NoError(t, err)
	out, err := ChatCompletionsRequestToResponsesRequest(chat)
	require.NoError(t, err)
	require.JSONEq(t, string(req.PromptCacheOptions), string(out.PromptCacheOptions))
	b, err := common.Marshal(chat)
	require.NoError(t, err)
	require.Equal(t, "false", gjson.GetBytes(b, "prompt_cache_options.enabled").Raw)
	require.Equal(t, "0", gjson.GetBytes(b, "prompt_cache_options.ttl").Raw)
}
