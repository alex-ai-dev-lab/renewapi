package model

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func TestLogTypeFiltersPreserveUserScopeAndPagination(t *testing.T) {
	previous := LOG_DB
	t.Cleanup(func() { LOG_DB = previous })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	require.NoError(t, db.Create(&[]Log{
		{UserId: 11, CreatedAt: 100, Type: LogTypeConsume},
		{UserId: 11, CreatedAt: 101, Type: LogTypeError},
		{UserId: 11, CreatedAt: 102, Type: LogTypeTopup},
		{UserId: 22, CreatedAt: 103, Type: LogTypeConsume},
	}).Error)
	rows, total, err := GetUserLogsWithContext(context.Background(), 11, 0, 0, 0, "", "", 0, 1, "", "", "", LogTypeConsume, LogTypeError)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 1)
	require.Equal(t, 11, rows[0].UserId)
	require.Equal(t, LogTypeError, rows[0].Type)
	rows, total, err = GetUserLogsWithContext(context.Background(), 11, 0, 0, 0, "", "", 1, 1, "", "", "", LogTypeConsume, LogTypeError)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, LogTypeConsume, rows[0].Type)
	_, total, err = GetUserLogsWithContext(context.Background(), 11, 0, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	_, total, err = GetAllLogsWithContext(context.Background(), 0, 0, 0, "", "", "", 0, 10, 0, "", "", "", LogTypeConsume, LogTypeError)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	_, total, err = GetAllLogsWithContext(context.Background(), LogTypeTopup, 0, 0, "", "", "", 0, 10, 0, "", "", "", LogTypeConsume)
	require.NoError(t, err)
	require.Zero(t, total)
}
