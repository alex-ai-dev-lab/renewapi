package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func normalizationInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{UserId: 1, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 257, ChannelType: constant.ChannelTypeOpenAI, ChannelSetting: dto.ChannelSettings{ResponsesRequestProfile: "codex"}}}
}

func TestFinalizeCodexFieldsAndScope(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("Session-Id", "synthetic-session")
	info := normalizationInfo()
	body := []byte(`{"model":"gpt-test","input":"hello","include":["file_search_call.results","file_search_call.results"],"max_output_tokens":64,"temperature":0,"top_p":0,"parallel_tool_calls":false,"future":{"zero":0}}`)
	final, err := FinalizeUpstreamJSON(c, info, "/v1/responses", body)
	require.NoError(t, err)
	require.Equal(t, []any{"file_search_call.results", "reasoning.encrypted_content"}, gjson.GetBytes(final, "include").Value())
	require.Equal(t, "input_text", gjson.GetBytes(final, "input.0.content.0.type").String())
	require.Equal(t, "hello", gjson.GetBytes(final, "input.0.content.0.text").String())
	require.False(t, gjson.GetBytes(final, "temperature").Exists())
	require.False(t, gjson.GetBytes(final, "top_p").Exists())
	require.Equal(t, int64(64), gjson.GetBytes(final, "max_output_tokens").Int())
	require.Equal(t, "false", gjson.GetBytes(final, "parallel_tool_calls").Raw)
	require.True(t, gjson.GetBytes(final, "future.zero").Exists())
	require.True(t, c.GetBool("responses_upstream_stream"))
	require.NotContains(t, info.RequestNormalization, "synthetic-session")
	key := gjson.GetBytes(final, "prompt_cache_key").String()
	for _, next := range []*relaycommon.RelayInfo{info, normalizationInfo()} {
		retried, err := FinalizeUpstreamJSON(c, next, "/v1/responses", body)
		require.NoError(t, err)
		require.Equal(t, key, gjson.GetBytes(retried, "prompt_cache_key").String())
	}
	for _, field := range []string{"user", "channel"} {
		next := normalizationInfo()
		if field == "user" {
			next.UserId++
		} else {
			next.ChannelId++
		}
		other, err := FinalizeUpstreamJSON(c, next, "/v1/responses", body)
		require.NoError(t, err)
		require.NotEqual(t, key, gjson.GetBytes(other, "prompt_cache_key").String())
	}
	info.ChannelSetting.ResponsesRequestProfile = "auto"
	info.ChannelSetting.RequiresCodexIdentity = common.GetPointer(true)
	standard, err := FinalizeUpstreamJSON(c, info, "/v1/responses", body)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(standard))
	info.ChannelSetting.ResponsesRequestProfile = "codex"
	compact, err := FinalizeUpstreamJSON(c, info, "/v1/responses/compact", body)
	require.NoError(t, err)
	require.Equal(t, body, compact)
}

func TestFinalizeCodexRejectsUnpreservableParameters(t *testing.T) {
	for _, fields := range []string{`"store":true`, `"previous_response_id":"resp_old"`, `"conversation":"conv_old"`, `"include":"bad"`, `"prompt_cache_key":42`, `"instructions":[]`, `"input":42`} {
		t.Run(fields, func(t *testing.T) {
			_, err := FinalizeUpstreamJSON(nil, normalizationInfo(), "/v1/responses", []byte(`{"model":"test","input":"hello",`+fields+`}`))
			var rejection *types.NewAPIError
			require.ErrorAs(t, err, &rejection)
			require.Equal(t, 400, rejection.StatusCode)
			require.Equal(t, rejection.Error(), types.PublicRelayError(rejection).Error())
		})
	}
	info := normalizationInfo()
	info.ChannelType = constant.ChannelTypeCodex
	body := []byte(`{"model":"test","input":"hello","max_output_tokens":64,"prompt_cache_key":"caller-key"}`)
	_, err := FinalizeUpstreamJSON(nil, info, "/backend-api/codex/responses", body)
	require.ErrorContains(t, err, "max_output_tokens")
	info.ChannelSetting.ResponsesSupportsMaxOutputTokens = common.GetPointer(true)
	final, err := FinalizeUpstreamJSON(nil, info, "/backend-api/codex/responses", body)
	require.NoError(t, err)
	require.Equal(t, "caller-key", gjson.GetBytes(final, "prompt_cache_key").String())
	require.Equal(t, int64(64), gjson.GetBytes(final, "max_output_tokens").Int())
}
