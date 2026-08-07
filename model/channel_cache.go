package model

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex
var channelRuntimeCache atomic.Pointer[channelRuntimeSnapshot]

type channelRuntimeSnapshot struct {
	groupModels map[string]map[string][]channelPriorityBucket
	channels    map[int]*Channel
	settings    map[int]cachedChannelSetting
}

type channelPriorityBucket struct {
	priority int64
	entries  []channelSelectionEntry
}

type channelSelectionEntry struct {
	channel             *Channel
	modelStatusCacheKey string
}

type cachedChannelSetting struct {
	raw     string
	setting dto.ChannelSettings
	valid   bool
}

func buildChannelRuntimeSnapshot(previous *channelRuntimeSnapshot) *channelRuntimeSnapshot {
	snapshot := &channelRuntimeSnapshot{
		groupModels: make(map[string]map[string][]channelPriorityBucket, len(group2model2channels)),
		channels:    make(map[int]*Channel, len(channelsIDM)),
		settings:    make(map[int]cachedChannelSetting, len(channelsIDM)),
	}
	for id, channel := range channelsIDM {
		if channel == nil {
			continue
		}
		snapshot.channels[id] = channel
		raw := ""
		if channel.Setting != nil {
			raw = *channel.Setting
		}
		if previous != nil {
			if cached, ok := previous.settings[id]; ok && cached.raw == raw {
				snapshot.settings[id] = cached
				continue
			}
		}
		cached := cachedChannelSetting{raw: raw, valid: true}
		if raw != "" {
			cached.valid = common.Unmarshal([]byte(raw), &cached.setting) == nil
		}
		snapshot.settings[id] = cached
	}

	for group, modelChannels := range group2model2channels {
		modelBuckets := make(map[string][]channelPriorityBucket, len(modelChannels))
		for modelName, channelIDs := range modelChannels {
			entries := make([]channelSelectionEntry, 0, len(channelIDs))
			for _, channelID := range channelIDs {
				channel := snapshot.channels[channelID]
				if channel == nil {
					continue
				}
				entries = append(entries, channelSelectionEntry{
					channel:             channel,
					modelStatusCacheKey: channelModelStatusCacheKey(channelID, group, modelName),
				})
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].channel.GetPriority() > entries[j].channel.GetPriority()
			})
			buckets := make([]channelPriorityBucket, 0, 4)
			for _, entry := range entries {
				priority := entry.channel.GetPriority()
				last := len(buckets) - 1
				if last < 0 || buckets[last].priority != priority {
					buckets = append(buckets, channelPriorityBucket{priority: priority})
					last++
				}
				buckets[last].entries = append(buckets[last].entries, entry)
			}
			modelBuckets[modelName] = buckets
		}
		snapshot.groupModels[group] = modelBuckets
	}
	return snapshot
}

func publishChannelRuntimeSnapshotLocked() {
	channelRuntimeCache.Store(buildChannelRuntimeSnapshot(channelRuntimeCache.Load()))
}

func loadChannelRuntimeSnapshot() *channelRuntimeSnapshot {
	if snapshot := channelRuntimeCache.Load(); snapshot != nil {
		return snapshot
	}
	channelSyncLock.RLock()
	snapshot := channelRuntimeCache.Load()
	if snapshot == nil {
		snapshot = buildChannelRuntimeSnapshot(nil)
	}
	channelSyncLock.RUnlock()
	if current := channelRuntimeCache.Load(); current != nil {
		return current
	}
	if channelRuntimeCache.CompareAndSwap(nil, snapshot) {
		return snapshot
	}
	return channelRuntimeCache.Load()
}

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
	publishChannelRuntimeSnapshotLocked()
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
	if len(excluded) > 0 {
		retry = 0
	}
	if retry < 0 {
		retry = 0
	}

	snapshot := loadChannelRuntimeSnapshot()
	now := common.GetTimestamp()
	channel, found, err := selectRandomChannelFromSnapshot(snapshot, group, model, retry, excluded, policy, now)
	if found || err != nil {
		return channel, err
	}

	normalizedModel := ratio_setting.FormatMatchingModelName(model)
	if normalizedModel == "" || normalizedModel == model {
		return nil, nil
	}
	channel, _, err = selectRandomChannelFromSnapshot(snapshot, group, normalizedModel, retry, excluded, policy, now)
	return channel, err
}

