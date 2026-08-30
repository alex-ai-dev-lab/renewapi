package claude

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relay/responsebridge"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func claudeStreamHandlerWithCompletionGuard(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	cfg := antipoison.ResponseGuardConfig(info)
	if cfg.Enabled && antipoison.StreamModeForConfig(cfg) == operation_setting.AntiPoisonStreamAggregateThenReplay {
		return claudeAggregateStreamThenReplayWithCompletionGuard(c, resp, info)
	}
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var err *types.NewAPIError
	var responseProof *antipoison.ProofStreamValidator
	preflightBuffer := antipoison.NewStreamPreflightBuffer(cfg)
	var pendingClaudeData []claudePendingStreamData
	if info.AntiPoisonResponseProofNonce != "" && antipoison.ResponseProofEnabled(info) {
		responseProof = antipoison.NewProofStreamValidator(info.AntiPoisonResponseProofNonce, antipoison.ResponseGuardConfig(info))
	}
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if responseProof != nil && !responseProof.Verified() {
			var claudeResponse dto.ClaudeResponse
			if unmarshalErr := common.UnmarshalJsonStr(data, &claudeResponse); unmarshalErr != nil {
				err = types.NewError(unmarshalErr, types.ErrorCodeBadResponseBody)
				sr.Stop(err)
				return
			}
			cleanData, hold, proofErr := prepareClaudeStreamResponseProof(&claudeResponse, data, responseProof)
			if proofErr != nil {
				err = proofErr
				sr.Stop(proofErr)
				return
			}
			if hold {
				if cleanData != "" {
					pendingClaudeData = append(pendingClaudeData, claudePendingStreamData{response: claudeResponse, data: cleanData})
				}
				return
			}
			for _, pending := range pendingClaudeData {
				err = handleClaudeStreamResponseDataWithPreflight(c, info, claudeInfo, pending.data, preflightBuffer)
				if err != nil {
					sr.Stop(err)
					return
				}
			}
			pendingClaudeData = nil
			data = cleanData
		}
		err = handleClaudeStreamResponseDataWithPreflight(c, info, claudeInfo, data, preflightBuffer)
		if err != nil {
			sr.Stop(err)
		}
	})
	if err != nil {
		return nil, err
	}
	if responseProof != nil {
		if proofErr := responseProof.Finalize(); proofErr != nil {
			logger.LogError(c, "anti-poison claude proof stream validation failed: "+proofErr.Error())
			return nil, types.NewError(antipoison.ResponseProofFailureError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
	}
	if preflightBuffer != nil {
		chunks, result, preflightErr := preflightBuffer.Finalize()
		if preflightErr != nil {
			antipoison.RecordOpaqueResult(c, result)
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, preflightBuffer.RawData())
			return nil, types.NewError(preflightErr, types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordOpaqueResult(c, result)
		for _, chunk := range chunks {
			if handleErr := HandleStreamResponseData(c, info, claudeInfo, chunk); handleErr != nil {
				return nil, handleErr
			}
		}
	}
	if completionErr := claudeStreamCompletionError(info, claudeInfo); completionErr != nil {
		return nil, completionErr
	}

	HandleStreamFinalResponse(c, info, claudeInfo)
	return claudeInfo.Usage, nil
}

func claudeAggregateStreamThenReplayWithCompletionGuard(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var finalErr *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		var claudeResponse dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(data, &claudeResponse); err != nil {
			finalErr = types.NewError(err, types.ErrorCodeBadResponseBody)
			sr.Stop(finalErr)
			return
		}
		if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
			finalErr = types.WithClaudeError(*claudeError, http.StatusInternalServerError)
			sr.Stop(finalErr)
			return
		}
		if claudeResponse.StopReason != "" {
			maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
		}
		if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
			maybeMarkClaudeRefusal(c, *claudeResponse.Delta.StopReason)
		}
		FormatClaudeResponseInfo(&claudeResponse, nil, claudeInfo)
	})
	if finalErr != nil {
		return nil, finalErr
	}
	if completionErr := claudeStreamCompletionError(info, claudeInfo); completionErr != nil {
		return nil, completionErr
	}
	finalResp := buildClaudeAggregatedResponse(claudeInfo)
	data, err := common.Marshal(finalResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	_, processedResp, handleErr := prepareClaudeResponseData(c, info, claudeInfo, data)
	if handleErr != nil {
		return nil, handleErr
	}
	if processedResp != nil {
		finalResp = *processedResp
	}
	if info.RelayFormat == types.RelayFormatOpenAI {
		replayClaudeAsOpenAIStream(c, info, finalResp, claudeInfo)
	} else if info.RelayFormat == types.RelayFormatOpenAIResponses {
		openaiResponse := ResponseClaude2OpenAI(&finalResp)
		openaiResponse.Usage = buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
		if emitErr := responsebridge.EmitChatResponseAsStream(c, info, openaiResponse); emitErr != nil {
			return nil, types.NewError(emitErr, types.ErrorCodeBadResponseBody)
		}
	} else {
		replayClaudeStream(c, finalResp, claudeInfo)
	}
	return claudeInfo.Usage, nil
}
