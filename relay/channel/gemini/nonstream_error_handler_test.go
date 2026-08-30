package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiNonstreamTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash",
		},
	}
	return c, w, info
}

func geminiNonstreamHTTPResponse(t *testing.T, response dto.GeminiChatResponse) *http.Response {
	t.Helper()
	data, err := common.Marshal(response)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func TestGeminiAdaptorNonstreamReturnsPromptBlockErrorWithoutWriting(t *testing.T) {
	c, w, info := geminiNonstreamTestContext(t)
	blockReason := "SAFETY"
	resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiChatPromptFeedback{BlockReason: &blockReason},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount: 7,
			TotalTokenCount:  7,
		},
	})

	usageAny, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "request blocked by Gemini API: SAFETY")
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 7, usage.PromptTokens)
	require.Empty(t, w.Body.String(), "relay controller must remain responsible for writing the error response")
}

func TestGeminiAdaptorNonstreamReturnsEmptyCandidateError(t *testing.T) {
	c, w, info := geminiNonstreamTestContext(t)
	resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "empty response from Gemini API")
	require.Empty(t, w.Body.String())
}

func TestGeminiAdaptorNonstreamStillWritesSuccessfulResponse(t *testing.T) {
	c, w, info := geminiNonstreamTestContext(t)
	finishReason := "STOP"
	resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &finishReason,
			Content: dto.GeminiChatContent{
				Role:  "model",
				Parts: []dto.GeminiPart{{Text: "hello"}},
			},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     3,
			CandidatesTokenCount: 1,
			TotalTokenCount:      4,
		},
	})

	usageAny, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 4, usage.TotalTokens)
	require.Contains(t, w.Body.String(), `"content":"hello"`)
	require.Contains(t, w.Body.String(), `"finish_reason":"stop"`)
}
