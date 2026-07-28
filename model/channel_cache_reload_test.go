package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReloadChannelCachePreservesLastKnownGoodOnDatabaseError(t *testing.T) {
	oldDB := DB
	oldMemoryCache := common.MemoryCacheEnabled
	oldChannels := channelsIDM
	oldGroups := group2model2channels
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCache
		channelsIDM = oldChannels
		group2model2channels = oldGroups
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	DB = db
	common.MemoryCacheEnabled = true
	sentinel := &Channel{Id: 42, Name: "last-known-good"}
	channelsIDM = map[int]*Channel{42: sentinel}
	group2model2channels = map[string]map[string][]int{"default": {"gpt-test": {42}}}

	require.Error(t, ReloadChannelCache())
	require.Same(t, sentinel, channelsIDM[42])
	require.Equal(t, []int{42}, group2model2channels["default"]["gpt-test"])
}
