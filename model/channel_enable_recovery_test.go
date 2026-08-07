package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelEnableRecoveryTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	oldRuntime := channelRuntimeCache.Load()
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channelRuntimeCache.Store(oldRuntime)
		ReloadChannelModelStatusCache()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &ChannelModelStatus{}))
	DB = db
	common.MemoryCacheEnabled = true
	group2model2channels = make(map[string]map[string][]int)
	channelsIDM = make(map[int]*Channel)
	channelRuntimeCache.Store(nil)
	ReloadChannelModelStatusCache()
}

func TestEnableChannelResetsAutoDisabledModelStatusesKeepsManual(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&[]ChannelModelStatus{
		{
			ChannelId:      71,
			Group:          "default",
			ModelName:      "claude-opus-4-6",
			Status:         common.ChannelStatusAutoDisabled,
			FailureCount:   3,
			LastError:      "not implemented",
			LastStatusCode: 501,
			DisabledUntil:  now + 600,
			LastDisabledAt: now,
			LastDisabledBy: "auto",
			CreatedTime:    now,
			UpdatedTime:    now,
		},
		{
			ChannelId:      71,
			Group:          "default",
			ModelName:      "claude-sonnet-4-6",
			Status:         common.ChannelStatusManuallyDisabled,
			FailureCount:   1,
			LastError:      "manual stop",
			LastStatusCode: 0,
			DisabledUntil:  0,
			LastDisabledAt: now,
			LastDisabledBy: "manual",
			CreatedTime:    now,
			UpdatedTime:    now,
		},
	}).Error)
	ReloadChannelModelStatusCache()
	require.True(t, IsChannelModelDisabledForGroup(71, "default", "claude-opus-4-6"))
	require.True(t, IsChannelModelDisabledForGroup(71, "default", "claude-sonnet-4-6"))

	rowsAffected, err := EnableAutoDisabledChannelModelStatuses(71)
	require.NoError(t, err)
	require.EqualValues(t, 1, rowsAffected)

	autoStatus, err := GetChannelModelStatus(71, "default", "claude-opus-4-6")
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, autoStatus.Status)
	require.Equal(t, 0, autoStatus.FailureCount)
	require.Equal(t, int64(0), autoStatus.DisabledUntil)
	require.Equal(t, int64(0), autoStatus.LastDisabledAt)
	require.Empty(t, autoStatus.LastDisabledBy)
	require.Empty(t, autoStatus.LastError)
	require.False(t, IsChannelModelDisabledForGroup(71, "default", "claude-opus-4-6"))

	manualStatus, err := GetChannelModelStatus(71, "default", "claude-sonnet-4-6")
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusManuallyDisabled, manualStatus.Status)
	require.Equal(t, "manual", manualStatus.LastDisabledBy)
	require.True(t, IsChannelModelDisabledForGroup(71, "default", "claude-sonnet-4-6"))
}

func TestCacheUpdateChannelStatusReAddsChannelSortedByPriority(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	highPriority := int64(100)
	lowPriority := int64(10)
	channelsIDM = map[int]*Channel{
		71: {Id: 71, Status: common.ChannelStatusManuallyDisabled, Group: "default", Models: "claude-opus-4-6", Priority: &highPriority},
		89: {Id: 89, Status: common.ChannelStatusEnabled, Group: "default", Models: "claude-opus-4-6", Priority: &lowPriority},
	}
	group2model2channels = map[string]map[string][]int{
		"default": {
			"claude-opus-4-6": {89},
		},
	}

	CacheUpdateChannelStatus(71, common.ChannelStatusEnabled)
	require.Equal(t, []int{71, 89}, group2model2channels["default"]["claude-opus-4-6"])

	CacheUpdateChannelStatus(71, common.ChannelStatusEnabled)
	require.Equal(t, []int{71, 89}, group2model2channels["default"]["claude-opus-4-6"])
}

func TestManualEnableChannelImmediatelyRestoresHighPriorityRouting(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	highPriority := int64(100)
	lowPriority := int64(10)
	channelsIDM = map[int]*Channel{
		71: {Id: 71, Status: common.ChannelStatusEnabled, Group: "default", Models: "claude-opus-4-6", Priority: &highPriority},
		89: {Id: 89, Status: common.ChannelStatusEnabled, Group: "default", Models: "claude-opus-4-6", Priority: &lowPriority},
	}
	group2model2channels = map[string]map[string][]int{
		"default": {
			"claude-opus-4-6": {71, 89},
		},
	}

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&ChannelModelStatus{
		ChannelId:     71,
		Group:         "default",
		ModelName:     "claude-opus-4-6",
		Status:        common.ChannelStatusAutoDisabled,
		FailureCount:  3,
		DisabledUntil: now + 600,
		CreatedTime:   now,
		UpdatedTime:   now,
	}).Error)
	ReloadChannelModelStatusCache()

	channel, err := GetRandomSatisfiedChannelExcludingWithPolicy("default", "claude-opus-4-6", 0, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 89, channel.Id)

	OnChannelEnabled(71)
	channel, err = GetRandomSatisfiedChannelExcludingWithPolicy("default", "claude-opus-4-6", 0, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 71, channel.Id)
}

