package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func BenchmarkGetRandomSatisfiedChannel1000(b *testing.B) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroups := group2model2channels
	oldChannels := channelsIDM
	oldRuntime := channelRuntimeCache.Load()
	oldModelStatuses := channelModelStatusCache.Load()
	b.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroups
		channelsIDM = oldChannels
		channelRuntimeCache.Store(oldRuntime)
		channelModelStatusCache.Store(oldModelStatuses)
	})

	common.MemoryCacheEnabled = true
	channelRuntimeCache.Store(nil)
	channelModelStatusCache.Store(&sync.Map{})
	channels := make([]int, 0, 1000)
	channelsIDM = make(map[int]*Channel, 1000)
	for id := 1; id <= 1000; id++ {
		priority := int64(9 - ((id - 1) / 100))
		weight := uint((id % 100) + 1)
		channels = append(channels, id)
		channelsIDM[id] = &Channel{
			Id:       id,
			Name:     fmt.Sprintf("channel-%d", id),
			Status:   common.ChannelStatusEnabled,
			Models:   "gpt-benchmark",
			Group:    "default",
			Priority: &priority,
			Weight:   &weight,
		}
	}
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-benchmark": channels},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		channel, err := GetRandomSatisfiedChannel("default", "gpt-benchmark", i%10)
		if err != nil || channel == nil {
			b.Fatalf("selection failed: channel=%v err=%v", channel, err)
		}
	}
}
