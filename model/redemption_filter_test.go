package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func TestRedemptionStatusFilterKeepsExpiryAndPaginationConsistent(t *testing.T) {
	previous := DB
	t.Cleanup(func() { DB = previous })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&Redemption{}))
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&[]Redemption{
		{Key: "a", Name: "qa-enabled", Status: 1},
		{Key: "b", Name: "qa-expired", Status: 1, ExpiredTime: now - 100},
		{Key: "c", Name: "qa-disabled", Status: 2, ExpiredTime: now - 100},
		{Key: "d", Name: "qa-used", Status: 3, ExpiredTime: now - 100},
	}).Error)
	for _, status := range []string{"1", "2", "3", "expired"} {
		rows, total, err := SearchRedemptions("qa-", 0, 10, status)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, rows, 1)
	}
	rows, total, err := SearchRedemptions("qa-", 0, 1, "2,expired")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 1)
	_, total, err = SearchRedemptions("qa-", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	_, _, err = SearchRedemptions("", 0, 10, "1 OR 1=1")
	require.Error(t, err)
}
