package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func maybeRunOpenAIProbe(c *gin.Context, info *relaycommon.RelayInfo, adaptor channelAdaptor, convert func(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error)) *types.NewAPIError {
	if !antiPoisonProbeDue(info) {
		return nil
	}
	probeReq := antipoison.BuildOpenAIProbeRequest(info.OriginModelName)
	probeInfo, probeCtx, err := buildAntiPoisonProbeContext(c, info, probeReq, relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	if err != nil {
		return types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
	}
	antipoison.ApplyOpenAIAnswerEnvelope(probeInfo, probeReq)
	converted, err := convert(probeCtx, probeInfo, probeReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return executeAntiPoisonProbe(probeCtx, c, probeInfo, adaptor, converted)
}

func maybeRunResponsesProbe(c *gin.Context, info *relaycommon.RelayInfo, adaptor channelAdaptor, convert func(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error)) *types.NewAPIError {
	if !antiPoisonProbeDue(info) {
		return nil
	}
	probeReq := antipoison.BuildResponsesProbeRequest(info.OriginModelName)
	probeInfo, probeCtx, err := buildAntiPoisonProbeContext(c, info, probeReq, relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses)
	if err != nil {
		return types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
	}
	antipoison.ApplyResponsesAnswerEnvelope(probeInfo, probeReq)
	converted, err := convert(probeCtx, probeInfo, *probeReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return executeAntiPoisonProbe(probeCtx, c, probeInfo, adaptor, converted)
}

func maybeRunClaudeProbe(c *gin.Context, info *relaycommon.RelayInfo, adaptor channelAdaptor, convert func(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error)) *types.NewAPIError {
	if !antiPoisonProbeDue(info) {
		return nil
	}
	probeReq := antipoison.BuildClaudeProbeRequest(info.OriginModelName)
	probeInfo, probeCtx, err := buildAntiPoisonProbeContext(c, info, probeReq, info.RelayMode, types.RelayFormatClaude)
	if err != nil {
		return types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
	}
	antipoison.ApplyClaudeAnswerEnvelope(probeInfo, probeReq)
	converted, err := convert(probeCtx, probeInfo, probeReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return executeAntiPoisonProbe(probeCtx, c, probeInfo, adaptor, converted)
}

func antiPoisonProbeDue(info *relaycommon.RelayInfo) bool {
	if !antipoison.ProbeRequired(info) {
		return false
	}
	cfg := antipoison.ResponseGuardConfig(info)
	if cfg.ProbeBeforeEveryRequest {
		return true
	}
	return cfg.StreamMode == operation_setting.AntiPoisonStreamPreflightFirstBytes &&
		!service.AntiPoisonProbeFresh(info.ChannelId, info.ChannelSetting)
}

type channelAdaptor interface {
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
}

func buildAntiPoisonProbeContext(parent *gin.Context, info *relaycommon.RelayInfo, request dto.Request, relayMode int, relayFormat types.RelayFormat) (*relaycommon.RelayInfo, *gin.Context, error) {
	if parent == nil || info == nil || info.ChannelMeta == nil {
		return nil, nil, fmt.Errorf("anti-poison probe missing relay context")
	}
	reqID := info.RequestId + "-probe"
	antipoison.MarkProbeRequest(parent, reqID)
	rec := httptest.NewRecorder()
	probeCtx, _ := gin.CreateTestContext(rec)
	method := http.MethodPost
	url := "/v1/chat/completions"
	if parent.Request != nil {
		method = parent.Request.Method
		if parent.Request.URL != nil {
			url = parent.Request.URL.String()
		}
	}
	req, _ := http.NewRequestWithContext(parent.Request.Context(), method, url, bytes.NewReader(nil))
	if parent.Request != nil {
		req.Header = parent.Request.Header.Clone()
	}
	probeCtx.Request = req
	copyRelayContext(parent, probeCtx)
	common.SetContextKey(probeCtx, common.RequestIdKey, reqID)
	probeInfo := *info
	meta := *info.ChannelMeta
	probeInfo.ChannelMeta = &meta
	probeInfo.Request = request
	probeInfo.RequestId = reqID
	probeInfo.IsStream = false
	probeInfo.IsChannelTest = true
	probeInfo.RelayMode = relayMode
	probeInfo.RelayFormat = relayFormat
	probeInfo.Billing = nil
	probeInfo.FinalPreConsumedQuota = 0
	probeInfo.DisablePing = true
	probeInfo.AntiPoisonGuardPrefix = ""
	probeInfo.AntiPoisonResponseProofNonce = ""
	probeInfo.AntiPoisonCanaryNonce = ""
	probeInfo.AntiPoisonAnswerEnvelopeNonce = ""
	probeInfo.AntiPoisonToolsDeclared = false
	probeInfo.AntiPoisonAllowedTools = nil
	return &probeInfo, probeCtx, nil
}

func executeAntiPoisonProbe(probeCtx *gin.Context, parent *gin.Context, info *relaycommon.RelayInfo, adaptor channelAdaptor, converted any) *types.NewAPIError {
	jsonData, err := common.Marshal(converted)
	if err != nil {
		return types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size
	info.UpstreamRequestGetBody = getBody
	resp, reqErr := adaptor.DoRequest(probeCtx, info, body)
	if reqErr != nil {
		service.RecordAntiPoisonProbeResult(info.ChannelId, info.ChannelSetting, false)
		return types.NewOpenAIError(reqErr, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResp, _ := resp.(*http.Response)
	if httpResp == nil {
		service.RecordAntiPoisonProbeResult(info.ChannelId, info.ChannelSetting, false)
		return types.NewError(fmt.Errorf("anti-poison probe response is nil"), types.ErrorCodeAntiPoisonValidationFailed)
	}
	if httpResp.StatusCode != http.StatusOK {
		service.RecordAntiPoisonProbeResult(info.ChannelId, info.ChannelSetting, false)
		err := service.RelayErrorHandler(probeCtx.Request.Context(), httpResp, false)
		return err
	}
	_, newAPIError := adaptor.DoResponse(probeCtx, httpResp, info)
	if newAPIError != nil {
		copyAntiPoisonProbeEvidence(probeCtx, parent)
		service.RecordAntiPoisonProbeResult(info.ChannelId, info.ChannelSetting, false)
		return newAPIError
	}
	service.RecordAntiPoisonProbeResult(info.ChannelId, info.ChannelSetting, true)
	return nil
}

func copyRelayContext(src, dst *gin.Context) {
	for _, key := range []constant.ContextKey{
		constant.ContextKeyChannelId,
		constant.ContextKeyChannelName,
		constant.ContextKeyChannelBaseUrl,
		constant.ContextKeyChannelType,
		constant.ContextKeyChannelSetting,
		constant.ContextKeyChannelOtherSetting,
		constant.ContextKeyChannelParamOverride,
		constant.ContextKeyChannelHeaderOverride,
		constant.ContextKeyChannelOrganization,
		constant.ContextKeyChannelAutoBan,
		constant.ContextKeyChannelCreateTime,
		constant.ContextKeyChannelKey,
		constant.ContextKeyOriginalModel,
	} {
		if v, ok := src.Get(string(key)); ok {
			dst.Set(string(key), v)
		}
	}
}

func copyAntiPoisonProbeEvidence(src, dst *gin.Context) {
	for _, key := range []constant.ContextKey{
		constant.ContextKeyAntiPoisonRiskLevel,
		constant.ContextKeyAntiPoisonRiskSignal,
		constant.ContextKeyAntiPoisonActionTaken,
		constant.ContextKeyAntiPoisonStreamMode,
		constant.ContextKeyAntiPoisonOpaqueScore,
		constant.ContextKeyAntiPoisonOpaqueHits,
		constant.ContextKeyAntiPoisonToolGuardResult,
		constant.ContextKeyAntiPoisonShapeResult,
		constant.ContextKeyAntiPoisonEnvelopeResult,
		constant.ContextKeyAntiPoisonProofResult,
		constant.ContextKeyAntiPoisonCanaryResult,
		constant.ContextKeyAntiPoisonEvidenceResponse,
	} {
		if v, ok := src.Get(string(key)); ok {
			dst.Set(string(key), v)
		}
	}
}
