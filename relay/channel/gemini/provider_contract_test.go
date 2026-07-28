package gemini

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiProviderGoldenContract(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-test:streamGenerateContent", nil)
	info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://gemini.example.test", ApiKey: "key-test", UpstreamModelName: "gemini-test"}}
	a := &Adaptor{}
	url, err := a.GetRequestURL(info)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(url, "/models/gemini-test:streamGenerateContent?alt=sse"), url)
	header := make(http.Header)
	require.NoError(t, a.SetupRequestHeader(c, &header, info))
	require.Equal(t, "key-test", header.Get("x-goog-api-key"))
	request := &dto.GeminiChatRequest{}
	converted, err := a.ConvertGeminiRequest(c, info, request)
	require.NoError(t, err)
	require.Same(t, request, converted)
}
