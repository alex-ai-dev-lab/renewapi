package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserHasPasswordDistinguishesPasswordlessAccounts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))

	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	passwordless := User{Username: "oauth-user", Password: "", AffCode: "oauth"}
	require.NoError(t, db.Create(&passwordless).Error)
	has, err := UserHasPassword(passwordless.Id)
	require.NoError(t, err)
	require.False(t, has)

	withPassword := User{Username: "password-user", Password: "stored-hash", AffCode: "password"}
	require.NoError(t, db.Create(&withPassword).Error)
	has, err = UserHasPassword(withPassword.Id)
	require.NoError(t, err)
	require.True(t, has)
}
