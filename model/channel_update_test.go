package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelUpdateTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		ReloadChannelModelStatusCache()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &ChannelModelStatus{}))
	DB = db
	common.MemoryCacheEnabled = false
	group2model2channels = make(map[string]map[string][]int)
	channelsIDM = make(map[int]*Channel)
	ReloadChannelModelStatusCache()
}

func TestChannelUpdateUsesOpenAIOrganizationDBColumn(t *testing.T) {
	setupChannelUpdateTestDB(t)

	var columns []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, DB.Raw("PRAGMA table_info(channels)").Scan(&columns).Error)
	columnNames := make(map[string]bool, len(columns))
	for _, column := range columns {
		columnNames[column.Name] = true
	}
	require.True(t, columnNames["open_ai_organization"])
	require.False(t, columnNames["openai_organization"])

	oldOrg := "org-old"
	channel := Channel{
		Type:               1,
		Key:                "sk-test",
		OpenAIOrganization: &oldOrg,
		Status:             common.ChannelStatusEnabled,
		Name:               "openai",
		Models:             "gpt-5.5",
		Group:              "default",
		CreatedTime:        common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&channel).Error)

	newOrg := "org-new"
	updated := Channel{
		Id:                 channel.Id,
		Type:               channel.Type,
		Key:                channel.Key,
		OpenAIOrganization: &newOrg,
		Status:             common.ChannelStatusEnabled,
		Name:               "openai edited",
		Models:             "gpt-5.5",
		Group:              "default",
	}
	require.NoError(t, updated.Update())

	var storedOrg string
	require.NoError(t, DB.Table("channels").Select("open_ai_organization").Where("id = ?", channel.Id).Scan(&storedOrg).Error)
	require.Equal(t, newOrg, storedOrg)
}
