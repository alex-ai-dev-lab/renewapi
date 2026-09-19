package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiStructuralTestStreamResponse(t *testing.T, response dto.GeminiChatResponse) *http.Response {
	t.Helper()

	data, err := common.Marshal(response)
	require.NoError(t, err)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(
			"data: " + string(data) + "\n\n",
		)),
	}
}

func TestGeminiStructuralFinishReason(t *testing.T) {
	structuralReasons := []string{
		"FINISH_REASON_UNSPECIFIED",
		"MALFORMED_FUNCTION_CALL",
		"UNEXPECTED_TOOL_CALL",
		"TOO_MANY_TOOL_CALLS",
		"MISSING_THOUGHT_SIGNATURE",
		"MALFORMED_RESPONSE",
		"NO_IMAGE",
		"IMAGE_OTHER",
	}

	for _, reason := range structuralReasons {
		t.Run(reason, func(t *testing.T) {
			require.True(t, geminiStructuralFinishReason(reason))
			require.True(t, geminiStructuralFinishReason(" "+reason+" "))
		})
	}

	nonStructuralReasons := []string{
		"",
		"STOP",
		"MAX_TOKENS",
		"SAFETY",
		"LANGUAGE",
		"IMAGE_SAFETY",
		"ESCALATION",
	}
	for _, reason := range nonStructuralReasons {
		t.Run("non_structural_"+reason, func(t *testing.T) {
			require.False(t, geminiStructuralFinishReason(reason))
		})
	}
}

func TestGeminiCompatibilityFinishReasonErrorIsSanitized(t *testing.T) {
	reason := " malformed_function_call "
	response := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        7,
			FinishReason: &reason,
			Content: dto.GeminiChatContent{
				Role: "model",
				Parts: []dto.GeminiPart{{
					Text: "sensitive generated content",
				}},
			},
		}},
	}

	apiErr := geminiCompatibilityFinishReasonError(response)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "candidate 7")
	require.Contains(t, apiErr.Error(), "MALFORMED_FUNCTION_CALL")
	require.NotContains(t, apiErr.Error(), "sensitive generated content")
	require.Nil(t, geminiCompatibilityFinishReasonError(nil))
}

func TestGeminiCompatibilityFinishReasonErrorKeepsStopNormal(t *testing.T) {
	reason := "STOP"
	response := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &reason,
		}},
	}

	require.Nil(t, geminiCompatibilityFinishReasonError(response))
}

func TestGeminiAdaptorNonstreamReturnsStructuralFinishReasonError(t *testing.T) {
	c, w, info := geminiNonstreamTestContext(t)
	reason := "MALFORMED_FUNCTION_CALL"

	resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &reason,
			Content: dto.GeminiChatContent{
				Role: "model",
				Parts: []dto.GeminiPart{{
					Text: "must not be forwarded",
				}},
			},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount: 3,
			TotalTokenCount:  3,
		},
	})

	usageAny, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "MALFORMED_FUNCTION_CALL")
	require.NotContains(t, apiErr.Error(), "must not be forwarded")
	require.Empty(t, w.Body.String())

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 3, usage.PromptTokens)
}

func TestGeminiAdaptorNonstreamMapsExtendedFilterFinishReasons(t *testing.T) {
	reasons := []string{
		"LANGUAGE",
		"IMAGE_SAFETY",
		"IMAGE_PROHIBITED_CONTENT",
		"IMAGE_RECITATION",
		"ESCALATION",
	}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			c, w, info := geminiNonstreamTestContext(t)
			finishReason := reason

			resp := geminiNonstreamHTTPResponse(t, dto.GeminiChatResponse{
				Candidates: []dto.GeminiChatCandidate{{
					Index:        0,
					FinishReason: &finishReason,
					Content: dto.GeminiChatContent{
						Role: "model",
						Parts: []dto.GeminiPart{{
							Text: "partial",
						}},
					},
				}},
			})

			_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

			require.Nil(t, apiErr)

			var converted dto.OpenAITextResponse
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &converted))
			require.Len(t, converted.Choices, 1)
			require.Equal(t, constant.FinishReasonContentFilter, converted.Choices[0].FinishReason)
		})
	}
}

func TestGeminiAdaptorStreamReturnsStructuralFinishReasonErrorBeforeWriting(t *testing.T) {
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
	resp := geminiStructuralTestStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{Index: 0, FinishReason: &reason}},
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
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		nil,
	)

	info := &relaycommon.RelayInfo{
		IsStream:       true,
		RelayMode:      relayconstant.RelayModeGemini,
		RelayFormat:    types.RelayFormatGemini,
		RequestURLPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		Request:        &dto.GeminiChatRequest{},
		ChannelMeta:    &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}

	reason := "MALFORMED_FUNCTION_CALL"
	resp := geminiStructuralTestStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{Index: 0, FinishReason: &reason}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.Contains(t, w.Body.String(), "MALFORMED_FUNCTION_CALL")
}

func TestGeminiAdaptorNativeStreamStillForwardsPromptBlock(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		nil,
	)

	info := &relaycommon.RelayInfo{
		IsStream:       true,
		RelayMode:      relayconstant.RelayModeGemini,
		RelayFormat:    types.RelayFormatGemini,
		RequestURLPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		Request:        &dto.GeminiChatRequest{},
		ChannelMeta:    &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}

	blockReason := "SAFETY"
	resp := geminiStructuralTestStreamResponse(t, dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiChatPromptFeedback{BlockReason: &blockReason},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.Contains(t, w.Body.String(), "SAFETY")
}