func TestUpdateChannelStatusEnableResetsAutoDisabledModelStatus(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	priority := int64(100)
	channel := Channel{
		Id:       71,
		Key:      "test-key",
		Status:   common.ChannelStatusAutoDisabled,
		Name:     "high",
		Group:    "default",
		Models:   "claude-opus-4-6",
		Priority: &priority,
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	InitChannelCache()

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&ChannelModelStatus{
		ChannelId:     71,
		Group:         "default",
		ModelName:     "claude-opus-4-6",
		Status:        common.ChannelStatusAutoDisabled,
		FailureCount:  3,
		DisabledUntil: now + 600,
		CreatedTime:   now,
		UpdatedTime:   now,
	}).Error)
	ReloadChannelModelStatusCache()

	require.True(t, UpdateChannelStatus(71, "", common.ChannelStatusEnabled, "manual enable"))

	status, err := GetChannelModelStatus(71, "default", "claude-opus-4-6")
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, status.Status)
	require.False(t, IsChannelModelDisabledForGroup(71, "default", "claude-opus-4-6"))

	var ability Ability
	require.NoError(t, DB.Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", 71, "default", "claude-opus-4-6").First(&ability).Error)
	require.True(t, ability.Enabled)
	require.Equal(t, []int{71}, group2model2channels["default"]["claude-opus-4-6"])
}

func TestEnableChannelByTagResetsAutoDisabledModelStatuses(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	tag := "claude-primary"
	priority := int64(100)
	channel := Channel{
		Id:       71,
		Key:      "test-key",
		Status:   common.ChannelStatusManuallyDisabled,
		Name:     "high",
		Group:    "default",
		Models:   "claude-opus-4-6",
		Priority: &priority,
		Tag:      &tag,
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	InitChannelCache()

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&ChannelModelStatus{
		ChannelId:     71,
		Group:         "default",
		ModelName:     "claude-opus-4-6",
		Status:        common.ChannelStatusAutoDisabled,
		FailureCount:  3,
		DisabledUntil: now + 600,
		CreatedTime:   now,
		UpdatedTime:   now,
	}).Error)
	ReloadChannelModelStatusCache()

	require.NoError(t, EnableChannelByTag(tag))

	status, err := GetChannelModelStatus(71, "default", "claude-opus-4-6")
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, status.Status)
	require.False(t, IsChannelModelDisabledForGroup(71, "default", "claude-opus-4-6"))

	var refreshed Channel
	require.NoError(t, DB.First(&refreshed, "id = ?", 71).Error)
	require.Equal(t, common.ChannelStatusEnabled, refreshed.Status)

	var ability Ability
	require.NoError(t, DB.Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", 71, "default", "claude-opus-4-6").First(&ability).Error)
	require.True(t, ability.Enabled)
	require.Equal(t, []int{71}, group2model2channels["default"]["claude-opus-4-6"])
}

func TestClearChannelAntiPoisonRiskEnableResetsAutoDisabledModelStatus(t *testing.T) {
	setupChannelEnableRecoveryTestDB(t)

	priority := int64(100)
	channel := Channel{
		Id:       71,
		Key:      "test-key",
		Status:   common.ChannelStatusManuallyDisabled,
		Name:     "high",
		Group:    "default",
		Models:   "claude-opus-4-6",
		Priority: &priority,
	}
	channel.SetOtherInfo(map[string]interface{}{
		"risk_status":   "anti_poison",
		"risk_reason":   "validation failed",
		"risk_action":   "manual_disabled",
		"status_reason": "anti-poison risk: validation failed",
	})
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	InitChannelCache()

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&ChannelModelStatus{
		ChannelId:     71,
		Group:         "default",
		ModelName:     "claude-opus-4-6",
		Status:        common.ChannelStatusAutoDisabled,
		FailureCount:  3,
		DisabledUntil: now + 600,
		CreatedTime:   now,
		UpdatedTime:   now,
	}).Error)
	ReloadChannelModelStatusCache()

	require.NoError(t, ClearChannelAntiPoisonRisk(71, true))

	status, err := GetChannelModelStatus(71, "default", "claude-opus-4-6")
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, status.Status)
	require.False(t, IsChannelModelDisabledForGroup(71, "default", "claude-opus-4-6"))

	var refreshed Channel
	require.NoError(t, DB.First(&refreshed, "id = ?", 71).Error)
	require.Equal(t, common.ChannelStatusEnabled, refreshed.Status)
	require.NotContains(t, refreshed.GetOtherInfo(), "risk_status")
	require.Equal(t, []int{71}, group2model2channels["default"]["claude-opus-4-6"])
}
