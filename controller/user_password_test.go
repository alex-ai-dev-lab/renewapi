package controller

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCheckUpdatePasswordRequiresExistingPassword(t *testing.T) {
	oldDB := model.DB
	t.Cleanup(func() { model.DB = oldDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db

	passwordless := model.User{Username: "oauth-user", Password: "", AffCode: "oauth"}
	require.NoError(t, db.Create(&passwordless).Error)
	update, err := checkUpdatePassword("", "NewPassword123", passwordless.Id)
	require.False(t, update)
	require.ErrorIs(t, err, errUserPasswordUnset)

	hash, err := common.Password2Hash("OldPassword123")
	require.NoError(t, err)
	withPassword := model.User{Username: "password-user", Password: hash, AffCode: "password"}
	require.NoError(t, db.Create(&withPassword).Error)

	update, err = checkUpdatePassword("wrong", "NewPassword123", withPassword.Id)
	require.False(t, update)
	require.ErrorIs(t, err, errOriginalPasswordFail)

	update, err = checkUpdatePassword("OldPassword123", "NewPassword123", withPassword.Id)
	require.NoError(t, err)
	require.True(t, update)

	update, err = checkUpdatePassword("", "", 0)
	require.NoError(t, err)
	require.False(t, update)
}

func TestUpdateSelfPasswordPreservesAccountFields(t *testing.T) {
	oldDB, oldRedis := model.DB, common.RedisEnabled
	t.Cleanup(func() { model.DB = oldDB; common.RedisEnabled = oldRedis })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RedisEnabled = false
	hash, err := common.Password2Hash("OldPassword123")
	require.NoError(t, err)
	user := model.User{Username: "profile-user", DisplayName: "Keep my name", Password: hash, AffCode: "profile-code", Role: 10, Status: 1, Group: "default", Email: "qa@example.test", Quota: 123456, Setting: "{}"}
	require.NoError(t, db.Create(&user).Error)
	payload, err := common.Marshal(map[string]interface{}{"original_password": "OldPassword123", "password": "NewPassword123", "role": 100, "quota": 999999})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", user.Id)
	ctx.Request = httptest.NewRequest("PUT", "/api/user/self", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateSelf(ctx)
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	require.True(t, result.Success, result.Message)
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	require.Equal(t, user.Username, after.Username)
	require.Equal(t, user.DisplayName, after.DisplayName)
	require.Equal(t, user.Role, after.Role)
	require.Equal(t, user.Status, after.Status)
	require.Equal(t, user.Group, after.Group)
	require.Equal(t, user.Email, after.Email)
	require.Equal(t, user.Quota, after.Quota)
	require.Equal(t, user.AffCode, after.AffCode)
	require.Equal(t, user.Setting, after.Setting)
	require.True(t, common.ValidatePasswordAndHash("NewPassword123", after.Password))
}
