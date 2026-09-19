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
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiStreamTestContext(t *testing.T, request *dto.GeminiChatRequest) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "gemini-3-flash-preview",
		Request:         request,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}
	return c, info
}

func geminiStreamResponse(t *testing.T, chunks ...dto.GeminiChatResponse) *http.Response {
	t.Helper()
	var body bytes.Buffer
	for _, chunk := range chunks {
		data, err := common.Marshal(chunk)
		require.NoError(t, err)
		body.WriteString("data: ")
		body.Write(data)
		body.WriteString("\n\n")
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body.Bytes()))}
}

func TestGeminiStreamCompletionGuardRejectsPartialEOF(t *testing.T) {
	c, info := geminiStreamTestContext(t, nil)
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:   0,
			Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "partial"}}},
		}},
	})

	usage, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool { return true })

	require.NotNil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "incomplete Gemini stream response")
}

func TestGeminiStreamCompletionGuardAcceptsFinishReasonAtEOF(t *testing.T) {
	c, info := geminiStreamTestContext(t, nil)
	finish := "STOP"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:        0,
			FinishReason: &finish,
			Content:      dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "done"}}},
		}},
	})

	usage, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool { return true })

	require.NotNil(t, usage)
	require.Nil(t, apiErr)
}

func TestGeminiStreamCompletionGuardRequiresAllRequestedCandidates(t *testing.T) {
	candidateCount := 2
	request := &dto.GeminiChatRequest{GenerationConfig: dto.GeminiChatGenerationConfig{CandidateCount: &candidateCount}}
	c, info := geminiStreamTestContext(t, request)
	finish := "STOP"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{Index: 0, FinishReason: &finish, Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "first"}}}},
			{Index: 1, Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "second partial"}}}},
		},
	})

	_, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool { return true })

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "expected=2 seen=2 finished=1")
}

func TestGeminiStreamCompletionGuardAcceptsAllRequestedCandidates(t *testing.T) {
	candidateCount := 2
	request := &dto.GeminiChatRequest{GenerationConfig: dto.GeminiChatGenerationConfig{CandidateCount: &candidateCount}}
	c, info := geminiStreamTestContext(t, request)
	stop := "STOP"
	length := "MAX_TOKENS"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{Index: 0, FinishReason: &stop},
			{Index: 1, FinishReason: &length},
		},
	})

	_, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool { return true })

	require.Nil(t, apiErr)
}

func TestGeminiStreamCompletionGuardRejectsMalformedFrame(t *testing.T) {
	c, info := geminiStreamTestContext(t, nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("data: {not-json}\n\n")),
	}

	_, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool { return true })

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "invalid Gemini stream response")
}

func TestGeminiStreamCompletionGuardReturnsPromptBlock(t *testing.T) {
	c, info := geminiStreamTestContext(t, nil)
	blockReason := "SAFETY"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiChatPromptFeedback{BlockReason: &blockReason},
	})
	callbackCalled := false

	_, apiErr := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(string, *dto.GeminiChatResponse) bool {
		callbackCalled = true
		return true
	})

	require.False(t, callbackCalled)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "request blocked by Gemini API: SAFETY")
}

func TestGeminiAdaptorChatStreamUsesCompletionGuard(t *testing.T) {
	c, info := geminiStreamTestContext(t, nil)
	info.IsStream = true
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:   0,
			Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "partial"}}},
		}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
}

func TestGeminiAdaptorNativeStreamUsesCompletionGuard(t *testing.T) {
	request := &dto.GeminiChatRequest{}
	c, info := geminiStreamTestContext(t, request)
	info.IsStream = true
	info.RelayMode = relayconstant.RelayModeGemini
	info.RequestURLPath = "/v1beta/models/gemini-3-flash-preview:streamGenerateContent"
	resp := geminiStreamResponse(t, dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Index:   0,
			Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: "partial"}}},
		}},
	})

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
}
