package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/antipoison"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx                                       *gin.Context
	TokenGroup                                string
	ModelName                                 string
	Retry                                     *int
	resetNextTry                              bool
	ExcludedChannelIds                        map[int]bool
	TriedMultiKeyIndexes                      map[int]map[int]bool
	PreferredChannelId                        int
	StrictPreferredChannel                    bool
	RequireClaudeThinkingSupport              bool
	RequireOpenAIResponsesSupport             bool
	ModelDefaultEndpoint                      string
	ClientRelayFormat                         types.RelayFormat
	Request                                   dto.Request
	ForceResponsesFunctionCallArgumentsObject bool
	LastSelectedChannelId                     int
	ProviderRoutingPolicy                     *ProviderRoutingPolicy
	ResponsesRequirement                      *ResponsesRoutingRequirement
	RequiredModelName                         string
	ModelMappingFallbackChannelId             int
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跳分组重试的 "auto" tokenGroup：
//
//   - Each group exhausts all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//   - ContextKeyAutoGroupIndex records which group to resume from on the next retry.
//     ContextKeyAutoGroupIndex 记录下次重试从哪个分组继续。
//   - priorityRetry is the priority level inside the current group: it equals param.Retry
//     while we stay in the same group, and is reset to 0 the moment we advance to a new one
//     (param.SetRetry(0) keeps the outer relay loop in sync).
//     priorityRetry 是当前分组内的优先级：留在同一分组时等于 param.Retry，
//     一旦切换到新分组就重置为 0（同时 param.SetRetry(0) 保持外层重试循环一致）。
//
// 注意: 这里仍会写 ContextKeyAutoGroupRetryIndex，但本函数已不再读它——历史上的
// startRetryIndex 算法（priorityRetry = Retry - startRetryIndex）已被上面的“切组就归零”
// 方式取代。若确认其他包也不读该 key，应连同常量一并删除。
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.PreferredChannelId > 0 && !param.ExcludedChannelIds[param.PreferredChannelId] {
		preferred, preferredErr := model.CacheGetChannel(param.PreferredChannelId)
		if preferredErr == nil && preferred != nil &&
			preferred.Status == common.ChannelStatusEnabled &&
			model.IsChannelEnabledForGroupModel(param.TokenGroup, param.ModelName, preferred.Id) &&
			!model.IsChannelModelDisabledForGroup(preferred.Id, param.TokenGroup, param.ModelName) &&
			channelSupportsRequiredModelForGroup(param, preferred, param.TokenGroup) &&
			ChannelMatchesProviderRoutingPolicy(preferred, param.ProviderRoutingPolicy) &&
			channelMatchesRetryRequirements(param, preferred) {
			param.LastSelectedChannelId = preferred.Id
			return preferred, selectGroup, nil
		}
	}
	if param.StrictPreferredChannel && param.PreferredChannelId > 0 {
		return nil, selectGroup, nil
	}

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)
		if param.PreferredChannelId > 0 && !param.ExcludedChannelIds[param.PreferredChannelId] {
			preferred, preferredErr := model.CacheGetChannel(param.PreferredChannelId)
			if preferredErr == nil && preferred != nil && preferred.Status == common.ChannelStatusEnabled &&
				ChannelMatchesProviderRoutingPolicy(preferred, param.ProviderRoutingPolicy) &&
				channelMatchesRetryRequirements(param, preferred) {
				for _, autoGroup := range autoGroups {
					if model.IsChannelEnabledForGroupModel(autoGroup, param.ModelName, preferred.Id) &&
						!model.IsChannelModelDisabledForGroup(preferred.Id, autoGroup, param.ModelName) &&
						channelSupportsRequiredModelForGroup(param, preferred, autoGroup) {
						common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
						param.LastSelectedChannelId = preferred.Id
						return preferred, autoGroup, nil
					}
				}
			}
		}

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		// lastSelectErr 保留最后一个真实错误（DB/缓存异常）。以前这个 error 被直接丢弃，
		// 导致下游无法区分“真的没有可用渠道”和“查渠道失败”。
		var lastSelectErr error
		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			var selectErr error
			channel, selectErr = getRandomSatisfiedChannelWithRequirements(param, autoGroup, priorityRetry)
			if selectErr != nil {
				lastSelectErr = selectErr
				logger.LogError(param.Ctx, fmt.Sprintf("select channel failed in group %s for model %s: %v", autoGroup, param.ModelName, selectErr))
			}
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
		// 所有分组都没选到渠道，且期间出过真实错误时，把错误往上报，
		// 与非 auto 分支的行为保持一致。
		if channel == nil && lastSelectErr != nil {
			return nil, selectGroup, lastSelectErr
		}
	} else {
		channel, err = getRandomSatisfiedChannelWithRequirements(param, param.TokenGroup, param.GetRetry())
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	if channel != nil {
		param.LastSelectedChannelId = channel.Id
	}
	return channel, selectGroup, nil
}

