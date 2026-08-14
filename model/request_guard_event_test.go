package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrateRequestGuardEventsSQLite(t *testing.T) {
	previousDB := DB
	previousSQLite := common.UsingSQLite
	db, err := gorm.Open(sqlite.Open("file:requestguard-migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousSQLite
	})

	require.NoError(t, migrateRequestGuardEventsV1())
	require.True(t, DB.Migrator().HasTable(&RequestGuardEvent{}))
	for _, index := range []string{
		"idx_request_guard_created",
		"idx_request_guard_request_id",
		"idx_request_guard_user_created",
		"idx_request_guard_decision_created",
	} {
		require.True(t, DB.Migrator().HasIndex(&RequestGuardEvent{}, index), index)
	}
	for _, column := range []string{"request_id", "request_group", "categories_text", "prompt_hmac", "redacted_preview", "created_at"} {
		require.True(t, DB.Migrator().HasColumn(&RequestGuardEvent{}, column), column)
	}

	require.NoError(t, CreateRequestGuardEvent(&RequestGuardEvent{RequestID: "req-1", Decision: "block", CategoriesText: "[]", CreatedAt: 1}))
	events, err := ListRequestGuardEvents(0, 10, "block")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "req-1", events[0].RequestID)
}

func TestMigrateRequestGuardEventsExternalDatabase(t *testing.T) {
	driver := os.Getenv("REQUEST_GUARD_TEST_DRIVER")
	dsn := os.Getenv("REQUEST_GUARD_TEST_DSN")
	if driver == "" || dsn == "" {
		t.Skip("set REQUEST_GUARD_TEST_DRIVER and REQUEST_GUARD_TEST_DSN for MySQL/PostgreSQL integration")
	}

	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	default:
		t.Fatalf("unsupported driver %q", driver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&RequestGuardEvent{}))
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&RequestGuardEvent{})
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, migrateRequestGuardEventsV1())
	require.True(t, db.Migrator().HasTable(&RequestGuardEvent{}))
	for _, index := range []string{
		"idx_request_guard_created",
		"idx_request_guard_request_id",
		"idx_request_guard_user_created",
		"idx_request_guard_decision_created",
	} {
		require.True(t, db.Migrator().HasIndex(&RequestGuardEvent{}, index), index)
	}

	require.NoError(t, CreateRequestGuardEvent(&RequestGuardEvent{
		RequestID: "requestguard-external-" + driver, Decision: "block", CategoriesText: "[]", CreatedAt: 1,
	}))
	events, err := ListRequestGuardEvents(0, 10, "block")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "requestguard-external-"+driver, events[0].RequestID)
}
