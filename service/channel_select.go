package service

import (
	"errors"
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
	RequireClaudeThinkingSupport              bool
	RequireOpenAIResponsesSupport             bool
	ModelDefaultEndpoint                      string
	ClientRelayFormat                         types.RelayFormat
	Request                                   dto.Request
	ForceResponsesFunctionCallArgumentsObject bool
	LastSelectedChannelId                     int
	ProviderRoutingPolicy                     *ProviderRoutingPolicy
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
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Each group will exhaust all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
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
			ChannelMatchesProviderRoutingPolicy(preferred, param.ProviderRoutingPolicy) &&
			channelMatchesRetryRequirements(param, preferred) {
			param.LastSelectedChannelId = preferred.Id
			return preferred, selectGroup, nil
		}
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
						!model.IsChannelModelDisabledForGroup(preferred.Id, autoGroup, param.ModelName) {
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

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = getRandomSatisfiedChannelWithRequirements(param, autoGroup, priorityRetry)
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
	if param.RequireClaudeThinkingSupport && !ChannelSupportsClaudeThinking(channel) {
		return false
	}
	if param.RequireOpenAIResponsesSupport {
		capability := EvaluateChannelProtocolCapability(channel, param.ModelName, types.RelayFormatOpenAIResponses, param.Request)
		if !capability.Supported || capability.Lossy {
			return false
		}
	}
	if !antipoison.ProductionRoutingAllowed(channel.Id, channel.GetSetting()) {
		return false
	}
	if !ChannelAntiPoisonCircuitAllowsProduction(channel.Id, channel.GetSetting()) {
		return false
	}
	return true
}

func ChannelAllowedForProduction(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	return antipoison.ProductionRoutingAllowed(channel.Id, channel.GetSetting()) &&
		ChannelAntiPoisonCircuitAllowsProduction(channel.Id, channel.GetSetting())
}

func getRandomSatisfiedChannelWithRequirements(param *RetryParam, group string, retry int) (*model.Channel, error) {
	if param == nil {
		return nil, errors.New("retry param is nil")
	}
	if !param.RequireClaudeThinkingSupport && !param.RequireOpenAIResponsesSupport {
		return model.GetRandomSatisfiedChannelExcludingWithPolicy(group, param.ModelName, retry, param.ExcludedChannelIds, param.ProviderRoutingPolicy)
	}
	excluded := param.ExcludedChannelIds
	if excluded == nil {
		excluded = make(map[int]bool)
		param.ExcludedChannelIds = excluded
	}
	for attempts := 0; attempts < 64; attempts++ {
		channel, err := model.GetRandomSatisfiedChannelExcludingWithPolicy(group, param.ModelName, retry, excluded, param.ProviderRoutingPolicy)
		if err != nil || channel == nil {
			return channel, err
		}
		if channelMatchesRetryRequirements(param, channel) {
			return channel, nil
		}
		excluded[channel.Id] = true
	}
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
