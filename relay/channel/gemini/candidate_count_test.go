package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiConvertOpenAIRequestMapsNToCandidateCount(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	n := 2
	request := &dto.GeneralOpenAIRequest{
		N: &n,
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)

	require.NoError(t, err)
	geminiRequest, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.NotNil(t, geminiRequest.GenerationConfig.CandidateCount)
	require.Equal(t, 2, *geminiRequest.GenerationConfig.CandidateCount)
}

func TestExpectedGeminiStreamCandidateCountUsesOpenAIN(t *testing.T) {
	n := 2
	info := &relaycommon.RelayInfo{Request: &dto.GeneralOpenAIRequest{N: &n}}

	require.Equal(t, 2, expectedGeminiStreamCandidateCount(info))
}

func TestExpectedGeminiStreamCandidateCountDefaultsToOne(t *testing.T) {
	info := &relaycommon.RelayInfo{Request: &dto.GeneralOpenAIRequest{}}

	require.Equal(t, 1, expectedGeminiStreamCandidateCount(info))
}
