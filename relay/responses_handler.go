package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	antipoison.AuditSignedHeaders(c, info)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	responsesReq, err := normalizeResponsesRequest(info.Request)
	if err != nil {
		return types.NewErrorWithStatusCode(
			err,
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	isCompactionRelated := info.ResponsesRequestKind != dto.ResponsesNormal

	if endpoint, ok := service.ShouldUseModelDefaultTextEndpointForResponses(info); ok {
		if info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("Responses compact cannot be bridged to %s", endpoint),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if capabilityErr := service.ValidateResponsesTextBridgeRequest(request); capabilityErr != nil {
			return types.NewErrorWithStatusCode(capabilityErr, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("model default endpoint rewrite to %s requires request conversion, but pass-through body is enabled", endpoint),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		chatReq, convErr := service.ResponsesRequestToChatCompletionsRequestForContext(c, request)
		if convErr != nil {
			return types.NewErrorWithStatusCode(convErr, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		chatInfo := *info
		chatInfo.Request = chatReq
		chatInfo.RelayMode = relayconstant.RelayModeChatCompletions
		chatInfo.RequestURLPath = "/v1/chat/completions"
		chatInfo.FinalRequestRelayFormat = ""
		chatInfo.RequestConversionChain = append(append([]types.RelayFormat(nil), info.RequestConversionChain...), types.RelayFormatOpenAI)
		logger.LogInfo(c, fmt.Sprintf(
			"channel #%d protocol bridge: client_format=%s upstream_endpoint=%s bridge=responses->%s lossy=false model=%s",
			info.ChannelId,
			info.RelayFormat,
			endpoint,
			endpoint,
			info.OriginModelName,
		))
		return TextHelper(c, &chatInfo)
	}

	if info.ChannelType == appconstant.ChannelTypeCodex && !info.IsStream && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		request.Stream = common.GetPointer(true)
		c.Set("responses_upstream_stream", true)
	}
	if !isCompactionRelated {
		antipoison.ApplyResponsesResponseProof(info, request)
		antipoison.ApplyResponsesAnswerEnvelope(info, request)
		antipoison.ApplyResponsesRequestGuard(info, request)
		antipoison.ApplyResponsesCanaryRequest(info, request)
	}
	antipoison.CaptureResponsesToolPolicy(info, request)
	if !isCompactionRelated {
		if result := service.SanitizeResponsesReasoningContent(request); result.Changed {
			logger.LogInfo(c, fmt.Sprintf(
				"responses reasoning content sanitized: removed_reasoning_content=%d",
				result.RemovedReasoningContent,
			))
		}
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	argsFormat := service.EffectiveResponsesFunctionCallArgumentsFormat(
		info.ChannelType,
		info.ChannelSetting,
		info.ForceResponsesFunctionCallArgumentsObject,
	)
	enforceArgsFormat := service.ShouldEnforceResponsesFunctionCallArgumentsFormat(
		info.ChannelType,
		info.ChannelSetting,
		info.ForceResponsesFunctionCallArgumentsObject,
	)
	passThroughEnabled := (model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled) && !enforceArgsFormat
	if isCompactionRelated {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		rawBody, err := storage.Bytes()
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		record := service.EffectiveResponsesCompactionRecordByID(info.ChannelId, info.OriginModelName, info.ChannelSetting)
		plan, err := service.PlanResponsesExecution(info.ResponsesRequestKind, record, request.Model, info.IsStream)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		jsonData, err := service.BuildResponsesCompactionRequestBody(rawBody, plan)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if plan.UpstreamPath == "/v1/responses/compact" {
			info.RelayMode = relayconstant.RelayModeResponsesCompact
			c.Set("responses_legacy_bridge_sse", plan.BridgeJSONToSSE)
		} else {
			// RelayInfo is reused across retries. A previous legacy candidate may
			// have switched the mode to compact, so restore the native mode before
			// trying a native-capable channel.
			info.RelayMode = relayconstant.RelayModeResponses
			c.Set("responses_legacy_bridge_sse", false)
		}
		c.Set("responses_upstream_stream", plan.UpstreamStream)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	} else if passThroughEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		if strings.TrimSpace(request.Model) != "" && request.Model != info.UpstreamModelName && strings.TrimSpace(info.UpstreamModelName) != "" {
			rawBody, err := storage.Bytes()
			if err != nil {
				return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
			}
			jsonData, err := rewriteResponsesPassThroughModel(rawBody, info.UpstreamModelName)
			if err != nil {
				return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			defer closer.Close()
			info.UpstreamRequestBodySize = size
			requestBody = body
		} else {
			requestBody = common.ReaderOnly(storage)
		}
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		convertedRequest, _, err = service.NormalizeResponsesFunctionCallArgumentsPayload(convertedRequest, argsFormat)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, passThroughEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		logger.LogDebug(c, "requestBody: %s", jsonData)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		jsonData = nil
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	if !passThroughEnabled && !isCompactionRelated {
		if probeErr := maybeRunResponsesProbe(c, info, adaptor,
			func(pc *gin.Context, pi *relaycommon.RelayInfo, req dto.OpenAIResponsesRequest) (any, error) {
				return adaptor.ConvertOpenAIResponsesRequest(pc, pi, req)
			}); probeErr != nil {
			antipoison.RecordRisk(c, antipoison.RiskSuspicious, "probe_failed", "block")
			return probeErr
		}
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	successChannelID := info.ChannelId
	if selectedChannelID := common.GetContextKeyInt(c, appconstant.ContextKeyChannelId); selectedChannelID > 0 {
		successChannelID = selectedChannelID
	}
	service.ObserveSessionRecoverySuccess(c, successChannelID)

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}

func normalizeResponsesRequest(input interface{}) (*dto.OpenAIResponsesRequest, error) {
	switch req := input.(type) {
	case *dto.OpenAIResponsesRequest:
		return req, nil
	case *dto.OpenAIResponsesCompactionRequest:
		return &dto.OpenAIResponsesRequest{
			Model:                req.Model,
			Input:                req.Input,
			Instructions:         req.Instructions,
			PreviousResponseID:   req.PreviousResponseID,
			ParallelToolCalls:    req.ParallelToolCalls,
			ServiceTier:          req.ServiceTier,
			PromptCacheKey:       req.PromptCacheKey,
			PromptCacheOptions:   req.PromptCacheOptions,
			PromptCacheRetention: req.PromptCacheRetention,
		}, nil
	default:
		return nil, fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", input)
	}
}

func rewriteResponsesPassThroughModel(body []byte, model string) ([]byte, error) {
	return sjson.SetBytes(body, "model", model)
}
