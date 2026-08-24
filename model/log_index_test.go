package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyLogIndex struct {
	ID        int   `gorm:"index:idx_created_at_id,priority:1"`
	CreatedAt int64 `gorm:"index:idx_created_at_id,priority:2"`
}

func (legacyLogIndex) TableName() string {
	return "logs"
}

func findLogCreatedAtIDIndexColumns(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	columns, _, err := getLogCreatedAtIDIndexColumns(db)
	require.NoError(t, err)
	return columns
}

func TestEnsureLogCreatedAtIDIndexRebuildsLegacyOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyLogIndex{}))
	require.Equal(t, []string{"id", "created_at"}, findLogCreatedAtIDIndexColumns(t, db))

	require.NoError(t, ensureLogCreatedAtIDIndex(db))
	require.Equal(t, []string{"created_at", "id"}, findLogCreatedAtIDIndexColumns(t, db))

	// A second startup must recognize the corrected index and avoid another rebuild.
	require.NoError(t, ensureLogCreatedAtIDIndex(db))
	require.Equal(t, []string{"created_at", "id"}, findLogCreatedAtIDIndexColumns(t, db))
}

func TestGetAllLogsOrdersByCreatedAtThenID(t *testing.T) {
	oldLogDB := LOG_DB
	t.Cleanup(func() { LOG_DB = oldLogDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	require.NoError(t, db.Create(&Log{CreatedAt: 200, Content: "same-time-older-id"}).Error)
	require.NoError(t, db.Create(&Log{CreatedAt: 100, Content: "older-time"}).Error)
	require.NoError(t, db.Create(&Log{CreatedAt: 200, Content: "same-time-newer-id"}).Error)

	logs, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Equal(t, []string{"same-time-newer-id", "same-time-older-id", "older-time"}, []string{
		logs[0].Content,
		logs[1].Content,
		logs[2].Content,
	})
}

func TestDeleteOldLogRespectsCutoffAndPreservesNewerRows(t *testing.T) {
	oldLogDB := LOG_DB
	t.Cleanup(func() { LOG_DB = oldLogDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db

	logs := []Log{
		{CreatedAt: 10, Content: "old-1"},
		{CreatedAt: 20, Content: "old-2"},
		{CreatedAt: 30, Content: "old-3"},
		{CreatedAt: 40, Content: "old-4"},
		{CreatedAt: 50, Content: "old-5"},
		{CreatedAt: 100, Content: "cutoff"},
		{CreatedAt: 101, Content: "newer"},
	}
	require.NoError(t, db.Create(&logs).Error)

	deleted, err := DeleteOldLog(context.Background(), 100, 2)
	require.NoError(t, err)
	require.Equal(t, int64(5), deleted)

	var remaining []Log
	require.NoError(t, db.Order("created_at asc").Find(&remaining).Error)
	require.Equal(t, []string{"cutoff", "newer"}, []string{
		remaining[0].Content,
		remaining[1].Content,
	})
}
