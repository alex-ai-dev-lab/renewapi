package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaAtomicTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}))
	DB = db
	common.RedisEnabled = false
	common.BatchUpdateEnabled = true
}

func TestDecreaseUserQuotaRequiresSufficientBalance(t *testing.T) {
	setupQuotaAtomicTestDB(t)
	user := User{Username: "alice", Password: "password123", Quota: 10}
	require.NoError(t, DB.Create(&user).Error)

	err := DecreaseUserQuota(user.Id, 11, false)
	require.True(t, errors.Is(err, ErrInsufficientQuota))

	var quota int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Scan(&quota).Error)
	require.Equal(t, 10, quota)
}

func TestDecreaseUserQuotaSucceedsAtomicallyWhenFunded(t *testing.T) {
	setupQuotaAtomicTestDB(t)
	user := User{Username: "bob", Password: "password123", Quota: 10}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DecreaseUserQuota(user.Id, 7, false))

	var quota int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Scan(&quota).Error)
	require.Equal(t, 3, quota)
}

func TestDecreaseTokenQuotaRequiresSufficientBalance(t *testing.T) {
	setupQuotaAtomicTestDB(t)
	token := Token{UserId: 1, Key: "sk-test", Status: common.TokenStatusEnabled, RemainQuota: 5}
	require.NoError(t, DB.Create(&token).Error)

	err := DecreaseTokenQuota(token.Id, token.Key, 6)
	require.True(t, errors.Is(err, ErrInsufficientQuota))

	var got Token
	require.NoError(t, DB.First(&got, token.Id).Error)
	require.Equal(t, 5, got.RemainQuota)
	require.Equal(t, 0, got.UsedQuota)
}

func TestDecreaseUnlimitedTokenQuotaStillTracksUsage(t *testing.T) {
	setupQuotaAtomicTestDB(t)
	token := Token{UserId: 1, Key: "sk-unlimited", Status: common.TokenStatusEnabled, RemainQuota: 0, UnlimitedQuota: true}
	require.NoError(t, DB.Create(&token).Error)

	require.NoError(t, DecreaseTokenQuota(token.Id, token.Key, 6))

	var got Token
	require.NoError(t, DB.First(&got, token.Id).Error)
	require.Equal(t, 0, got.RemainQuota)
	require.Equal(t, 6, got.UsedQuota)
}
