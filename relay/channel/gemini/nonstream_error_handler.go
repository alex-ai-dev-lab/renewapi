package gemini

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func geminiCandidateOpenAIFinishReason(candidate dto.GeminiChatCandidate) string {
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			return constant.FinishReasonToolCalls
		}
	}
	if candidate.FinishReason == nil {
		return constant.FinishReasonStop
	}
	switch strings.ToUpper(strings.TrimSpace(*candidate.FinishReason)) {
	case "STOP":
		return constant.FinishReasonStop
	case "MAX_TOKENS":
		return constant.FinishReasonLength
	case "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII",
		"IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION", "ESCALATION", "OTHER":
		return constant.FinishReasonContentFilter
	default:
		return constant.FinishReasonContentFilter
	}
}

func normalizeGeminiChoiceFinishReasons(response *dto.GeminiChatResponse, converted *dto.OpenAITextResponse) {
	if response == nil || converted == nil || len(response.Candidates) == 0 || len(converted.Choices) == 0 {
		return
	}
	candidates := make(map[int64]dto.GeminiChatCandidate, len(response.Candidates))
	for _, candidate := range response.Candidates {
		candidates[candidate.Index] = candidate
	}
	for i := range converted.Choices {
		candidate, ok := candidates[int64(converted.Choices[i].Index)]
		if !ok {
			continue
		}
		converted.Choices[i].FinishReason = geminiCandidateOpenAIFinishReason(candidate)
	}
}

func GeminiChatHandlerWithErrorReturn(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	logger.LogDebug(c, "Gemini response body: %s", responseBody)

	var geminiResponse dto.GeminiChatResponse
	if err := common.Unmarshal(responseBody, &geminiResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
	if len(geminiResponse.Candidates) == 0 {
		var newAPIError *types.NewAPIError
		if geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "gemini_block_reason="+*geminiResponse.PromptFeedback.BlockReason)
			newAPIError = types.NewOpenAIError(
				errors.New("request blocked by Gemini API: "+*geminiResponse.PromptFeedback.BlockReason),
				types.ErrorCodePromptBlocked,
				http.StatusBadRequest,
			)
		} else {
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "gemini_empty_candidates")
			newAPIError = types.NewOpenAIError(
				errors.New("empty response from Gemini API"),
				types.ErrorCodeEmptyResponse,
				http.StatusInternalServerError,
			)
		}
		service.ResetStatusCode(newAPIError, c.GetString("status_code_mapping"))
		return &usage, newAPIError
	}
	if finishErr := geminiCompatibilityFinishReasonError(&geminiResponse); finishErr != nil {
		return &usage, finishErr
	}

	fullTextResponse := responseGeminiChat2OpenAI(c, &geminiResponse)
	normalizeGeminiChoiceFinishReasons(&geminiResponse, fullTextResponse)
	fullTextResponse.Model = info.UpstreamModelName
	fullTextResponse.Usage = usage

	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		responseBody, err = common.Marshal(fullTextResponse)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatClaude:
		claudeResp := service.ResponseOpenAI2Claude(fullTextResponse, info)
		responseBody, err = common.Marshal(claudeResp)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatGemini:
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &usage, nil
}
