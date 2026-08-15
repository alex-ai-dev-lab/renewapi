package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserUpdateTestState(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldQuotaForNewUser := common.QuotaForNewUser
	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.QuotaForNewUser = oldQuotaForNewUser
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.QuotaForNewUser = 0
}

func TestEnsureEmailAvailableRejectsExistingEmailCaseInsensitive(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "Taken@Example.com",
		AffCode:  "existing",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := EnsureEmailAvailable(" taken@example.COM ", 0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	user, err := GetUniqueUserByEmail("TAKEN@example.com")
	require.NoError(t, err)
	assert.Equal(t, "existing", user.Username)
	require.NoError(t, EnsureEmailAvailable("taken@example.com", user.Id))
}

func TestInsertRejectsDuplicateEmailAndNormalizesNewEmail(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "taken@example.com",
		AffCode:  "existing",
		Status:   common.UserStatusEnabled,
	}).Error)

	duplicate := &User{
		Username: "oauth-user",
		Email:    "TAKEN@example.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.ErrorIs(t, duplicate.Insert(0), ErrEmailAlreadyTaken)

	created := &User{
		Username: "new-user",
		Email:    " New@Example.COM ",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, created.Insert(0))
	assert.Equal(t, "new@example.com", created.Email)
}

func TestValidateAndFillRejectsPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "passwordless-user",
		Password: "",
		AffCode:  "passwordless",
		Status:   common.UserStatusEnabled,
	}).Error)

	loginUser := User{Username: "passwordless-user", Password: "NewPassword123"}
	require.ErrorIs(t, loginUser.ValidateAndFill(), ErrInvalidCredentials)
}

func TestResetUserPasswordByEmailRequiresSingleActiveMatch(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "duplicate-1", Password: "old-1", Email: "legacy@example.com",
		AffCode: "dupe1", Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Username: "duplicate-2", Password: "old-2", Email: "LEGACY@example.com",
		AffCode: "dupe2", Status: common.UserStatusEnabled,
	}).Error)

	err := ResetUserPasswordByEmail("legacy@example.com", "NewPassword123")
	require.ErrorIs(t, err, ErrEmailAmbiguous)

	var duplicates []User
	require.NoError(t, DB.Where("LOWER(email) = ?", "legacy@example.com").Order("username asc").Find(&duplicates).Error)
	require.Len(t, duplicates, 2)
	assert.Equal(t, "old-1", duplicates[0].Password)
	assert.Equal(t, "old-2", duplicates[1].Password)

	require.NoError(t, DB.Create(&User{
		Username: "unique", Password: "old", Email: "unique@example.com",
		AffCode: "unique", Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, ResetUserPasswordByEmail("UNIQUE@example.com", "NewPassword123"))

	var unique User
	require.NoError(t, DB.Where("username = ?", "unique").First(&unique).Error)
	assert.True(t, common.ValidatePasswordAndHash("NewPassword123", unique.Password))
	assert.True(t, errors.Is(ResetUserPasswordByEmail("missing@example.com", "NewPassword123"), ErrEmailNotFound))
}

func TestUpdateWithTxPreservesQuotaCounters(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username: "counter-user", Password: "old", Email: "old@example.com",
		AffCode: "counter", Quota: 100, UsedQuota: 20, RequestCount: 3,
	}
	require.NoError(t, DB.Create(&user).Error)

	user.Email = "new@example.com"
	user.Quota = 0
	user.UsedQuota = 0
	user.RequestCount = 0
	require.NoError(t, user.UpdateWithTx(DB, false))

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, "new@example.com", stored.Email)
	assert.Equal(t, 100, stored.Quota)
	assert.Equal(t, 20, stored.UsedQuota)
	assert.Equal(t, 3, stored.RequestCount)
}

func TestUpdateUserBindColumnPreservesConcurrentAccountState(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username: "binding-user", Password: "old", Email: "old@example.com",
		AffCode: "binding", Quota: 100, Status: common.UserStatusEnabled,
		Group: "default",
	}
	require.NoError(t, DB.Create(&user).Error)

	// Simulate state changed after the binding handler loaded its User snapshot.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"quota":  275,
		"status": common.UserStatusDisabled,
		"group":  "vip",
	}).Error)

	require.NoError(t, UpdateUserBindColumn(user.Id, "github_id", "github-123"))

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, "github-123", stored.GitHubId)
	assert.Equal(t, 275, stored.Quota)
	assert.Equal(t, common.UserStatusDisabled, stored.Status)
	assert.Equal(t, "vip", stored.Group)
}

func TestUpdateUserBindColumnDoesNotLoseConcurrentQuotaOrStatusUpdates(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username: "binding-concurrent", Password: "old", AffCode: "binding-concurrent",
		Quota: 100, Status: common.UserStatusEnabled, Group: "default",
	}
	require.NoError(t, DB.Create(&user).Error)

	start := make(chan struct{})
	errs := make(chan error, 3)
	var wg sync.WaitGroup
	operations := []func() error{
		func() error { return UpdateUserBindColumn(user.Id, "github_id", "github-concurrent") },
		func() error {
			return DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", gorm.Expr("quota + ?", 75)).Error
		},
		func() error {
			return DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
				"status": common.UserStatusDisabled,
				"group":  "vip",
			}).Error
		},
	}
	for _, operation := range operations {
		wg.Add(1)
		go func(operation func() error) {
			defer wg.Done()
			<-start
			errs <- operation()
		}(operation)
	}
	close(start)
	wg.Wait()
	close(errs)
	for operationErr := range errs {
		require.NoError(t, operationErr)
	}

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, "github-concurrent", stored.GitHubId)
	assert.Equal(t, 175, stored.Quota)
	assert.Equal(t, common.UserStatusDisabled, stored.Status)
	assert.Equal(t, "vip", stored.Group)
}

func TestUpdateUserBindColumnRejectsUnknownColumn(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{Username: "binding-whitelist", Password: "old", AffCode: "binding-whitelist"}
	require.NoError(t, DB.Create(&user).Error)

	err := UpdateUserBindColumn(user.Id, "quota", "999999")
	require.ErrorContains(t, err, "unsupported user binding column")

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Zero(t, stored.Quota)
}

func TestUpdateUserBindColumnKeepsDuplicateDetection(t *testing.T) {
	setupUserUpdateTestState(t)

	first := User{Username: "binding-first", Password: "old", AffCode: "binding-first"}
	second := User{Username: "binding-second", Password: "old", AffCode: "binding-second"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)
	require.NoError(t, UpdateUserBindColumn(first.Id, "wechat_id", "wechat-duplicate"))

	assert.True(t, IsWeChatIdAlreadyTaken("wechat-duplicate"))
	assert.False(t, IsWeChatIdAlreadyTaken("wechat-unused"))
}
