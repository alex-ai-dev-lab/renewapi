package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesCompactRequestKeepsEndpointAndMapsModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabledIdentitySetting, err := appcommon.Marshal(model.ClientIdentitySetting{Enabled: false})
	require.NoError(t, err)
	appcommon.OptionMapRWMutex.Lock()
	if appcommon.OptionMap == nil {
		appcommon.OptionMap = map[string]string{}
	}
	originalIdentitySetting, hadIdentitySetting := appcommon.OptionMap["client_identity_setting"]
	originalHeaderRuleSetting, hadHeaderRuleSetting := appcommon.OptionMap["header_rule_setting"]
	appcommon.OptionMap["client_identity_setting"] = string(disabledIdentitySetting)
	appcommon.OptionMap["header_rule_setting"] = `{"enabled":false,"apply_to_channel_test":true,"groups":[]}`
	appcommon.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		appcommon.OptionMapRWMutex.Lock()
		defer appcommon.OptionMapRWMutex.Unlock()
		if hadIdentitySetting {
			appcommon.OptionMap["client_identity_setting"] = originalIdentitySetting
		} else {
			delete(appcommon.OptionMap, "client_identity_setting")
		}
		if hadHeaderRuleSetting {
			appcommon.OptionMap["header_rule_setting"] = originalHeaderRuleSetting
		} else {
			delete(appcommon.OptionMap, "header_rule_setting")
		}
	})

	var upstreamPath string
	var upstreamBody struct {
		Model string `json:"model"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		require.NoError(t, appcommon.Unmarshal(body, &upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmp_test","object":"response.compaction","output":[],"usage":{"input_tokens":1,"output_tokens":0,"total_tokens":1}}`))
	}))
	defer server.Close()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("model_mapping", `{"gpt-5.4-openai-compact":"gpt-5.4"}`)

	request := &dto.OpenAIResponsesRequest{
		Model: "gpt-5.4-openai-compact",
		Input: []byte(`[{"role":"user","content":"compact this"}]`),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		RelayFormat:     types.RelayFormatOpenAIResponsesCompaction,
		OriginModelName: "gpt-5.4-openai-compact",
		RequestURLPath:  "/v1/responses/compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeOpenAI,
			ChannelBaseUrl:    server.URL,
			ApiType:           appconstant.APITypeOpenAI,
			UpstreamModelName: "gpt-5.4-openai-compact",
		},
	}

	require.NoError(t, helper.ModelMappedHelper(ctx, info, request))
	adaptor := &Adaptor{}
	adaptor.Init(info)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(ctx, info, *request)
	require.NoError(t, err)
	body, err := appcommon.Marshal(converted)
	require.NoError(t, err)

	resp, err := adaptor.DoRequest(ctx, info, bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.(*http.Response).Body.Close()

	require.Equal(t, "/v1/responses/compact", upstreamPath)
	require.Equal(t, "gpt-5.4", upstreamBody.Model)
}
