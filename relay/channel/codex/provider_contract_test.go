package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexProviderGoldenContract(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://chatgpt.example.test",
			ApiKey:         "{\"access_token\":\"token-test\",\"account_id\":\"acct-test\"}",
		},
	}
	a := &Adaptor{}
	url, err := a.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.example.test/backend-api/codex/responses", url)
	header := make(http.Header)
	require.NoError(t, a.SetupRequestHeader(c, &header, info))
	require.Equal(t, "Bearer token-test", header.Get("Authorization"))
	require.Equal(t, "acct-test", header.Get("chatgpt-account-id"))
	zero := 0.0
	max := uint(0)
	converted, err := a.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{Input: []byte("\"hello\""), Temperature: &zero, TopP: &zero, MaxOutputTokens: &max})
	require.NoError(t, err)
	got := converted.(dto.OpenAIResponsesRequest)
	require.JSONEq(t, "false", string(got.Store))
	require.Nil(t, got.Temperature)
	require.Nil(t, got.TopP)
	require.Nil(t, got.MaxOutputTokens)
}
