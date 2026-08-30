package gemini

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func geminiStreamHandlerWithCompletionGuard(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	resp *http.Response,
	callback func(data string, geminiResponse *dto.GeminiChatResponse) bool,
) (*dto.Usage, *types.NewAPIError) {
	tracker := newGeminiStreamCompletionTracker(info)
	promptBlockReason := ""
	callbackStopped := false
	var generationErr *types.NewAPIError

	usage, err := geminiStreamHandler(c, info, resp, func(data string, geminiResponse *dto.GeminiChatResponse) bool {
		tracker.Observe(geminiResponse)
		if info == nil || info.RelayFormat != types.RelayFormatGemini {
			if finishErr := geminiCompatibilityFinishReasonError(geminiResponse); finishErr != nil {
				generationErr = finishErr
				return false
			}
		}
		if len(geminiResponse.Candidates) == 0 && geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			promptBlockReason = *geminiResponse.PromptFeedback.BlockReason
			if info == nil || info.RelayFormat != types.RelayFormatGemini {
				return false
			}
		}
		ok := callback(data, geminiResponse)
		if !ok {
			callbackStopped = true
		}
		return ok
	})
	if err != nil {
		return usage, err
	}
	if generationErr != nil {
		return usage, generationErr
	}
	if streamErr := geminiStreamOutcomeError(info, tracker, promptBlockReason, callbackStopped); streamErr != nil {
		return usage, streamErr
	}
	return usage, nil
}

func GeminiTextGenerationStreamHandlerWithCompletionGuard(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)
	return geminiStreamHandlerWithCompletionGuard(c, info, resp, func(data string, _ *dto.GeminiChatResponse) bool {
		if err := helper.StringData(c, data); err != nil {
			logger.LogError(c, "failed to write stream data: "+err.Error())
			return false
		}
		info.SendResponseCount++
		return true
	})
}

func GeminiChatStreamHandlerWithCompletionGuard(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	id := helper.GetResponseID(c)
	createAt := common.GetTimestamp()
	finishReason := constant.FinishReasonStop
	expectedCandidates := expectedGeminiStreamCandidateCount(info)
	toolCallIndexByChoice := make(map[int]map[string]int)
	nextToolCallIndexByChoice := make(map[int]int)

	usage, err := geminiStreamHandlerWithCompletionGuard(c, info, resp, func(data string, geminiResponse *dto.GeminiChatResponse) bool {
		response, isStop := streamResponseGeminiChat2OpenAI(geminiResponse)

		response.Id = id
		response.Created = createAt
		response.Model = info.UpstreamModelName
		if response.IsToolCall() {
			finishReason = constant.FinishReasonToolCalls
			if info.RelayFormat == types.RelayFormatClaude {
				for choiceIdx := range response.Choices {
					response.Choices[choiceIdx].FinishReason = nil
				}
			}
		}
		for choiceIdx := range response.Choices {
			choiceKey := response.Choices[choiceIdx].Index
			for toolIdx := range response.Choices[choiceIdx].Delta.ToolCalls {
				tool := &response.Choices[choiceIdx].Delta.ToolCalls[toolIdx]
				if tool.ID == "" {
					continue
				}
				m := toolCallIndexByChoice[choiceKey]
				if m == nil {
					m = make(map[string]int)
					toolCallIndexByChoice[choiceKey] = m
				}
				if idx, ok := m[tool.ID]; ok {
					tool.SetIndex(idx)
					continue
				}
				idx := nextToolCallIndexByChoice[choiceKey]
				nextToolCallIndexByChoice[choiceKey] = idx + 1
				m[tool.ID] = idx
				tool.SetIndex(idx)
			}
		}

		logger.LogDebug(c, "info.SendResponseCount = %d", info.SendResponseCount)
		if info.SendResponseCount == 0 {
			emptyResponse := helper.GenerateStartEmptyResponse(id, createAt, info.UpstreamModelName, nil)
			if expectedCandidates > 1 {
				emptyResponse.Choices = make([]dto.ChatCompletionsStreamResponseChoice, expectedCandidates)
				for choiceIndex := 0; choiceIndex < expectedCandidates; choiceIndex++ {
					emptyResponse.Choices[choiceIndex] = dto.ChatCompletionsStreamResponseChoice{
						Index: choiceIndex,
						Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
							Role:    "assistant",
							Content: common.GetPointer(""),
						},
					}
				}
			}
			if response.IsToolCall() && expectedCandidates == 1 {
				if len(emptyResponse.Choices) > 0 && len(response.Choices) > 0 {
					toolCalls := response.Choices[0].Delta.ToolCalls
					copiedToolCalls := make([]dto.ToolCallResponse, len(toolCalls))
					for idx := range toolCalls {
						copiedToolCalls[idx] = toolCalls[idx]
						copiedToolCalls[idx].Function.Arguments = ""
					}
					emptyResponse.Choices[0].Delta.ToolCalls = copiedToolCalls
				}
				finishReason = constant.FinishReasonToolCalls
				if err := handleStream(c, info, emptyResponse); err != nil {
					logger.LogError(c, err.Error())
				}

				response.ClearToolCalls()
				if response.IsFinished() {
					response.Choices[0].FinishReason = nil
				}
			} else if err := handleStream(c, info, emptyResponse); err != nil {
				logger.LogError(c, err.Error())
			}
		}

		if err := handleStream(c, info, response); err != nil {
			logger.LogError(c, err.Error())
		}
		if isStop && info.RelayFormat != types.RelayFormatClaude && expectedCandidates == 1 {
			_ = handleStream(c, info, helper.GenerateStopResponse(id, createAt, info.UpstreamModelName, finishReason))
		}
		return true
	})

	if err != nil {
		return usage, err
	}

	response := helper.GenerateFinalUsageResponse(id, createAt, info.UpstreamModelName, *usage)
	if info.RelayFormat == types.RelayFormatClaude && info.ClaudeConvertInfo != nil && !info.ClaudeConvertInfo.Done {
		response = helper.GenerateStopResponse(id, createAt, info.UpstreamModelName, finishReason)
		response.Usage = usage
	}
	if handleErr := handleFinalStream(c, info, response); handleErr != nil {
		common.SysLog("send final response failed: " + handleErr.Error())
	}
	return usage, nil
}
