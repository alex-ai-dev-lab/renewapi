package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetDatabaseStatsReportsSharedPool(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(3)
	oldDB := DB
	oldLogDB := LOG_DB
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		require.NoError(t, sqlDB.Close())
	})

	stats := GetDatabaseStats()
	require.True(t, stats.LogSharedWithMain)
	require.True(t, stats.Main.Available)
	require.True(t, stats.Log.Available)
	require.Equal(t, 3, stats.Main.MaxOpenConnections)
}

func TestLogDatabasePoolConfigOverridesAndFallsBack(t *testing.T) {
	t.Setenv("SQL_MAX_OPEN_CONNS", "120")
	t.Setenv("LOG_SQL_MAX_OPEN_CONNS", "24")
	require.Equal(t, 24, getDatabaseEnvInt("LOG_SQL_MAX_OPEN_CONNS", "SQL_MAX_OPEN_CONNS", 1000))

	t.Setenv("LOG_SQL_MAX_OPEN_CONNS", "")
	require.Equal(t, 120, getDatabaseEnvInt("LOG_SQL_MAX_OPEN_CONNS", "SQL_MAX_OPEN_CONNS", 1000))
}
