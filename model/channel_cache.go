package model

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

func InitChannelCache() {
	if err := ReloadChannelCache(); err != nil {
		common.SysError("failed to reload channel cache: " + err.Error())
	}
}

// ReloadChannelCache publishes a new snapshot only after all required queries
// succeed, preserving the last-known-good cache during database failures.
func ReloadChannelCache() error {
	if !common.MemoryCacheEnabled {
		return nil
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	if err := DB.Find(&channels).Error; err != nil {
		return err
	}
	ReloadChannelModelStatusCache()
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
	}
	var abilities []*Ability
	if err := DB.Find(&abilities).Error; err != nil {
		return err
	}
	if err := ReloadChannelModelCapabilityCacheWithError(); err != nil {
		common.SysError("failed to reload channel model capability cache; keeping last-known-good snapshot: " + err.Error())
	}
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := channel.GetRoutingModels()
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
	return nil
}

func RunChannelCacheSync(ctx context.Context, frequency int) {
	if frequency <= 0 {
		frequency = 60
	}
	ticker := time.NewTicker(time.Duration(frequency) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			common.SysLog("syncing channels from database")
			InitChannelCache()
		}
	}
}

func SyncChannelCache(frequency int) {
	RunChannelCacheSync(context.Background(), frequency)
}

func GetRandomSatisfiedChannel(group string, model string, retry int) (*Channel, error) {
	return GetRandomSatisfiedChannelExcluding(group, model, retry, nil)
}

func GetRandomSatisfiedChannelExcluding(group string, model string, retry int, excluded map[int]bool) (*Channel, error) {
	return GetRandomSatisfiedChannelExcludingWithPolicy(group, model, retry, excluded, nil)
}

func GetRandomSatisfiedChannelExcludingWithPolicy(group string, model string, retry int, excluded map[int]bool, policy ChannelRoutingPolicy) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannelExcludingWithPolicy(group, model, retry, excluded, policy)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]
	channels = filterExcludedChannelIDs(channels, excluded)
	channels = FilterChannelIDsByModelStatus(channels, group, model)
	channels = filterRandomSelectableChannelIDs(channels)
	channels = filterProviderRoutingChannelIDs(channels, policy)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
		channels = filterExcludedChannelIDs(channels, excluded)
		channels = FilterChannelIDsByModelStatus(channels, group, normalizedModel)
		channels = filterRandomSelectableChannelIDs(channels)
		channels = filterProviderRoutingChannelIDs(channels, policy)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(excluded) > 0 {
		retry = 0
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	sortChannelsByProviderRoutingOrder(targetChannels, policy)
	targetChannels = keepBestProviderRoutingOrderRank(targetChannels, policy)

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	sumWeight = 0
	for _, channel := range targetChannels {
		sumWeight += channel.GetWeight()
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func filterExcludedChannelIDs(channels []int, excluded map[int]bool) []int {
	if len(channels) == 0 || len(excluded) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		if excluded[channelID] {
			continue
		}
		filtered = append(filtered, channelID)
	}
	return filtered
}

func filterRandomSelectableChannelIDs(channels []int) []int {
	if len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		channel, ok := channelsIDM[channelID]
		if !ok || channel.Type == constant.ChannelTypeMock {
			continue
		}
		filtered = append(filtered, channelID)
	}
	return filtered
}

func filterProviderRoutingChannelIDs(channels []int, policy ChannelRoutingPolicy) []int {
	if len(channels) == 0 || policy == nil || policy.Empty() {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		channel, ok := channelsIDM[channelID]
		if !ok || !policy.Matches(channel) {
			continue
		}
		filtered = append(filtered, channelID)
	}
	return filtered
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

// CacheGetAllChannels returns a snapshot slice of all cached channels (including
// disabled ones). It lets background loops such as the auto-test scheduler reuse
// the already-synced in-memory cache instead of issuing a full DB load (with
// keys) on every scan. Returns nil when the memory cache is disabled so callers
// can fall back to a direct DB query.
func CacheGetAllChannels() []*Channel {
	if !common.MemoryCacheEnabled {
		return nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	channels := make([]*Channel, 0, len(channelsIDM))
	for _, channel := range channelsIDM {
		if channel != nil {
			channels = append(channels, channel)
		}
	}
	return channels
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	channel, ok := channelsIDM[id]
	if ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// Remove the channel from its own group/model buckets instead of rebuilding
		// every bucket in the cache. Fall back to a full scan only when the channel
		// metadata is unknown.
		if !ok || channel == nil {
			for group, model2channels := range group2model2channels {
				for modelName, channels := range model2channels {
					filtered := make([]int, 0, len(channels))
					for _, channelId := range channels {
						if channelId != id {
							filtered = append(filtered, channelId)
						}
					}
					group2model2channels[group][modelName] = filtered
				}
			}
			return
		}
		for _, group := range channel.GetGroups() {
			model2channels, exists := group2model2channels[group]
			if !exists {
				continue
			}
			for _, modelName := range channel.GetRoutingModels() {
				modelName = strings.TrimSpace(modelName)
				channels, exists := model2channels[modelName]
				if !exists {
					continue
				}
				filtered := make([]int, 0, len(channels))
				for _, channelId := range channels {
					if channelId != id {
						filtered = append(filtered, channelId)
					}
				}
				model2channels[modelName] = filtered
			}
		}
		return
	}
	if !ok {
		return
	}
	if group2model2channels == nil {
		group2model2channels = make(map[string]map[string][]int)
	}
	for _, group := range channel.GetGroups() {
		if group == "" {
			continue
		}
		if group2model2channels[group] == nil {
			group2model2channels[group] = make(map[string][]int)
		}
		for _, modelName := range channel.GetRoutingModels() {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			channels := group2model2channels[group][modelName]
			alreadyPresent := false
			for _, channelID := range channels {
				if channelID == id {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				channels = append(channels, id)
			}
			sort.Slice(channels, func(i, j int) bool {
				left, leftOK := channelsIDM[channels[i]]
				right, rightOK := channelsIDM[channels[j]]
				if !leftOK || left == nil {
					return false
				}
				if !rightOK || right == nil {
					return true
				}
				return left.GetPriority() > right.GetPriority()
			})
			group2model2channels[group][modelName] = channels
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
}
