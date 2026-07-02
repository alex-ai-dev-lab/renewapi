package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type ModelRequest struct {
	Model          string                         `json:"model"`
	Group          string                         `json:"group,omitempty"`
	Models         []string                       `json:"-"`
	ProviderPolicy *service.ProviderRoutingPolicy `json:"-"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		var channel *model.Channel
		channelId, ok := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId)
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
			return
		}
		if modelRequest.ProviderPolicy != nil && !modelRequest.ProviderPolicy.Empty() {
			common.SetContextKey(c, constant.ContextKeyProviderRoutingPolicy, modelRequest.ProviderPolicy)
		}
		if len(modelRequest.Models) > 1 {
			common.SetContextKey(c, constant.ContextKeyFallbackModels, modelRequest.Models)
		}
		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			channel, err = model.GetChannelById(id, true)
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			if channel.Status != common.ChannelStatusEnabled {
				abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorChannelDisabled))
				return
			}
			if !service.ChannelAllowedForProduction(channel) {
				abortWithOpenAiMessage(c, http.StatusForbidden, "channel is quarantined by anti-poison profile")
				return
			}
		} else {
			// Select a channel for the user
			// check token model mapping
			modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
			if modelLimitEnable {
				s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
				if !ok {
					// token model limit is empty, all models are not allowed
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenNoModelAccess))
					return
				}
				var tokenModelLimit map[string]bool
				tokenModelLimit, ok = s.(map[string]bool)
				if !ok {
					tokenModelLimit = map[string]bool{}
				}
				matchName := ratio_setting.FormatMatchingModelName(modelRequest.Model) // match gpts & thinking-*
				if _, ok := tokenModelLimit[matchName]; !ok {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model}))
					return
				}
			}

			if shouldSelectChannel {
				if modelRequest.Model == "" {
					abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorModelNameRequired))
					return
				}
				var selectGroup string
				usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
				// check path is /pg/chat/completions
				if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
					playgroundRequest := &dto.PlayGroundRequest{}
					err = common.UnmarshalBodyReusable(c, playgroundRequest)
					if err != nil {
						abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidPlayground, map[string]any{"Error": err.Error()}))
						return
					}
					if playgroundRequest.Group != "" {
						if !service.GroupInUserUsableGroups(usingGroup, playgroundRequest.Group) && playgroundRequest.Group != usingGroup {
							abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied))
							return
						}
						usingGroup = playgroundRequest.Group
						common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
					}
				}

				if preferredChannelID, found := service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
					preferred, err := model.CacheGetChannel(preferredChannelID)
					if err == nil && preferred != nil {
						if preferred.Status != common.ChannelStatusEnabled {
							if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
								abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorAffinityChannelDisabled))
								return
							}
						} else if !service.ChannelMatchesProviderRoutingPolicy(preferred, modelRequest.ProviderPolicy) {
							loggerMsg := fmt.Sprintf("affinity channel skipped by provider routing policy: channel=%d", preferred.Id)
							common.SysLog(loggerMsg)
						} else if usingGroup == "auto" {
							userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
							autoGroups := service.GetUserAutoGroup(userGroup)
							for _, g := range autoGroups {
								if model.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) &&
									!model.IsChannelModelDisabledForGroup(preferred.Id, g, modelRequest.Model) {
									if channelAffinityFallbackOnly() && higherPriorityChannelAvailable(g, modelRequest.Model, preferred, modelRequest.ProviderPolicy) {
										common.SysLog(fmt.Sprintf("affinity channel deferred: higher priority channel available for group=%s model=%s affinity_channel=%d", g, modelRequest.Model, preferred.Id))
									} else {
										selectGroup = g
										common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
										channel = preferred
										service.MarkChannelAffinityUsed(c, g, preferred.Id)
									}
									break
								}
							}
						} else if model.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) &&
							!model.IsChannelModelDisabledForGroup(preferred.Id, usingGroup, modelRequest.Model) {
							if channelAffinityFallbackOnly() && higherPriorityChannelAvailable(usingGroup, modelRequest.Model, preferred, modelRequest.ProviderPolicy) {
								common.SysLog(fmt.Sprintf("affinity channel deferred: higher priority channel available for group=%s model=%s affinity_channel=%d", usingGroup, modelRequest.Model, preferred.Id))
							} else {
								channel = preferred
								selectGroup = usingGroup
								service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
							}
						}
					}
				}

				if channel == nil {
					channel, selectGroup, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
						Ctx:                   c,
						ModelName:             modelRequest.Model,
						TokenGroup:            usingGroup,
						Retry:                 common.GetPointer(0),
						ProviderRoutingPolicy: modelRequest.ProviderPolicy,
					})
					if err != nil {
						showGroup := usingGroup
						if usingGroup == "auto" {
							showGroup = fmt.Sprintf("auto(%s)", selectGroup)
						}
						message := i18n.T(c, i18n.MsgDistributorGetChannelFailed, map[string]any{"Group": showGroup, "Model": modelRequest.Model, "Error": err.Error()})
						// 如果错误，但是渠道不为空，说明是数据库一致性问题
						//if channel != nil {
						//	common.SysError(fmt.Sprintf("渠道不存在：%d", channel.Id))
						//	message = "数据库一致性已被破坏，请联系管理员"
						//}
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, message, types.ErrorCodeModelNotFound)
						return
					}
					if channel == nil {
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": usingGroup, "Model": modelRequest.Model}), types.ErrorCodeModelNotFound)
						return
					}
				}
			}
		}
		common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
		SetupContextForSelectedChannel(c, channel, modelRequest.Model)
		c.Next()
		if channel != nil && c.Writer != nil && c.Writer.Status() < http.StatusBadRequest {
			service.RecordChannelAffinity(c, channel.Id)
		}
	}
}

// channelAffinityFallbackOnly reports whether channel affinity should only be used
// as a fallback. When enabled (default), an affinity-preferred channel is only used
// if no strictly higher priority channel is currently available for the group+model.
// This prevents a session from being stuck on a lower priority channel due to cache
// affinity once a higher priority channel becomes healthy again.
func channelAffinityFallbackOnly() bool {
	return common.GetEnvOrDefaultBool("CHANNEL_AFFINITY_FALLBACK_ONLY", true)
}

// higherPriorityChannelAvailable reports whether a strictly higher priority channel
// than the affinity channel is currently selectable for the given group+model.
//
// It reuses the in-memory cache selection path (GetRandomSatisfiedChannelExcludingWithPolicy),
// the same one Distribute uses to pick the actual channel. This keeps the fallback
// decision consistent with the channel that would really be selected and avoids
// issuing synchronous ability/channel DB queries on every affinity-preferred request.
func higherPriorityChannelAvailable(group, modelName string, affinityChannel *model.Channel, policy *service.ProviderRoutingPolicy) bool {
	if affinityChannel == nil {
		return false
	}
	var routingPolicy model.ChannelRoutingPolicy
	if policy != nil && !policy.Empty() {
		routingPolicy = policy
	}
	topChannel, err := model.GetRandomSatisfiedChannelExcludingWithPolicy(group, modelName, 0, nil, routingPolicy)
	if err != nil || topChannel == nil {
		return false
	}
	return topChannel.GetPriority() > affinityChannel.GetPriority()
}

// getModelFromRequest 从请求中读取模型信息
// 根据 Content-Type 自动处理：
// - application/json
// - application/x-www-form-urlencoded
// - multipart/form-data
func getModelFromRequest(c *gin.Context) (*ModelRequest, error) {
	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") {
		modelRequest, err := getModelFromJSONBody(c)
		if err != nil {
			return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		}
		return modelRequest, nil
	}

	var modelRequest ModelRequest
	err := common.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
	}
	return &modelRequest, nil
}

func getModelFromJSONBody(c *gin.Context) (*ModelRequest, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	requestBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(requestBody) {
		return nil, errors.New("invalid JSON request body")
	}

	values := gjson.GetManyBytes(requestBody, "model", "group")
	model, err := getJSONStringValue(values[0], "model")
	if err != nil {
		return nil, err
	}
	group, err := getJSONStringValue(values[1], "group")
	if err != nil {
		return nil, err
	}

	if _, seekErr := storage.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}
	c.Request.Body = io.NopCloser(storage)

	return &ModelRequest{
		Model:          model,
		Group:          group,
		Models:         parseFallbackModelsFromJSON(requestBody, model),
		ProviderPolicy: parseProviderRoutingPolicyFromJSON(requestBody),
	}, nil
}

func parseFallbackModelsFromJSON(requestBody []byte, primaryModel string) []string {
	if !common.GetEnvOrDefaultBool("REQUEST_MODELS_FALLBACK_ENABLED", false) {
		return nil
	}
	result := gjson.GetBytes(requestBody, "models")
	if !result.Exists() || !result.IsArray() {
		return nil
	}
	models := make([]string, 0, len(result.Array())+1)
	if primaryModel != "" {
		models = append(models, primaryModel)
	}
	for _, item := range result.Array() {
		if item.Type != gjson.String {
			continue
		}
		model := strings.TrimSpace(item.String())
		if model != "" {
			models = append(models, model)
		}
	}
	models = dedupeProviderSelectors(models)
	if len(models) < 2 {
		return nil
	}
	maxModels := common.GetEnvOrDefault("REQUEST_MODELS_FALLBACK_MAX", 4)
	if maxModels <= 0 {
		return nil
	}
	if len(models) > maxModels {
		models = models[:maxModels]
	}
	return models
}

func parseProviderRoutingPolicyFromJSON(requestBody []byte) *service.ProviderRoutingPolicy {
	if !common.GetEnvOrDefaultBool("PROVIDER_ROUTING_CONTROL_ENABLED", false) {
		return nil
	}
	policy := &service.ProviderRoutingPolicy{
		Only:   readProviderRoutingSelectors(requestBody, "provider.only", "only"),
		Ignore: readProviderRoutingSelectors(requestBody, "provider.ignore", "ignore"),
		Order:  readProviderRoutingSelectors(requestBody, "provider.order", "order"),
	}
	if policy.Empty() {
		return nil
	}
	return policy
}

func readProviderRoutingSelectors(requestBody []byte, paths ...string) []string {
	var selectors []string
	for _, path := range paths {
		result := gjson.GetBytes(requestBody, path)
		if !result.Exists() || result.Type == gjson.Null {
			continue
		}
		switch result.Type {
		case gjson.String:
			selectors = appendProviderSelectorCSV(selectors, result.String())
		case gjson.JSON:
			if result.IsArray() {
				for _, item := range result.Array() {
					if item.Type == gjson.String {
						selectors = appendProviderSelectorCSV(selectors, item.String())
					}
				}
			}
		}
	}
	return dedupeProviderSelectors(selectors)
}

func appendProviderSelectorCSV(selectors []string, value string) []string {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			selectors = append(selectors, part)
		}
	}
	return selectors
}

func dedupeProviderSelectors(selectors []string) []string {
	if len(selectors) < 2 {
		return selectors
	}
	seen := make(map[string]struct{}, len(selectors))
	deduped := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		key := strings.ToLower(strings.TrimSpace(selector))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, selector)
	}
	return deduped
}

func getJSONStringValue(result gjson.Result, field string) (string, error) {
	if !result.Exists() || result.Type == gjson.Null {
		return "", nil
	}
	if result.Type != gjson.String {
		return "", fmt.Errorf("field %s must be a string", field)
	}
	return result.String(), nil
}

func getModelRequest(c *gin.Context) (*ModelRequest, bool, error) {
	var modelRequest ModelRequest
	shouldSelectChannel := true
	var err error
	if strings.Contains(c.Request.URL.Path, "/mj/") {
		relayMode := relayconstant.Path2RelayModeMidjourney(c.Request.URL.Path)
		if relayMode == relayconstant.RelayModeMidjourneyTaskFetch ||
			relayMode == relayconstant.RelayModeMidjourneyTaskFetchByCondition ||
			relayMode == relayconstant.RelayModeMidjourneyNotify ||
			relayMode == relayconstant.RelayModeMidjourneyTaskImageSeed {
			shouldSelectChannel = false
		} else {
			midjourneyRequest := dto.MidjourneyRequest{}
			err = common.UnmarshalBodyReusable(c, &midjourneyRequest)
			if err != nil {
				return nil, false, errors.New(i18n.T(c, i18n.MsgDistributorInvalidMidjourney, map[string]any{"Error": err.Error()}))
			}
			midjourneyModel, mjErr, success := service.GetMjRequestModel(relayMode, &midjourneyRequest)
			if mjErr != nil {
				return nil, false, fmt.Errorf("%s", mjErr.Description)
			}
			if midjourneyModel == "" {
				if !success {
					return nil, false, fmt.Errorf("%s", i18n.T(c, i18n.MsgDistributorInvalidParseModel))
				} else {
					// task fetch, task fetch by condition, notify
					shouldSelectChannel = false
				}
			}
			modelRequest.Model = midjourneyModel
		}
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/suno/") {
		relayMode := relayconstant.Path2RelaySuno(c.Request.Method, c.Request.URL.Path)
		if relayMode == relayconstant.RelayModeSunoFetch ||
			relayMode == relayconstant.RelayModeSunoFetchByID {
			shouldSelectChannel = false
		} else {
			modelName := service.CoverTaskActionToModelName(constant.TaskPlatformSuno, c.Param("action"))
			modelRequest.Model = modelName
		}
		c.Set("platform", string(constant.TaskPlatformSuno))
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/v1/videos/") && strings.HasSuffix(c.Request.URL.Path, "/remix") {
		relayMode := relayconstant.RelayModeVideoSubmit
		c.Set("relay_mode", relayMode)
		shouldSelectChannel = false
	} else if strings.Contains(c.Request.URL.Path, "/v1/videos") {
		//curl https://api.openai.com/v1/videos \
		//  -H "Authorization: Bearer $OPENAI_API_KEY" \
		//  -F "model=sora-2" \
		//  -F "prompt=A calico cat playing a piano on stage"
		//	-F input_reference="@image.jpg"
		relayMode := relayconstant.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			relayMode = relayconstant.RelayModeVideoSubmit
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			if req != nil {
				modelRequest.Model = req.Model
				modelRequest.Models = req.Models
				modelRequest.ProviderPolicy = req.ProviderPolicy
			}
		} else if c.Request.Method == http.MethodGet {
			relayMode = relayconstant.RelayModeVideoFetchByID
			shouldSelectChannel = false
		}
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/v1/video/generations") {
		relayMode := relayconstant.RelayModeUnknown
		if c.Request.Method == http.MethodPost {
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			modelRequest.Model = req.Model
			modelRequest.Models = req.Models
			modelRequest.ProviderPolicy = req.ProviderPolicy
			relayMode = relayconstant.RelayModeVideoSubmit
		} else if c.Request.Method == http.MethodGet {
			relayMode = relayconstant.RelayModeVideoFetchByID
			shouldSelectChannel = false
		}
		if _, ok := c.Get("relay_mode"); !ok {
			c.Set("relay_mode", relayMode)
		}
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
		// Gemini API 路径处理: /v1beta/models/gemini-2.0-flash:generateContent
		relayMode := relayconstant.RelayModeGemini
		modelName := extractModelNameFromGeminiPath(c.Request.URL.Path)
		if modelName != "" {
			modelRequest.Model = modelName
		}
		c.Set("relay_mode", relayMode)
	} else if !strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") && !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = req.Group
		modelRequest.Models = req.Models
		modelRequest.ProviderPolicy = req.ProviderPolicy
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/realtime") {
		//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
		modelRequest.Model = c.Query("model")
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") {
		modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1/images/edits") {
		//modelRequest.Model = common.GetStringIfEmpty(c.PostForm("model"), "gpt-image-1")
		contentType := c.ContentType()
		if slices.Contains([]string{gin.MIMEPOSTForm, gin.MIMEMultipartPOSTForm}, contentType) {
			req, err := getModelFromRequest(c)
			if err == nil && req.Model != "" {
				modelRequest.Model = req.Model
				modelRequest.Models = req.Models
				modelRequest.ProviderPolicy = req.ProviderPolicy
			}
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
		relayMode := relayconstant.RelayModeAudioSpeech
		if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/speech") {

			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "tts-1")
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/translations") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
				modelRequest.Models = req.Models
				modelRequest.ProviderPolicy = req.ProviderPolicy
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranslation
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
				modelRequest.Models = req.Models
				modelRequest.ProviderPolicy = req.ProviderPolicy
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranscription
		}
		c.Set("relay_mode", relayMode)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
		// playground chat completions
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = req.Group
		modelRequest.Models = req.Models
		modelRequest.ProviderPolicy = req.ProviderPolicy
		common.SetContextKey(c, constant.ContextKeyTokenGroup, modelRequest.Group)
	}

	if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") && modelRequest.Model != "" {
		modelRequest.Model = ratio_setting.WithCompactModelSuffix(modelRequest.Model)
	}
	modelRequest.Model = service.ResolveLatestModelAlias(modelRequest.Model)
	if len(modelRequest.Models) > 0 {
		for i := range modelRequest.Models {
			modelRequest.Models[i] = service.ResolveLatestModelAlias(modelRequest.Models[i])
		}
		modelRequest.Models = dedupeProviderSelectors(modelRequest.Models)
	}
	return &modelRequest, shouldSelectChannel, nil
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) *types.NewAPIError {
	c.Set("original_model", modelName) // for retry
	if channel == nil {
		return types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	// Resolve the effective upstream type + base URL for this (channel, model).
	// When no per-model override row exists this returns the channel's own type
	// and base URL, so the context is populated exactly as before.
	resolvedChannelType, resolvedBaseURL := resolveChannelRoute(channel, modelName)
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, resolvedChannelType)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	paramOverride := channel.GetParamOverride()
	headerOverride := channel.GetHeaderOverride()
	if mergedParam, applied := service.ApplyChannelAffinityOverrideTemplate(c, paramOverride); applied {
		paramOverride = mergedParam
	}
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, paramOverride)
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, headerOverride)
	if nil != channel.OpenAIOrganization && *channel.OpenAIOrganization != "" {
		common.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key, index, usedAffinityKey, newAPIError := service.ResolveChannelAffinityMultiKey(c, channel)
	if !usedAffinityKey && newAPIError == nil {
		key, index, newAPIError = channel.GetNextEnabledKey()
	}
	if newAPIError != nil {
		return newAPIError
	}
	if channel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		// 必须设置为 false，否则在重试到单个 key 的时候会导致日志显示错误
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	// c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, resolvedBaseURL)

	common.SetContextKey(c, constant.ContextKeySystemPromptOverride, false)

	// TODO: api_version统一
	// Use the resolved channel type so a per-model protocol override also selects
	// the correct upstream-specific context (api_version, region, ...). With no
	// override resolvedChannelType == channel.Type, preserving prior behavior.
	switch resolvedChannelType {
	case constant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeVertexAi:
		c.Set("region", selectVertexRegion(channel.Other))
	case constant.ChannelTypeXunfei:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeAli:
		c.Set("plugin", channel.Other)
	case constant.ChannelCloudflare:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeMokaAI:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeCoze:
		c.Set("bot_id", channel.Other)
	}
	return nil
}

// resolveChannelRoute returns the effective upstream channel type and base URL
// for the given channel + model, applying any per-model endpoint override and
// then the existing {model} placeholder substitution. When no override applies
// it returns the channel's own type and base URL, matching prior behavior.
func resolveChannelRoute(channel *model.Channel, modelName string) (int, string) {
	if channel == nil {
		return 0, ""
	}
	channelType, baseURL, _ := model.ResolveModelRoute(channel, modelName)
	if strings.Contains(baseURL, "{model}") {
		baseURL = strings.ReplaceAll(baseURL, "{model}", modelName)
	}
	return channelType, baseURL
}

func resolveChannelBaseURL(channel *model.Channel, modelName string) string {
	_, baseURL := resolveChannelRoute(channel, modelName)
	return baseURL
}

func selectVertexRegion(other string) string {
	parts := strings.Split(other, ",")
	regions := make([]string, 0, len(parts))
	for _, part := range parts {
		region := strings.TrimSpace(part)
		if region != "" {
			regions = append(regions, region)
		}
	}
	if len(regions) == 0 {
		return strings.TrimSpace(other)
	}
	if len(regions) == 1 {
		return regions[0]
	}
	next := atomic.AddUint64(&vertexRegionCounter, 1)
	return regions[int(next-1)%len(regions)]
}

var vertexRegionCounter uint64

// extractModelNameFromGeminiPath 从 Gemini API URL 路径中提取模型名
// 输入格式: /v1beta/models/gemini-2.0-flash:generateContent
// 输出: gemini-2.0-flash
func extractModelNameFromGeminiPath(path string) string {
	// 查找 "/models/" 的位置
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return ""
	}

	// 从 "/models/" 之后开始提取
	startIndex := modelsIndex + len(modelsPrefix)
	if startIndex >= len(path) {
		return ""
	}

	// 查找 ":" 的位置，模型名在 ":" 之前
	colonIndex := strings.Index(path[startIndex:], ":")
	if colonIndex == -1 {
		// 如果没有找到 ":"，返回从 "/models/" 到路径结尾的部分
		return path[startIndex:]
	}

	// 返回模型名部分
	return path[startIndex : startIndex+colonIndex]
}
