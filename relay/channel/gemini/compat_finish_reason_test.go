package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiAdaptorNonstreamReturnsStructuralFinishReasonError(t *testing.T) {
	c, w, info := geminiNonstreamTestContext(t)
	reason := "MALFORMED_FUNCTION_CALL"
	resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &reason,
		}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "MALFORMED_FUNCTION_CALL")
	require.Empty(t, w.Body.String())
}

func TestGeminiAdaptorStreamReturnsStructuralFinishReasonError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
		Request:     &dto.GeneralOpenAIRequest{},
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}
	reason := "UNEXPECTED_TOOL_CALL"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &reason,
		}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "UNEXPECTED_TOOL_CALL")
	require.Empty(t, w.Body.String())
}

func TestGeminiAdaptorNativeStreamForwardsStructuralFinishReason(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", nil)
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		RelayMode:      relayconstant.RelayModeGemini,
		RelayFormat:    types.RelayFormatGemini,
		RequestURLPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		Request:        &dto.GeminiChatRequest{},
		ChannelMeta:    &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}
	reason := "MALFORMED_FUNCTION_CALL"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &reason,
		}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.Contains(t, w.Body.String(), "MALFORMED_FUNCTION_CALL")
}
