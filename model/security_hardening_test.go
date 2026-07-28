package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSecurityHardeningDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldRedis := common.RedisEnabled
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		DB, common.RedisEnabled = oldDB, oldRedis
		_ = sqlDB.Close()
	})
	sqlDB.SetMaxOpenConns(20)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &TwoFA{}, &TwoFABackupCode{}, &PasskeyCredential{}, &UserOAuthBinding{}))
	DB = db
	common.RedisEnabled = false
	initCol()
}

func TestBackupCodeCanOnlyBeConsumedOnceConcurrently(t *testing.T) {
	setupSecurityHardeningDB(t)
	const code = "ABCD-1234"
	hash, err := common.HashBackupCode(code)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&TwoFABackupCode{UserId: 7, CodeHash: hash}).Error)

	const attempts = 20
	results := make(chan bool, attempts)
	errorsSeen := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			valid, err := ValidateBackupCode(7, code)
			if err != nil {
				errorsSeen <- err
			}
			results <- valid
		}()
	}
	wg.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		require.NoError(t, err)
	}
	successes := 0
	for valid := range results {
		if valid {
			successes++
		}
	}
	require.Equal(t, 1, successes)
}

func TestHardDeleteRemovesAuthenticationArtifacts(t *testing.T) {
	setupSecurityHardeningDB(t)
	user := User{Username: "delete-me", Password: "password123", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&Token{UserId: user.Id, Key: "delete-token"}).Error)
	require.NoError(t, DB.Create(&TwoFA{UserId: user.Id, Secret: "secret"}).Error)
	require.NoError(t, DB.Create(&TwoFABackupCode{UserId: user.Id, CodeHash: "hash"}).Error)
	require.NoError(t, DB.Create(&PasskeyCredential{UserID: user.Id, CredentialID: "credential", PublicKey: "key"}).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 1, ProviderUserId: "remote-user"}).Error)

	require.NoError(t, HardDeleteUserById(user.Id))
	for _, model := range []interface{}{&User{}, &Token{}, &TwoFA{}, &TwoFABackupCode{}, &PasskeyCredential{}, &UserOAuthBinding{}} {
		var count int64
		require.NoError(t, DB.Unscoped().Model(model).Count(&count).Error)
		require.Zero(t, count, "%T was not deleted", model)
	}
}
