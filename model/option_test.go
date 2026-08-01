package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionsBulkRollsBackDatabaseAndOptionMapOnApplyError(t *testing.T) {
	oldDB := DB
	oldOptionMap := common.OptionMap
	t.Cleanup(func() {
		DB = oldDB
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	require.NoError(t, DB.Create(&Option{Key: "About", Value: "old-about"}).Error)
	require.NoError(t, DB.Create(&Option{Key: "ModelRequestRateLimitGroup", Value: `{"default":[10,5]}`}).Error)

	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"About":                      "old-about",
		"ModelRequestRateLimitGroup": `{"default":[10,5]}`,
	}
	common.OptionMapRWMutex.Unlock()

	err = UpdateOptionsBulk(map[string]string{
		"About":                      "new-about",
		"ModelRequestRateLimitGroup": "{invalid",
	})
	require.Error(t, err)

	var about Option
	require.NoError(t, DB.First(&about, "key = ?", "About").Error)
	require.Equal(t, "old-about", about.Value)
	var rateLimitGroup Option
	require.NoError(t, DB.First(&rateLimitGroup, "key = ?", "ModelRequestRateLimitGroup").Error)
	require.Equal(t, `{"default":[10,5]}`, rateLimitGroup.Value)

	common.OptionMapRWMutex.RLock()
	gotAbout := common.OptionMap["About"]
	gotRateLimitGroup := common.OptionMap["ModelRequestRateLimitGroup"]
	common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "old-about", gotAbout)
	require.Equal(t, `{"default":[10,5]}`, gotRateLimitGroup)
}
