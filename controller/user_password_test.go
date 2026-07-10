package controller

import (
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
