package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeProviderGoldenContract(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-version", "2024-01-01")
	info := &relaycommon.RelayInfo{IsClaudeBetaQuery: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://claude.example.test", ApiKey: "key-test"}}
	a := &Adaptor{}
	url, err := a.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://claude.example.test/v1/messages?beta=true", url)
	header := make(http.Header)
	require.NoError(t, a.SetupRequestHeader(c, &header, info))
	require.Equal(t, "key-test", header.Get("x-api-key"))
	require.Equal(t, "2024-01-01", header.Get("anthropic-version"))
	request := &dto.ClaudeRequest{Model: "claude-test", Stream: new(bool)}
	converted, err := a.ConvertClaudeRequest(c, info, request)
	require.NoError(t, err)
	require.Same(t, request, converted)
}
