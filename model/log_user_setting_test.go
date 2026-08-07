package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetLogUserSettingUsesRequestContext(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{RecordIpLog: true})

	oldDB := DB
	DB = nil
	t.Cleanup(func() { DB = oldDB })

	setting := getLogUserSetting(ctx, 123)
	require.True(t, setting.RecordIpLog)
}

func TestGetLogUserSettingFallsBackToDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))

	settingJSON, err := common.Marshal(dto.UserSetting{RecordIpLog: true})
	require.NoError(t, err)
	require.NoError(t, db.Create(&User{Id: 321, Username: "log-setting-user", Setting: string(settingJSON)}).Error)

	oldDB := DB
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	DB = db
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})

	ctx, _ := gin.CreateTestContext(nil)
	setting := getLogUserSetting(ctx, 321)
	require.True(t, setting.RecordIpLog)
}
