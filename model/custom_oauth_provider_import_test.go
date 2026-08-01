package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestCustomOAuthProvider(slug string) *CustomOAuthProvider {
	return &CustomOAuthProvider{
		Name:                  slug,
		Slug:                  slug,
		ClientId:              "client-id",
		ClientSecret:          "client-secret",
		AuthorizationEndpoint: "https://issuer.example.com/authorize",
		TokenEndpoint:         "https://issuer.example.com/token",
		UserInfoEndpoint:      "https://issuer.example.com/userinfo",
	}
}

func TestImportCustomOAuthProvidersIsAtomicAndPreservesExistingSecret(t *testing.T) {
	previousDB := DB
	t.Cleanup(func() { DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomOAuthProvider{}))
	DB = db

	existing := newTestCustomOAuthProvider("existing")
	require.NoError(t, DB.Create(existing).Error)

	invalidNewProvider := newTestCustomOAuthProvider("new-provider")
	invalidNewProvider.ClientSecret = ""
	_, err = ImportCustomOAuthProviders([]*CustomOAuthProvider{
		{
			Name:                  "updated",
			Slug:                  "existing",
			ClientId:              "new-client-id",
			AuthorizationEndpoint: existing.AuthorizationEndpoint,
			TokenEndpoint:         existing.TokenEndpoint,
			UserInfoEndpoint:      existing.UserInfoEndpoint,
		},
		invalidNewProvider,
	})
	require.Error(t, err)

	var afterFailure CustomOAuthProvider
	require.NoError(t, DB.First(&afterFailure, "slug = ?", "existing").Error)
	require.Equal(t, "existing", afterFailure.Name)
	require.Equal(t, "client-secret", afterFailure.ClientSecret)
	var count int64
	require.NoError(t, DB.Model(&CustomOAuthProvider{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	results, err := ImportCustomOAuthProviders([]*CustomOAuthProvider{
		{
			Name:                  "updated",
			Slug:                  "existing",
			ClientId:              "new-client-id",
			AuthorizationEndpoint: existing.AuthorizationEndpoint,
			TokenEndpoint:         existing.TokenEndpoint,
			UserInfoEndpoint:      existing.UserInfoEndpoint,
		},
		newTestCustomOAuthProvider("new-provider"),
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.False(t, results[0].Created)
	require.True(t, results[1].Created)

	var updated CustomOAuthProvider
	require.NoError(t, DB.First(&updated, "slug = ?", "existing").Error)
	require.Equal(t, "updated", updated.Name)
	require.Equal(t, "client-secret", updated.ClientSecret)
	require.NoError(t, DB.Model(&CustomOAuthProvider{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