func channelMatchesRetryRequirements(param *RetryParam, channel *model.Channel) bool {
	if param == nil || channel == nil {
		return false
	}
	if ShouldAvoidChannelForSession(param.Ctx, channel.Id) {
		return false
	}
	if param.RequireClaudeThinkingSupport && !ChannelSupportsClaudeThinking(channel) {
		return false
	}
	if param.RequireOpenAIResponsesSupport {
		capability := EvaluateChannelProtocolCapability(channel, param.ModelName, types.RelayFormatOpenAIResponses, param.Request)
		if !capability.Supported || capability.Lossy {
			return false
		}
	}
	if !ChannelMatchesResponsesRequirement(channel, param.ModelName, param.ResponsesRequirement, param.Request) {
		return false
	}
	channelSetting := channelSettingForRouting(channel)
	if !antipoison.ProductionRoutingAllowed(channel.Id, channelSetting) {
		return false
	}
	if !ChannelAntiPoisonCircuitAllowsProduction(channel.Id, channelSetting) {
		return false
	}
	return true
}

func channelSupportsRequiredModelForGroup(param *RetryParam, channel *model.Channel, group string) bool {
	if param == nil || channel == nil || param.RequiredModelName == "" || strings.EqualFold(param.RequiredModelName, param.ModelName) {
		return true
	}
	return model.IsChannelEnabledForGroupModel(group, param.RequiredModelName, channel.Id) &&
		!model.IsChannelModelDisabledForGroup(channel.Id, group, param.RequiredModelName)
}

func ChannelAllowedForProduction(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	channelSetting := channelSettingForRouting(channel)
	return antipoison.ProductionRoutingAllowed(channel.Id, channelSetting) &&
		ChannelAntiPoisonCircuitAllowsProduction(channel.Id, channelSetting)
}

func channelSettingForRouting(channel *model.Channel) dto.ChannelSettings {
	if channel == nil {
		return dto.ChannelSettings{}
	}
	if setting, ok := model.CacheGetChannelSettingReadOnly(channel.Id); ok {
		return setting
	}
	return channel.GetSetting()
}

// requirementFilterMaxAttempts 限制“拉一个渠道→不满要求→排除后重拉”的次数，
// 避免在大量渠道都不满要求时死循环。
const requirementFilterMaxAttempts = 64

func getRandomSatisfiedChannelWithRequirements(param *RetryParam, group string, retry int) (*model.Channel, error) {
	if param == nil {
		return nil, errors.New("retry param is nil")
	}
	if !param.RequireClaudeThinkingSupport && !param.RequireOpenAIResponsesSupport && param.ResponsesRequirement == nil {
		return model.GetRandomSatisfiedChannelExcludingWithPolicy(group, param.ModelName, retry, param.ExcludedChannelIds, param.ProviderRoutingPolicy)
	}
	excluded := param.ExcludedChannelIds
	if excluded == nil {
		excluded = make(map[int]bool)
		param.ExcludedChannelIds = excluded
	}
	for attempts := 0; attempts < requirementFilterMaxAttempts; attempts++ {
		channel, err := model.GetRandomSatisfiedChannelExcludingWithPolicy(group, param.ModelName, retry, excluded, param.ProviderRoutingPolicy)
		if err != nil || channel == nil {
			return channel, err
		}
		if channelMatchesRetryRequirements(param, channel) && channelSupportsRequiredModelForGroup(param, channel, group) {
			return channel, nil
		}
		excluded[channel.Id] = true
	}
	// 以前这里静默返回 nil, nil，与“真的没有渠道”完全无法区分。
	logger.LogError(param.Ctx, fmt.Sprintf("channel requirement filter exhausted %d attempts in group %s for model %s", requirementFilterMaxAttempts, group, param.ModelName))
	return nil, nil
}

type ProviderRoutingPolicy struct {
	Only   []string
	Ignore []string
	Order  []string
}

func (p *ProviderRoutingPolicy) Empty() bool {
	return p == nil || (len(p.Only) == 0 && len(p.Ignore) == 0 && len(p.Order) == 0)
}

func ChannelMatchesProviderRoutingPolicy(channel *model.Channel, policy *ProviderRoutingPolicy) bool {
	if policy == nil {
		return true
	}
	return policy.Matches(channel)
}

func ProviderRoutingOrderRank(channel *model.Channel, policy *ProviderRoutingPolicy) int {
	if policy == nil {
		return 0
	}
	return policy.OrderRank(channel)
}

