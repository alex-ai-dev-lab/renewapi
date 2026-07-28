package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/gin-gonic/gin"
)

type testResult struct {
	context      *gin.Context
	localErr     error
	newAPIError  *types.NewAPIError
	firstByteMs  int64
	totalMs      int64
	requestBody  string
	responseBody string
	httpStatus   int
	endpoint     string
}

const channelTestNoncePrefix = "NEWAPI_TEST_"
const channelTestClaudeUserAgent = "claude-cli/2.1.177 (external, cli)"
const channelTestClaudeVersion = "2023-06-01"
const channelTestCodexUserAgent = "codex-cli_rs/0.59.0"
const channelTestCodexOriginator = "codex_cli_rs"

func newChannelTestNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return channelTestNoncePrefix + hex.EncodeToString(b[:])
	}
	return channelTestNoncePrefix + strconv.FormatInt(time.Now().UnixNano(), 16)
}

func channelTestNoncePrompt(nonce string) string {
	return "Reply with exactly this token and nothing else. Do not add quotes, markdown, spaces, or punctuation:\n" + nonce
}

func buildChannelTestHeaders(endpointType string, isStream bool) http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if isStream {
		headers.Set("Accept", "text/event-stream")
	} else {
		headers.Set("Accept", "application/json")
	}

	switch constant.EndpointType(strings.TrimSpace(endpointType)) {
	case constant.EndpointTypeOpenAIResponse:
		headers.Set("User-Agent", channelTestCodexUserAgent)
		headers.Set("Originator", channelTestCodexOriginator)
	case constant.EndpointTypeAnthropic:
		headers.Set("User-Agent", channelTestClaudeUserAgent)
		headers.Set("X-App", "cli")
		headers.Set("anthropic-version", channelTestClaudeVersion)
	}

	return headers
}

type firstByteTrackingReader struct {
	rc      io.ReadCloser
	once    sync.Once
	onFirst func()
}

func (r *firstByteTrackingReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if n > 0 {
		r.once.Do(r.onFirst)
	}
	return n, err
}

func (r *firstByteTrackingReader) Close() error {
	return r.rc.Close()
}

func sanitizeTestPayload(b []byte) string {
	const maxLen = 8 << 10
	s := strings.TrimSpace(string(b))
	if len(s) > maxLen {
		s = s[:maxLen] + "...(truncated)"
	}
	return common.MaskSensitiveInfo(s)
}

func resolveTestMaxTokens(fallback uint) uint {
	if n := operation_setting.GetChannelTestSetting().MaxTokens; n > 0 {
		return uint(n)
	}
	return fallback
}

func normalizeChannelTestEndpoint(channel *model.Channel, modelName, endpointType string) string {
	normalized := strings.TrimSpace(endpointType)
	if normalized != "" {
		return normalized
	}

	modelName = strings.TrimSpace(modelName)
	lowerModel := strings.ToLower(modelName)
	if strings.HasSuffix(lowerModel, ratio_setting.CompactModelSuffix) {
		return string(constant.EndpointTypeOpenAIResponseCompact)
	}
	if common.IsOpenAIResponseOnlyModel(lowerModel) || strings.Contains(lowerModel, "codex") {
		return string(constant.EndpointTypeOpenAIResponse)
	}
	if channel != nil {
		if channel.Type == constant.ChannelTypeCodex {
			return string(constant.EndpointTypeOpenAIResponse)
		}
		if channelTestRequiresCodexResponses(channel) {
			return string(constant.EndpointTypeOpenAIResponse)
		}
		if service.ShouldChatCompletionsUseResponsesForChannel(channel.Id, channel.Type, channel.GetBaseURL(), modelName) {
			return string(constant.EndpointTypeOpenAIResponse)
		}
		if channel.Type == constant.ChannelTypeAnthropic {
			return string(constant.EndpointTypeAnthropic)
		}
	}
	return normalized
}

func channelTestRequiresCodexResponses(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	setting := channel.GetSetting()
	if setting.RequiresCodexIdentity != nil && *setting.RequiresCodexIdentity {
		return true
	}
	return strings.Contains(strings.ToLower(setting.UserAgentOverride), "codex")
}

func endpointTypeSupportsStreaming(endpointType constant.EndpointType) bool {
	switch endpointType {
	case constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeGemini:
		return true
	default:
		return false
	}
}

func shouldUseStreamForChannelTest(channel *model.Channel, modelName, endpointType string) bool {
	normalized := normalizeChannelTestEndpoint(channel, modelName, endpointType)
	if normalized != "" {
		return endpointTypeSupportsStreaming(constant.EndpointType(normalized))
	}

	lowerModel := strings.ToLower(strings.TrimSpace(modelName))
	if strings.Contains(lowerModel, "rerank") ||
		strings.Contains(lowerModel, "embedding") ||
		strings.Contains(lowerModel, "embed") ||
		strings.HasPrefix(lowerModel, "m3e") ||
		strings.Contains(lowerModel, "bge-") {
		return false
	}
	return true
}

func resolveChannelTestUserID(c *gin.Context) (int, error) {
	if c != nil {
		if userID := c.GetInt("id"); userID > 0 {
			return userID, nil
		}
	}

	var rootUser model.User
	if err := model.DB.Select("id").Where("role = ?", common.RoleRootUser).First(&rootUser).Error; err != nil {
		return 0, fmt.Errorf("failed to resolve channel test user: %w", err)
	}
	if rootUser.Id == 0 {
		return 0, errors.New("failed to resolve channel test user")
	}
	return rootUser.Id, nil
}

