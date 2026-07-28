package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProviderGoldenContract(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions, RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI, ChannelBaseUrl: "https://api.example.test",
			ApiKey: "sk-test", UpstreamModelName: "gpt-5-test",
		},
	}
	a := &Adaptor{}
	url, err := a.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.example.test/v1/chat/completions", url)
	header := make(http.Header)
	require.NoError(t, a.SetupRequestHeader(c, &header, info))
	require.Equal(t, "Bearer sk-test", header.Get("Authorization"))

	zero := 0.0
	request := &dto.GeneralOpenAIRequest{Model: "gpt-5-test", Stream: common.GetPointer(true), Temperature: &zero, TopP: &zero}
	converted, err := a.ConvertOpenAIRequest(c, info, request)
	require.NoError(t, err)
	got := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, *got.Stream)
	require.Nil(t, got.Temperature)
	require.Nil(t, got.TopP)
}
