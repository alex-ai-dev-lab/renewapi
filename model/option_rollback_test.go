package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionsBulkRollbackRemovesRuntimeKeyMissingFromSnapshot(t *testing.T) {
	previousDB := DB
	previousOptionMap := common.OptionMap
	t.Cleanup(func() {
		DB = previousDB
		common.OptionMap = previousOptionMap
	})

	db, err := gorm.Open(sqlite.Open("file:option_rollback_missing_map?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	common.OptionMap = map[string]string{}

	require.NoError(t, db.Create(&Option{Key: "ExistingButNotLoaded", Value: "old"}).Error)

	values := map[string]string{
		"ExistingButNotLoaded":       "new",
		"ModelRequestRateLimitGroup": `{"broken":[1]}`,
	}
	err = UpdateOptionsBulk(values)
	require.Error(t, err)

	var option Option
	require.NoError(t, db.Where("key = ?", "ExistingButNotLoaded").First(&option).Error)
	require.Equal(t, "old", option.Value)
	common.OptionMapRWMutex.RLock()
	_, exists := common.OptionMap["ExistingButNotLoaded"]
	common.OptionMapRWMutex.RUnlock()
	require.False(t, exists)
}