func testChannel(channel *model.Channel, testUserID int, testModel string, endpointType string, isStream bool) testResult {
	tik := time.Now()
	cfg := operation_setting.GetChannelTestSetting()
	var unsupportedTestChannelTypes = []int{
		constant.ChannelTypeMidjourney,
		constant.ChannelTypeMidjourneyPlus,
		constant.ChannelTypeSunoAPI,
		constant.ChannelTypeKling,
		constant.ChannelTypeJimeng,
		constant.ChannelTypeDoubaoVideo,
		constant.ChannelTypeVidu,
	}
	if lo.Contains(unsupportedTestChannelTypes, channel.Type) {
		channelTypeName := constant.GetChannelTypeName(channel.Type)
		return testResult{
			localErr: fmt.Errorf("%s channel test is not supported", channelTypeName),
		}
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	testModel = strings.TrimSpace(testModel)
	if testModel == "" {
		if channel.TestModel != nil && *channel.TestModel != "" {
			testModel = strings.TrimSpace(*channel.TestModel)
		} else {
			models := channel.GetModels()
			if len(models) > 0 {
				testModel = strings.TrimSpace(models[0])
			}
			if testModel == "" {
				testModel = "gpt-4o-mini"
			}
		}
	}

	if strings.TrimSpace(endpointType) == "" {
		endpointType = strings.TrimSpace(cfg.EndpointType)
	}
	endpointType = normalizeChannelTestEndpoint(channel, testModel, endpointType)
	switch constant.EndpointType(endpointType) {
	case constant.EndpointTypeAudioSpeech,
		constant.EndpointTypeAudioTranscription,
		constant.EndpointTypeAudioTranslation:
		err := fmt.Errorf("%s channel test is not supported because audio tests require endpoint-specific request bodies", endpointType)
		return testResult{
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeInvalidRequest),
		}
	case constant.EndpointTypeOpenAIVideo:
		err := errors.New("openai-video channel test is not supported because video generation is asynchronous")
		return testResult{
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeInvalidRequest),
		}
	}

	switch strings.ToLower(strings.TrimSpace(cfg.StreamMode)) {
	case "on":
		isStream = true
	case "off":
		isStream = false
	default:
		// Auto mode: detect streaming from the endpoint/model. An explicit on/off
		// from settings must win and must not be overridden by auto-detection.
		if shouldUseStreamForChannelTest(channel, testModel, endpointType) {
			isStream = true
		}
	}

	requestPath := "/v1/chat/completions"

	// 如果指定了端点类型，使用指定的端点类型
	if endpointType != "" {
		if endpointInfo, ok := common.GetDefaultEndpointInfo(constant.EndpointType(endpointType)); ok {
			requestPath = endpointInfo.Path
		}
	} else {
		// 如果没有指定端点类型，使用原有的自动检测逻辑

		if strings.Contains(strings.ToLower(testModel), "rerank") {
			requestPath = "/v1/rerank"
		}

		// 先判断是否为 Embedding 模型
		if strings.Contains(strings.ToLower(testModel), "embedding") ||
			strings.HasPrefix(testModel, "m3e") || // m3e 系列模型
			strings.Contains(testModel, "bge-") || // bge 系列模型
			strings.Contains(testModel, "embed") ||
			channel.Type == constant.ChannelTypeMokaAI { // 其他 embedding 模型
			requestPath = "/v1/embeddings" // 修改请求路径
		}

		// VolcEngine 图像生成模型
		if channel.Type == constant.ChannelTypeVolcEngine && strings.Contains(testModel, "seedream") {
			requestPath = "/v1/images/generations"
		}

		// responses-only models
		if strings.Contains(strings.ToLower(testModel), "codex") {
			requestPath = "/v1/responses"
		}

		// responses compaction models (must use /v1/responses/compact)
		if strings.HasSuffix(testModel, ratio_setting.CompactModelSuffix) {
			requestPath = "/v1/responses/compact"
		}
	}
	c.Request = &http.Request{
		Method: "POST",
		URL:    &url.URL{Path: requestPath}, // 使用动态路径
		Body:   nil,
		Header: buildChannelTestHeaders(endpointType, isStream),
	}
	if cfg.TimeoutSeconds > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
	}

	cache, err := model.GetUserCache(testUserID)
	if err != nil {
		return testResult{
			localErr:    err,
			newAPIError: nil,
		}
	}
	cache.WriteContext(c)
	c.Set("id", testUserID)

	//c.Request.Header.Set("Authorization", "Bearer "+channel.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("channel", channel.Type)
	c.Set("base_url", channel.GetBaseURL())
	group, _ := model.GetUserGroup(testUserID, false)
	c.Set("group", group)

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, testModel)
	if newAPIError != nil {
		return testResult{
			context:     c,
			localErr:    newAPIError,
			newAPIError: newAPIError,
		}
	}

	// Determine relay format based on endpoint type or request path
	var relayFormat types.RelayFormat
	if endpointType != "" {
		// 根据指定的端点类型设置 relayFormat
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeOpenAI:
			relayFormat = types.RelayFormatOpenAI
		case constant.EndpointTypeOpenAIResponse:
			relayFormat = types.RelayFormatOpenAIResponses
		case constant.EndpointTypeOpenAIResponseCompact:
			relayFormat = types.RelayFormatOpenAIResponsesCompaction
		case constant.EndpointTypeAnthropic:
			relayFormat = types.RelayFormatClaude
		case constant.EndpointTypeGemini:
			relayFormat = types.RelayFormatGemini
		case constant.EndpointTypeJinaRerank:
			relayFormat = types.RelayFormatRerank
		case constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits:
			relayFormat = types.RelayFormatOpenAIImage
		case constant.EndpointTypeEmbeddings, constant.EndpointTypeModerations:
			relayFormat = types.RelayFormatEmbedding
		default:
			relayFormat = types.RelayFormatOpenAI
		}
	} else {
		// 根据请求路径自动检测
		relayFormat = types.RelayFormatOpenAI
		if c.Request.URL.Path == "/v1/embeddings" {
			relayFormat = types.RelayFormatEmbedding
		}
		if c.Request.URL.Path == "/v1/images/generations" {
			relayFormat = types.RelayFormatOpenAIImage
		}
		if c.Request.URL.Path == "/v1/messages" {
			relayFormat = types.RelayFormatClaude
		}
		if strings.Contains(c.Request.URL.Path, "/v1beta/models") {
			relayFormat = types.RelayFormatGemini
		}
		if c.Request.URL.Path == "/v1/rerank" || c.Request.URL.Path == "/rerank" {
			relayFormat = types.RelayFormatRerank
		}
		if c.Request.URL.Path == "/v1/responses" {
			relayFormat = types.RelayFormatOpenAIResponses
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") {
			relayFormat = types.RelayFormatOpenAIResponsesCompaction
		}
	}

	testNonce := ""
	if channelTestNonceEnabledForChannel(channel) && channelTestRequestSupportsNonce(endpointType, testModel, channel) {
		testNonce = newChannelTestNonce()
	}
	request := buildTestRequest(testModel, endpointType, channel, isStream, testNonce)

	info, err := relaycommon.GenRelayInfo(c, relayFormat, request, nil)

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeGenRelayInfoFailed),
		}
	}

	info.IsChannelTest = true
	info.InitChannelMeta(c)

	err = attachTestBillingRequestInput(info, request)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeJsonMarshalFailed),
		}
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeChannelModelMappedError),
		}
	}

	testModel = info.UpstreamModelName
	// 更新请求中的模型名称
	request.SetModelName(testModel)

	apiType := info.ApiType
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		apiType != constant.APITypeOpenAI &&
		apiType != constant.APITypeCodex {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("responses compaction test only supports openai/codex channels, got api type %d", apiType),
			newAPIError: types.NewError(fmt.Errorf("unsupported api type: %d", apiType), types.ErrorCodeInvalidApiType),
		}
	}
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("invalid api type: %d, adaptor is nil", apiType),
			newAPIError: types.NewError(fmt.Errorf("invalid api type: %d, adaptor is nil", apiType), types.ErrorCodeInvalidApiType),
		}
	}

	//// 创建一个用于日志的 info 副本，移除 ApiKey
	//logInfo := info
	//logInfo.ApiKey = ""
	common.SysLog(fmt.Sprintf("testing channel %d with model %s , info %+v ", channel.Id, testModel, info.ToString()))

	priceData, err := helper.ModelPriceHelper(c, info, 0, request.GetTokenCountMeta())
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest)),
		}
	}

	adaptor.Init(info)

	var convertedRequest any
	// 根据 RelayMode 选择正确的转换函数
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings, relayconstant.RelayModeModerations:
		// Embedding 请求 - request 已经是正确的类型
		if embeddingReq, ok := request.(*dto.EmbeddingRequest); ok {
			convertedRequest, err = adaptor.ConvertEmbeddingRequest(c, info, *embeddingReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid embedding request type"),
				newAPIError: types.NewError(errors.New("invalid embedding request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		// 图像生成请求 - request 已经是正确的类型
		if imageReq, ok := request.(*dto.ImageRequest); ok {
			convertedRequest, err = adaptor.ConvertImageRequest(c, info, *imageReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid image request type"),
				newAPIError: types.NewError(errors.New("invalid image request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeRerank:
		// Rerank 请求 - request 已经是正确的类型
		if rerankReq, ok := request.(*dto.RerankRequest); ok {
			convertedRequest, err = adaptor.ConvertRerankRequest(c, info.RelayMode, *rerankReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid rerank request type"),
				newAPIError: types.NewError(errors.New("invalid rerank request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponses:
		// Response 请求 - request 已经是正确的类型
		if responseReq, ok := request.(*dto.OpenAIResponsesRequest); ok {
			service.SanitizeResponsesReasoningContent(responseReq)
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *responseReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid response request type"),
				newAPIError: types.NewError(errors.New("invalid response request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponsesCompact:
		// Response compaction request - convert to OpenAIResponsesRequest before adapting
		switch req := request.(type) {
		case *dto.OpenAIResponsesCompactionRequest:
			responseReq := dto.OpenAIResponsesRequest{
				Model:              req.Model,
				Input:              req.Input,
				Instructions:       req.Instructions,
				PreviousResponseID: req.PreviousResponseID,
			}
			service.SanitizeResponsesReasoningContent(&responseReq)
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, responseReq)
		case *dto.OpenAIResponsesRequest:
			service.SanitizeResponsesReasoningContent(req)
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *req)
		default:
			return testResult{
				context:     c,
				localErr:    errors.New("invalid response compaction request type"),
				newAPIError: types.NewError(errors.New("invalid response compaction request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	default:
		if info.RelayFormat == types.RelayFormatClaude {
			if claudeReq, ok := request.(*dto.ClaudeRequest); ok {
				convertedRequest, err = adaptor.ConvertClaudeRequest(c, info, claudeReq)
				break
			}
			return testResult{
				context:     c,
				localErr:    errors.New("invalid claude request type"),
				newAPIError: types.NewError(errors.New("invalid claude request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
		// Chat/Completion 等其他请求类型
		if generalReq, ok := request.(*dto.GeneralOpenAIRequest); ok {
			convertedRequest, err = adaptor.ConvertOpenAIRequest(c, info, generalReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid general request type"),
				newAPIError: types.NewError(errors.New("invalid general request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	}

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeConvertRequestFailed),
		}
	}
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeJsonMarshalFailed),
		}
	}
	if effort := strings.TrimSpace(cfg.ReasoningEffort); effort != "" && effort != "none" {
		// Scope reasoning effort by relay format: chat completions use the
		// top-level reasoning_effort field, while the Responses API nests it under
		// reasoning.effort. Other formats (embedding/rerank/image) reject the
		// field, so they are intentionally left untouched.
		switch relayFormat {
		case types.RelayFormatOpenAI:
			if v, setErr := sjson.SetBytes(jsonData, "reasoning_effort", effort); setErr == nil {
				jsonData = v
			}
		case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
			if v, setErr := sjson.SetBytes(jsonData, "reasoning.effort", effort); setErr == nil {
				jsonData = v
			}
		}
	}
	// The Responses/Codex request body is built from a raw Input payload that
	// ignores the struct-level max tokens, so honor an explicitly configured
	// positive MaxTokens here. It is left unset by default (0) to avoid starving
	// reasoning models on these endpoints.
	if relayFormat == types.RelayFormatOpenAIResponses || relayFormat == types.RelayFormatOpenAIResponsesCompaction {
		if n := cfg.MaxTokens; n > 0 {
			if v, setErr := sjson.SetBytes(jsonData, "max_output_tokens", n); setErr == nil {
				jsonData = v
			}
		}
	}

	//jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
	//if err != nil {
	//	return testResult{
	//		context:     c,
	//		localErr:    err,
	//		newAPIError: types.NewError(err, types.ErrorCodeConvertRequestFailed),
	//	}
	//}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
				return testResult{
					context:     c,
					localErr:    fixedErr,
					newAPIError: relaycommon.NewAPIErrorFromParamOverride(fixedErr),
				}
			}
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid),
			}
		}
	}

	requestBody := bytes.NewBuffer(jsonData)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		}
	}
	var httpResp *http.Response
	responseContentTypeSSE := false
	if resp != nil {
		httpResp = resp.(*http.Response)
		if strings.HasPrefix(strings.ToLower(httpResp.Header.Get("Content-Type")), "text/event-stream") {
			responseContentTypeSSE = true
		}
		if httpResp.StatusCode != http.StatusOK {
			err := service.RelayErrorHandler(c.Request.Context(), httpResp, true)
			common.SysError(fmt.Sprintf(
				"channel test bad response: channel_id=%d name=%s type=%d model=%s endpoint_type=%s status=%d err=%v",
				channel.Id,
				channel.Name,
				channel.Type,
				testModel,
				endpointType,
				httpResp.StatusCode,
				err,
			))
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError),
				requestBody: sanitizeTestPayload(jsonData),
				httpStatus:  httpResp.StatusCode,
				endpoint:    endpointType,
			}
		}
	}
	var firstByteAt time.Time
	if httpResp != nil && httpResp.Body != nil {
		httpResp.Body = &firstByteTrackingReader{
			rc: httpResp.Body,
			onFirst: func() {
				firstByteAt = time.Now()
			},
		}
	}
	usageA, respErr := adaptor.DoResponse(c, httpResp, info)
	if respErr != nil {
		return testResult{
			context:     c,
			localErr:    respErr,
			newAPIError: respErr,
		}
	}
	result := w.Result()
	respBody, err := readTestResponseBody(result.Body, isStream || responseContentTypeSSE)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError),
		}
	}
	actualStream := isStream
	if !actualStream && responseContentTypeSSE && validateStreamTestResponseBody(respBody) == nil {
		actualStream = true
		info.IsStream = true
	}
	usage, usageErr := coerceTestUsage(usageA, actualStream, info.GetEstimatePromptTokens())
	if usageErr != nil {
		return testResult{
			context:     c,
			localErr:    usageErr,
			newAPIError: types.NewOpenAIError(usageErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	if bodyErr := validateTestResponseBody(respBody, actualStream, testNonce); bodyErr != nil {
		return testResult{
			context:     c,
			localErr:    bodyErr,
			newAPIError: types.NewOpenAIError(bodyErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	info.SetEstimatePromptTokens(usage.PromptTokens)

	quota, tieredResult := settleTestQuota(info, priceData, usage)
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	consumedTime := float64(milliseconds) / 1000.0
	other := buildTestLogOther(c, info, priceData, usage, tieredResult)
	model.RecordConsumeLog(c, testUserID, model.RecordConsumeLogParams{
		ChannelId:        channel.Id,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        info.OriginModelName,
		TokenName:        "模型测试",
		Quota:            quota,
		Content:          "模型测试",
		UseTimeSeconds:   int(consumedTime),
		IsStream:         info.IsStream,
		Group:            info.UsingGroup,
		Other:            other,
	})
	common.SysLog(fmt.Sprintf("testing channel #%d, response: \n%s", channel.Id, string(respBody)))
	firstByteMs := int64(0)
	if !firstByteAt.IsZero() {
		firstByteMs = firstByteAt.Sub(tik).Milliseconds()
	}
	httpStatus := http.StatusOK
	if httpResp != nil {
		httpStatus = httpResp.StatusCode
	}
	return testResult{
		context:      c,
		localErr:     nil,
		newAPIError:  nil,
		firstByteMs:  firstByteMs,
		totalMs:      time.Since(tik).Milliseconds(),
		requestBody:  sanitizeTestPayload(jsonData),
		responseBody: sanitizeTestPayload(respBody),
		httpStatus:   httpStatus,
		endpoint:     endpointType,
	}
}

func attachTestBillingRequestInput(info *relaycommon.RelayInfo, request dto.Request) error {
	if info == nil {
		return nil
	}

	input, err := helper.BuildBillingExprRequestInputFromRequest(request, info.RequestHeaders)
	if err != nil {
		return err
	}
	info.BillingRequestInput = &input
	return nil
}

func settleTestQuota(info *relaycommon.RelayInfo, priceData types.PriceData, usage *dto.Usage) (int, *billingexpr.TieredResult) {
	if usage != nil && info != nil && info.TieredBillingSnapshot != nil {
		isClaudeUsageSemantic := usage.UsageSemantic == "anthropic" || info.GetFinalRequestRelayFormat() == types.RelayFormatClaude
		usedVars := billingexpr.UsedVars(info.TieredBillingSnapshot.ExprString)
		if ok, quota, result := service.TryTieredSettle(info, service.BuildTieredTokenParams(usage, isClaudeUsageSemantic, usedVars)); ok {
			return quota, result
		}
	}

	quota := 0
	if !priceData.UsePrice {
		completionQuota := common.QuotaRound(float64(usage.CompletionTokens) * priceData.CompletionRatio)
		quota = common.QuotaRound(float64(usage.PromptTokens+completionQuota) * priceData.ModelRatio)
		if priceData.ModelRatio != 0 && quota <= 0 {
			quota = 1
		}
		return quota, nil
	}

	return common.QuotaFromFloat(priceData.ModelPrice * common.QuotaPerUnit), nil
}

func buildTestLogOther(c *gin.Context, info *relaycommon.RelayInfo, priceData types.PriceData, usage *dto.Usage, tieredResult *billingexpr.TieredResult) map[string]interface{} {
	other := service.GenerateTextOtherInfo(c, info, priceData.ModelRatio, priceData.GroupRatioInfo.GroupRatio, priceData.CompletionRatio,
		usage.PromptTokensDetails.CachedTokens, priceData.CacheRatio, priceData.ModelPrice, priceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredResult != nil {
		service.InjectTieredBillingInfo(other, info, tieredResult)
	}
	return other
}

func coerceTestUsage(usageAny any, isStream bool, estimatePromptTokens int) (*dto.Usage, error) {
	switch u := usageAny.(type) {
	case *dto.Usage:
		return u, nil
	case dto.Usage:
		return &u, nil
	case nil:
		if !isStream {
			return nil, errors.New("usage is nil")
		}
		usage := &dto.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	default:
		if !isStream {
			return nil, fmt.Errorf("invalid usage type: %T", usageAny)
		}
		usage := &dto.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	}
}

func readTestResponseBody(body io.ReadCloser, isStream bool) ([]byte, error) {
	defer func() { _ = body.Close() }()
	const maxStreamLogBytes = 256 << 10
	if isStream {
		return io.ReadAll(io.LimitReader(body, maxStreamLogBytes))
	}
	return io.ReadAll(body)
}

func detectErrorFromTestResponseBody(respBody []byte) error {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return nil
	}
	if message := detectErrorMessageFromJSONBytes(b); message != "" {
		return fmt.Errorf("upstream error: %s", message)
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if message := detectErrorMessageFromJSONBytes(payload); message != "" {
			return fmt.Errorf("upstream error: %s", message)
		}
	}

	return nil
}

func validateStreamTestResponseBody(respBody []byte) error {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return errors.New("stream response body is empty")
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}

		return nil
	}

	return errors.New("stream response body does not contain a valid stream event")
}

func validateTestResponseBody(respBody []byte, isStream bool, expectedNonce string) error {
	if bodyErr := detectErrorFromTestResponseBody(respBody); bodyErr != nil {
		return bodyErr
	}
	if expectedNonce != "" {
		if !responseBodyContainsTestNonce(respBody, expectedNonce) {
			return fmt.Errorf("channel test nonce mismatch: expected %s", expectedNonce)
		}
	}
	if isStream {
		return validateStreamTestResponseBody(respBody)
	}
	return nil
}

func channelTestRequestSupportsNonce(endpointType string, modelName string, channel *model.Channel) bool {
	normalized := normalizeChannelTestEndpoint(channel, modelName, endpointType)
	switch constant.EndpointType(normalized) {
	case constant.EndpointTypeEmbeddings,
		constant.EndpointTypeImageGeneration,
		constant.EndpointTypeImageEdits,
		constant.EndpointTypeJinaRerank,
		constant.EndpointTypeModerations,
		constant.EndpointTypeAudioSpeech,
		constant.EndpointTypeAudioTranscription,
		constant.EndpointTypeAudioTranslation,
		constant.EndpointTypeOpenAIVideo,
		constant.EndpointTypeOpenAIResponseCompact:
		return false
	default:
	}
	lowerModel := strings.ToLower(modelName)
	return !strings.Contains(lowerModel, "embedding") &&
		!strings.Contains(lowerModel, "rerank") &&
		!strings.Contains(lowerModel, "image")
}

func channelTestNonceEnabledForChannel(channel *model.Channel) bool {
	if !antipoison.ChannelTestNonceEnabled() {
		return false
	}
	if channel == nil {
		return true
	}
	return antipoison.FromChannelSettingsForChannel(channel.Id, channel.GetSetting()).Enabled
}

func responseBodyContainsTestNonce(respBody []byte, expectedNonce string) bool {
	if expectedNonce == "" {
		return true
	}
	raw := string(respBody)
	if strings.Contains(raw, expectedNonce) {
		return true
	}
	text := extractChannelTestResponseText(respBody)
	if strings.Contains(text, expectedNonce) {
		return true
	}
	return strings.Contains(normalizeChannelTestNonceText(text), normalizeChannelTestNonceText(expectedNonce))
}

func normalizeChannelTestNonceText(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func extractChannelTestResponseText(respBody []byte) string {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return ""
	}
	var out strings.Builder
	appendJSONTextFields := func(payload []byte) {
		for _, path := range []string{
			"choices.#.delta.content",
			"choices.#.text",
			"choices.#.message.content",
			"delta",
			"content.#.text",
			"message.content.#.text",
			"output.#.content.#.text",
			"output.#.content.#.text.value",
			"response.output_text.delta",
			"response.delta",
		} {
			result := gjson.GetBytes(payload, path)
			if !result.Exists() {
				continue
			}
			if result.IsArray() {
				result.ForEach(func(_, value gjson.Result) bool {
					out.WriteString(value.String())
					return true
				})
				continue
			}
			out.WriteString(result.String())
		}
	}

	if len(b) > 0 && (b[0] == '{' || b[0] == '[') {
		appendJSONTextFields(b)
		return out.String()
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		appendJSONTextFields(payload)
	}
	return out.String()
}

func shouldUseStreamForAutomaticChannelTest(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	modelName := ""
	if channel.TestModel != nil {
		modelName = strings.TrimSpace(*channel.TestModel)
	}
	if modelName == "" {
		models := channel.GetModels()
		if len(models) > 0 {
			modelName = strings.TrimSpace(models[0])
		}
	}
	return shouldUseStreamForChannelTest(channel, modelName, "")
}

func detectErrorMessageFromJSONBytes(jsonBytes []byte) string {
	if len(jsonBytes) == 0 {
		return ""
	}
	if jsonBytes[0] != '{' && jsonBytes[0] != '[' {
		return ""
	}
	errVal := gjson.GetBytes(jsonBytes, "error")
	if !errVal.Exists() || errVal.Type == gjson.Null {
		return ""
	}

	message := gjson.GetBytes(jsonBytes, "error.message").String()
	if message == "" {
		message = gjson.GetBytes(jsonBytes, "error.error.message").String()
	}
	if message == "" && errVal.Type == gjson.String {
		message = errVal.String()
	}
	if message == "" {
		message = errVal.Raw
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "upstream returned error payload"
	}
	return message
}

func buildTestRequest(model string, endpointType string, channel *model.Channel, isStream bool, nonce string) dto.Request {
	cfg := operation_setting.GetChannelTestSetting()
	prompt := strings.TrimSpace(cfg.Prompt)
	if prompt == "" {
		prompt = "hi"
	}
	if nonce != "" {
		prompt = channelTestNoncePrompt(nonce)
	}
	// Build the Responses/Codex input from the same prompt (which already carries
	// the anti-poison nonce when enabled) instead of a hardcoded "hi", so these
	// endpoints honor the configured probe prompt and pass nonce validation.
	testResponsesInput := json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`)
	if b, err := json.Marshal([]map[string]any{{
		"role": "user",
		"content": []map[string]any{{
			"type": "input_text",
			"text": prompt,
		}},
	}}); err == nil {
		testResponsesInput = b
	}

	// 根据端点类型构建不同的测试请求
	if endpointType != "" {
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeEmbeddings:
			// 返回 EmbeddingRequest
			return &dto.EmbeddingRequest{
				Model: model,
				Input: []any{"hello world"},
			}
		case constant.EndpointTypeModerations:
			return &dto.EmbeddingRequest{
				Model: model,
				Input: []any{"hello world"},
			}
		case constant.EndpointTypeImageGeneration, constant.EndpointTypeImageEdits:
			// 返回 ImageRequest
			return &dto.ImageRequest{
				Model:  model,
				Prompt: "a cute cat",
				N:      lo.ToPtr(uint(1)),
				Size:   "1024x1024",
			}
		case constant.EndpointTypeJinaRerank:
			// 返回 RerankRequest
			return &dto.RerankRequest{
				Model:     model,
				Query:     "What is Deep Learning?",
				Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
				TopN:      lo.ToPtr(2),
			}
		case constant.EndpointTypeOpenAIResponse:
			// 返回 OpenAIResponsesRequest
			return &dto.OpenAIResponsesRequest{
				Model:          model,
				Input:          testResponsesInput,
				Store:          json.RawMessage(`false`),
				PromptCacheKey: json.RawMessage(`"newapi-channel-test"`),
				Stream:         lo.ToPtr(isStream),
			}
		case constant.EndpointTypeOpenAIResponseCompact:
			// 返回 OpenAIResponsesCompactionRequest
			return &dto.OpenAIResponsesCompactionRequest{
				Model: model,
				Input: testResponsesInput,
			}
		case constant.EndpointTypeAnthropic:
			return &dto.ClaudeRequest{
				Model:  model,
				Stream: lo.ToPtr(isStream),
				Messages: []dto.ClaudeMessage{
					{
						Role:    "user",
						Content: prompt,
					},
				},
				MaxTokens: lo.ToPtr(resolveTestMaxTokens(16)),
			}
		case constant.EndpointTypeGemini, constant.EndpointTypeOpenAI:
			// 返回 GeneralOpenAIRequest
			maxTokens := resolveTestMaxTokens(16)
			if constant.EndpointType(endpointType) == constant.EndpointTypeGemini {
				maxTokens = resolveTestMaxTokens(3000)
			}
			req := &dto.GeneralOpenAIRequest{
				Model:  model,
				Stream: lo.ToPtr(isStream),
				Messages: []dto.Message{
					{
						Role:    "user",
						Content: prompt,
					},
				},
				MaxTokens: lo.ToPtr(maxTokens),
			}
			if isStream {
				req.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
			}
			return req
		}
	}

	// 自动检测逻辑（保持原有行为）
	if strings.Contains(strings.ToLower(model), "rerank") {
		return &dto.RerankRequest{
			Model:     model,
			Query:     "What is Deep Learning?",
			Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
			TopN:      lo.ToPtr(2),
		}
	}

	// 先判断是否为 Embedding 模型
	if strings.Contains(strings.ToLower(model), "embedding") ||
		strings.HasPrefix(model, "m3e") ||
		strings.Contains(model, "bge-") {
		// 返回 EmbeddingRequest
		return &dto.EmbeddingRequest{
			Model: model,
			Input: []any{"hello world"},
		}
	}

	// Responses compaction models (must use /v1/responses/compact)
	if strings.HasSuffix(model, ratio_setting.CompactModelSuffix) {
		return &dto.OpenAIResponsesCompactionRequest{
			Model: model,
			Input: testResponsesInput,
		}
	}

	// Responses-only models (e.g. codex series)
	if strings.Contains(strings.ToLower(model), "codex") {
		return &dto.OpenAIResponsesRequest{
			Model:          model,
			Input:          testResponsesInput,
			Store:          json.RawMessage(`false`),
			PromptCacheKey: json.RawMessage(`"newapi-channel-test"`),
			Stream:         lo.ToPtr(isStream),
		}
	}

	// Chat/Completion 请求 - 返回 GeneralOpenAIRequest
	testRequest := &dto.GeneralOpenAIRequest{
		Model:  model,
		Stream: lo.ToPtr(isStream),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
	if isStream {
		testRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}

	if strings.HasPrefix(model, "o") {
		testRequest.MaxCompletionTokens = lo.ToPtr(resolveTestMaxTokens(16))
	} else if strings.Contains(model, "thinking") {
		if !strings.Contains(model, "claude") {
			testRequest.MaxTokens = lo.ToPtr(resolveTestMaxTokens(50))
		}
	} else if strings.Contains(model, "gemini") {
		testRequest.MaxTokens = lo.ToPtr(resolveTestMaxTokens(3000))
	} else {
		testRequest.MaxTokens = lo.ToPtr(resolveTestMaxTokens(16))
	}

	return testRequest
}

func TestChannel(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(channelId)
	if err != nil {
		channel, err = model.GetChannelById(channelId, true)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	//defer func() {
	//	if channel.ChannelInfo.IsMultiKey {
	//		go func() { _ = channel.SaveChannelInfo() }()
	//	}
	//}()
	testModel := c.Query("model")
	endpointType := c.Query("endpoint_type")
	isStream, _ := strconv.ParseBool(c.Query("stream"))
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tik := time.Now()
	result := testChannel(channel, testUserID, testModel, endpointType, isStream)
	resultEndpoint := endpointType
	if result.endpoint != "" {
		resultEndpoint = result.endpoint
	}
	if result.localErr != nil {
		service.RecordChannelModelFailure(service.ChannelModelFailureParams{
			ChannelId: channel.Id,
			Group:     common.GetContextKeyString(result.context, constant.ContextKeyUsingGroup),
			ModelName: common.GetContextKeyString(result.context, constant.ContextKeyOriginalModel),
			Endpoint:  resultEndpoint,
			RequestId: common.GetContextKeyString(result.context, common.RequestIdKey),
			Error:     result.newAPIError,
			AutoBan:   channel.GetAutoBan(),
		})
		resp := gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
		}
		if result.newAPIError != nil {
			resp["error_code"] = result.newAPIError.GetErrorCode()
		}
		if result.httpStatus != 0 {
			resp["http_status"] = result.httpStatus
		}
		if result.endpoint != "" {
			resp["endpoint_type"] = result.endpoint
		}
		if result.requestBody != "" {
			resp["request"] = result.requestBody
		}
		if result.responseBody != "" {
			resp["response"] = result.responseBody
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if result.newAPIError != nil {
		service.RecordChannelModelFailure(service.ChannelModelFailureParams{
			ChannelId: channel.Id,
			Group:     common.GetContextKeyString(result.context, constant.ContextKeyUsingGroup),
			ModelName: common.GetContextKeyString(result.context, constant.ContextKeyOriginalModel),
			Endpoint:  resultEndpoint,
			RequestId: common.GetContextKeyString(result.context, common.RequestIdKey),
			Error:     result.newAPIError,
			AutoBan:   channel.GetAutoBan(),
		})
		resp := gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       consumedTime,
			"error_code": result.newAPIError.GetErrorCode(),
		}
		if result.httpStatus != 0 {
			resp["http_status"] = result.httpStatus
		}
		if result.endpoint != "" {
			resp["endpoint_type"] = result.endpoint
		}
		if result.requestBody != "" {
			resp["request"] = result.requestBody
		}
		if result.responseBody != "" {
			resp["response"] = result.responseBody
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	service.RecordChannelModelSuccess(channel.Id, common.GetContextKeyString(result.context, constant.ContextKeyUsingGroup), common.GetContextKeyString(result.context, constant.ContextKeyOriginalModel), resultEndpoint, common.GetContextKeyString(result.context, common.RequestIdKey))
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "",
		"time":            consumedTime,
		"total_time":      float64(result.totalMs) / 1000.0,
		"first_byte_time": float64(result.firstByteMs) / 1000.0,
		"endpoint_type":   result.endpoint,
		"http_status":     result.httpStatus,
		"request":         result.requestBody,
		"response":        result.responseBody,
	})
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func testAllChannels(notify bool) error {
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return err
	}

	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		testAllChannelsLock.Unlock()
		return errors.New("测试已在运行中")
	}
	testAllChannelsRunning = true
	testAllChannelsLock.Unlock()
	channels, getChannelErr := model.GetAllChannels(0, 0, true, false)
	if getChannelErr != nil {
		return getChannelErr
	}
	var disableThreshold = int64(common.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	gopool.Go(func() {
		// 使用 defer 确保无论如何都会重置运行状态，防止死锁
		defer func() {
			testAllChannelsLock.Lock()
			testAllChannelsRunning = false
			testAllChannelsLock.Unlock()
		}()

		for _, channel := range channels {
			if channel.Status != common.ChannelStatusEnabled &&
				channel.Status != common.ChannelStatusAutoDisabled {
				continue
			}
			if !channel.AllowAutoTestAndRecover() {
				continue
			}
			isChannelEnabled := channel.Status == common.ChannelStatusEnabled
			tik := time.Now()
			result := testChannel(channel, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(channel))
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()

			shouldBanChannel := false
			newAPIError := result.newAPIError
			if newAPIError == nil {
				service.RecordChannelModelSuccess(channel.Id, common.GetContextKeyString(result.context, constant.ContextKeyUsingGroup), common.GetContextKeyString(result.context, constant.ContextKeyOriginalModel), "", common.GetContextKeyString(result.context, common.RequestIdKey))
			} else {
				service.RecordChannelModelFailure(service.ChannelModelFailureParams{
					ChannelId: channel.Id,
					Group:     common.GetContextKeyString(result.context, constant.ContextKeyUsingGroup),
					ModelName: common.GetContextKeyString(result.context, constant.ContextKeyOriginalModel),
					RequestId: common.GetContextKeyString(result.context, common.RequestIdKey),
					Error:     newAPIError,
					AutoBan:   channel.GetAutoBan(),
				})
			}
			// request error disables the channel
			if newAPIError != nil {
				shouldBanChannel = service.IsAntiPoisonValidationError(result.newAPIError) || service.ShouldDisableChannel(result.newAPIError)
			}

			// 当错误检查通过，才检查响应时间
			if common.AutomaticDisableChannelEnabled && !shouldBanChannel {
				if milliseconds > disableThreshold {
					err := fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
					newAPIError = types.NewOpenAIError(err, types.ErrorCodeChannelResponseTimeExceeded, http.StatusRequestTimeout)
					shouldBanChannel = true
				}
			}

			// disable channel
			if isChannelEnabled && shouldBanChannel && (channel.GetAutoBan() || service.IsAntiPoisonValidationError(newAPIError)) {
				processChannelError(result.context, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(result.context, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)
			}

			// enable channel
			if !isChannelEnabled && service.ShouldEnableChannel(newAPIError, channel.Status) {
				service.EnableChannel(channel.Id, common.GetContextKeyString(result.context, constant.ContextKeyChannelKey), channel.Name)
			}

			channel.UpdateResponseTime(milliseconds)
			time.Sleep(common.RequestInterval)
		}

		if notify {
			service.NotifyRootUser(dto.NotifyTypeChannelTest, "通道测试完成", "所有通道测试已完成")
		}
	})
	return nil
}

func TestAllChannels(c *gin.Context) {
	err := testAllChannels(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

var autoTestChannelsOnce sync.Once

func AutomaticallyTestChannels() {
	// 只在Master节点定时测试渠道
	if !common.IsMasterNode {
		return
	}
	autoTestChannelsOnce.Do(func() {
		// Per-channel independent scheduling (interval / retry / retry-threshold /
		// time-window). See channel_test_scheduler.go.
		startIndependentAutoTest()
	})
}