func selectRandomChannelFromSnapshot(
	snapshot *channelRuntimeSnapshot,
	group string,
	modelName string,
	retry int,
	excluded map[int]bool,
	policy ChannelRoutingPolicy,
	now int64,
) (*Channel, bool, error) {
	if snapshot == nil {
		return nil, false, nil
	}
	modelBuckets := snapshot.groupModels[group]
	if modelBuckets == nil {
		return nil, false, nil
	}
	buckets := modelBuckets[modelName]
	if len(buckets) == 0 {
		return nil, false, nil
	}

	policyEmpty := policy == nil || policy.Empty()
	selectedBucket := -1
	lastAvailableBucket := -1
	availableBuckets := 0
	totalCandidates := 0
	for bucketIndex := range buckets {
		available := false
		for _, entry := range buckets[bucketIndex].entries {
			if !channelSelectionEntrySelectable(entry, excluded, policy, policyEmpty, now) {
				continue
			}
			available = true
			totalCandidates++
		}
		if !available {
			continue
		}
		if availableBuckets == retry {
			selectedBucket = bucketIndex
		}
		lastAvailableBucket = bucketIndex
		availableBuckets++
	}
	if totalCandidates == 0 {
		return nil, false, nil
	}
	if selectedBucket < 0 {
		selectedBucket = lastAvailableBucket
	}
	bucket := buckets[selectedBucket]
	if totalCandidates == 1 {
		for _, entry := range bucket.entries {
			if channelSelectionEntrySelectable(entry, excluded, policy, policyEmpty, now) {
				return entry.channel, true, nil
			}
		}
	}

	bestRank := 0
	bestRankSet := false
	selectedCount := 0
	sumWeight := 0
	for _, entry := range bucket.entries {
		if !channelSelectionEntrySelectable(entry, excluded, policy, policyEmpty, now) {
			continue
		}
		rank := 0
		if !policyEmpty {
			rank = policy.OrderRank(entry.channel)
		}
		if !bestRankSet || rank < bestRank {
			bestRank = rank
			bestRankSet = true
			selectedCount = 0
			sumWeight = 0
		}
		if rank == bestRank {
			selectedCount++
			sumWeight += entry.channel.GetWeight()
		}
	}
	if selectedCount == 0 {
		return nil, true, fmt.Errorf("no channel found, group: %s, model: %s, priority: %d", group, modelName, bucket.priority)
	}

	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = selectedCount * 100
		smoothingAdjustment = 100
	} else if sumWeight/selectedCount < 10 {
		smoothingFactor = 100
	}
	randomWeight := rand.Intn(sumWeight * smoothingFactor)
	for _, entry := range bucket.entries {
		if !channelSelectionEntrySelectable(entry, excluded, policy, policyEmpty, now) {
			continue
		}
		if !policyEmpty && policy.OrderRank(entry.channel) != bestRank {
			continue
		}
		randomWeight -= entry.channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return entry.channel, true, nil
		}
	}
	return nil, true, fmt.Errorf("channel not found, group: %s, model: %s, priority: %d", group, modelName, bucket.priority)
}

func channelSelectionEntrySelectable(
	entry channelSelectionEntry,
	excluded map[int]bool,
	policy ChannelRoutingPolicy,
	policyEmpty bool,
	now int64,
) bool {
	channel := entry.channel
	if channel == nil || excluded[channel.Id] || channel.Type == constant.ChannelTypeMock {
		return false
	}
	if isChannelModelStatusDisabledByCacheKey(entry.modelStatusCacheKey, now) {
		return false
	}
	return policyEmpty || policy.Matches(channel)
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	c, ok := loadChannelRuntimeSnapshot().channels[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

// CacheGetChannelSettingReadOnly returns the parsed setting stored in the
// current runtime snapshot. Callers must treat nested slices, maps and pointers
// as immutable. A false result preserves the existing parse-and-repair fallback.
func CacheGetChannelSettingReadOnly(id int) (dto.ChannelSettings, bool) {
	if !common.MemoryCacheEnabled {
		return dto.ChannelSettings{}, false
	}
	cached, ok := loadChannelRuntimeSnapshot().settings[id]
	if !ok || !cached.valid {
		return dto.ChannelSettings{}, false
	}
	return cached.setting, true
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	c, ok := loadChannelRuntimeSnapshot().channels[id]
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
	snapshot := loadChannelRuntimeSnapshot()
	channels := make([]*Channel, 0, len(snapshot.channels))
	for _, channel := range snapshot.channels {
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
	defer func() {
		publishChannelRuntimeSnapshotLocked()
		channelSyncLock.Unlock()
	}()
	channel, ok := channelsIDM[id]
	if ok && channel != nil {
		updated := *channel
		updated.Status = status
		channel = &updated
		channelsIDM[id] = channel
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
	if channel == nil {
		return
	}
	channelSyncLock.Lock()
	defer func() {
		publishChannelRuntimeSnapshotLocked()
		channelSyncLock.Unlock()
	}()

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
}