func (p *ProviderRoutingPolicy) Matches(channel *model.Channel) bool {
	if p == nil || p.Empty() || channel == nil {
		return true
	}
	if len(p.Only) > 0 && !channelMatchesAnyProviderSelector(channel, p.Only) {
		return false
	}
	if len(p.Ignore) > 0 && channelMatchesAnyProviderSelector(channel, p.Ignore) {
		return false
	}
	return true
}

func (p *ProviderRoutingPolicy) OrderRank(channel *model.Channel) int {
	if p == nil {
		return 0
	}
	if len(p.Order) == 0 || channel == nil {
		return len(p.Order)
	}
	for i, selector := range p.Order {
		if channelMatchesProviderSelector(channel, selector) {
			return i
		}
	}
	return len(p.Order)
}

func channelMatchesAnyProviderSelector(channel *model.Channel, selectors []string) bool {
	for _, selector := range selectors {
		if channelMatchesProviderSelector(channel, selector) {
			return true
		}
	}
	return false
}

func channelMatchesProviderSelector(channel *model.Channel, selector string) bool {
	normalized := normalizeProviderSelector(selector)
	if normalized == "" || channel == nil {
		return false
	}
	candidates := []string{
		normalizeProviderSelector(constant.GetChannelTypeName(channel.Type)),
		normalizeProviderSelector(channel.Name),
		normalizeProviderSelector(channel.GetTag()),
		normalizeProviderSelector(channel.GetBaseURL()),
		normalizeProviderSelector(channel.GetBaseURLHost()),
		normalizeProviderSelector(channel.IdString()),
	}
	for _, candidate := range candidates {
		if candidate == normalized {
			return true
		}
	}
	return false
}

func normalizeProviderSelector(selector string) string {
	selector = strings.TrimSpace(strings.ToLower(selector))
	selector = strings.TrimPrefix(selector, "provider:")
	selector = strings.TrimPrefix(selector, "type:")
	selector = strings.TrimPrefix(selector, "channel:")
	selector = strings.TrimPrefix(selector, "tag:")
	selector = strings.TrimPrefix(selector, "id:")
	selector = strings.TrimPrefix(selector, "#")
	selector = strings.TrimSuffix(selector, "/")
	return selector
}

func ExcludeChannelForRetry(param *RetryParam, channelID int) {
	if param == nil || channelID <= 0 {
		return
	}
	if param.ExcludedChannelIds == nil {
		param.ExcludedChannelIds = make(map[int]bool)
	}
	param.ExcludedChannelIds[channelID] = true
}

func RecordTriedMultiKeyIndex(param *RetryParam, channelID int, keyIndex int) {
	if param == nil || channelID <= 0 || keyIndex < 0 {
		return
	}
	if param.TriedMultiKeyIndexes == nil {
		param.TriedMultiKeyIndexes = make(map[int]map[int]bool)
	}
	if param.TriedMultiKeyIndexes[channelID] == nil {
		param.TriedMultiKeyIndexes[channelID] = make(map[int]bool)
	}
	param.TriedMultiKeyIndexes[channelID][keyIndex] = true
}

func HasUntriedEnabledMultiKey(param *RetryParam, channel *model.Channel) bool {
	if param == nil || channel == nil || !channel.ChannelInfo.IsMultiKey {
		return false
	}
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return false
	}
	tried := param.TriedMultiKeyIndexes[channel.Id]
	for i := range keys {
		if tried != nil && tried[i] {
			continue
		}
		if status, ok := channel.ChannelInfo.MultiKeyStatusList[i]; ok && status != common.ChannelStatusEnabled {
			continue
		}
		return true
	}
	return false
}

// SelectUntriedEnabledMultiKey 选一个本次请求还没用过、且处于启用状态的 key。
// 入参语义与 HasUntriedEnabledMultiKey 一致：后者返回 true 时，这里就应该能选出来。
func SelectUntriedEnabledMultiKey(param *RetryParam, channel *model.Channel) (string, int, bool, *types.NewAPIError) {
	if param == nil || channel == nil || !channel.ChannelInfo.IsMultiKey {
		return "", 0, false, nil
	}
	keys := channel.GetKeys()
	tried := param.TriedMultiKeyIndexes[channel.Id]
	// 第一次选择必须保留渠道亲和/轮询已经选出的 key；这里只负责失败后的
	// replacement，避免无故改变首个请求的 key。
	if len(tried) == 0 {
		return "", 0, false, nil
	}
	for index := range keys {
		if tried != nil && tried[index] {
			continue
		}
		key, selectedIndex, enabled, err := channel.GetEnabledKeyByIndex(index)
		if err != nil {
			return "", 0, false, err
		}
		if enabled {
			return key, selectedIndex, true, nil
		}
	}
	return "", 0, false, nil
}
