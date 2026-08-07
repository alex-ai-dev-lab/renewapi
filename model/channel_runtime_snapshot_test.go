package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

type snapshotTestRoutingPolicy struct {
	allowed map[int]bool
	ranks   map[int]int
}

func (p snapshotTestRoutingPolicy) Empty() bool {
	return false
}

func (p snapshotTestRoutingPolicy) Matches(channel *Channel) bool {
	return channel != nil && (p.allowed == nil || p.allowed[channel.Id])
}

func (p snapshotTestRoutingPolicy) OrderRank(channel *Channel) int {
	if channel == nil {
		return 99
	}
	if rank, ok := p.ranks[channel.Id]; ok {
		return rank
	}
	return 99
}

func setupChannelRuntimeSnapshotTest(t *testing.T, channels map[int]*Channel, routes map[string]map[string][]int) {
	t.Helper()
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroups := group2model2channels
	oldChannels := channelsIDM
	oldRuntime := channelRuntimeCache.Load()
	oldModelStatuses := channelModelStatusCache.Load()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroups
		channelsIDM = oldChannels
		channelRuntimeCache.Store(oldRuntime)
		channelModelStatusCache.Store(oldModelStatuses)
	})

	common.MemoryCacheEnabled = true
	group2model2channels = routes
	channelsIDM = channels
	channelRuntimeCache.Store(nil)
	channelModelStatusCache.Store(&sync.Map{})
}

func snapshotTestChannel(id int, priority int64) *Channel {
	weight := uint(100)
	return &Channel{
		Id:       id,
		Name:     fmt.Sprintf("channel-%d", id),
		Status:   common.ChannelStatusEnabled,
		Models:   "gpt-test",
		Group:    "default",
		Priority: &priority,
		Weight:   &weight,
	}
}

func TestChannelRuntimeSnapshotPreservesPriorityRetryAndExclusion(t *testing.T) {
	channels := map[int]*Channel{
		1: snapshotTestChannel(1, 100),
		2: snapshotTestChannel(2, 50),
		3: snapshotTestChannel(3, 0),
	}
	setupChannelRuntimeSnapshotTest(t, channels, map[string]map[string][]int{
		"default": {"gpt-test": {3, 1, 2}},
	})

	for retry, expectedID := range []int{1, 2, 3, 3} {
		channel, err := GetRandomSatisfiedChannel("default", "gpt-test", retry)
		require.NoError(t, err)
		require.Equal(t, expectedID, channel.Id)
	}

	channel, err := GetRandomSatisfiedChannelExcluding("default", "gpt-test", 2, map[int]bool{1: true})
	require.NoError(t, err)
	require.Equal(t, 2, channel.Id)
}

func TestChannelRuntimeSnapshotAppliesModelStatusAndProviderPolicy(t *testing.T) {
	channels := map[int]*Channel{
		1: snapshotTestChannel(1, 100),
		2: snapshotTestChannel(2, 100),
		3: snapshotTestChannel(3, 50),
	}
	setupChannelRuntimeSnapshotTest(t, channels, map[string]map[string][]int{
		"default": {"gpt-test": {1, 2, 3}},
	})

	channelModelStatusCacheMap().Store(
		channelModelStatusCacheKey(1, "default", "gpt-test"),
		ChannelModelStatus{ChannelId: 1, Group: "default", ModelName: "gpt-test", Status: common.ChannelStatusManuallyDisabled},
	)
	policy := snapshotTestRoutingPolicy{
		allowed: map[int]bool{2: true, 3: true},
		ranks:   map[int]int{2: 0, 3: 1},
	}
	channel, err := GetRandomSatisfiedChannelExcludingWithPolicy("default", "gpt-test", 0, nil, policy)
	require.NoError(t, err)
	require.Equal(t, 2, channel.Id)

	policy.allowed[2] = false
	channel, err = GetRandomSatisfiedChannelExcludingWithPolicy("default", "gpt-test", 0, nil, policy)
	require.NoError(t, err)
	require.Equal(t, 3, channel.Id)
}

func TestChannelRuntimeSnapshotUsesNormalizedModelFallback(t *testing.T) {
	channel := snapshotTestChannel(7, 100)
	setupChannelRuntimeSnapshotTest(t, map[int]*Channel{7: channel}, map[string]map[string][]int{
		"default": {"gpt-4o-gizmo-*": {7}},
	})

	selected, err := GetRandomSatisfiedChannel("default", "gpt-4o-gizmo-customer", 0)
	require.NoError(t, err)
	require.Same(t, channel, selected)
}

func TestChannelRuntimeSnapshotCachesParsedSettingsAndRefreshes(t *testing.T) {
	channel := snapshotTestChannel(9, 100)
	channel.SetSetting(dto.ChannelSettings{AntiPoisonProfile: "strict"})
	setupChannelRuntimeSnapshotTest(t, map[int]*Channel{9: channel}, map[string]map[string][]int{
		"default": {"gpt-test": {9}},
	})

	setting, ok := CacheGetChannelSettingReadOnly(9)
	require.True(t, ok)
	require.Equal(t, "strict", setting.AntiPoisonProfile)

	updated := *channel
	updated.SetSetting(dto.ChannelSettings{AntiPoisonProfile: "probation"})
	CacheUpdateChannel(&updated)
	setting, ok = CacheGetChannelSettingReadOnly(9)
	require.True(t, ok)
	require.Equal(t, "probation", setting.AntiPoisonProfile)
}

func TestChannelRuntimeSnapshotConcurrentPublishAndSelection(t *testing.T) {
	channels := map[int]*Channel{
		1: snapshotTestChannel(1, 100),
		2: snapshotTestChannel(2, 50),
	}
	setupChannelRuntimeSnapshotTest(t, channels, map[string]map[string][]int{
		"default": {"gpt-test": {1, 2}},
	})

	errCh := make(chan error, 4)
	var readers sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for i := 0; i < 500; i++ {
				channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0)
				if err != nil || channel == nil {
					errCh <- fmt.Errorf("selection failed: channel=%v err=%v", channel, err)
					return
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		CacheUpdateChannelStatus(1, common.ChannelStatusManuallyDisabled)
		CacheUpdateChannelStatus(1, common.ChannelStatusEnabled)
	}
	readers.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}
