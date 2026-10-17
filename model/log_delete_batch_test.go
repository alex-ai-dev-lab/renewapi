package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeleteOldLogBoundsEveryTransaction(t *testing.T) {
	setupAuditLogQueries(t)
	db := LOG_DB
	rows := make([]Log, 205)
	for i := range rows {
		rows[i].CreatedAt = 10
	}
	require.NoError(t, db.Create(&rows).Error)
	require.NoError(t, db.Create(&Log{CreatedAt: 20}).Error)
	var batches []int64
	require.NoError(t, db.Callback().Delete().After("gorm:delete").Register("audit:batch", func(tx *gorm.DB) { batches = append(batches, tx.RowsAffected) }))
	deleted, err := DeleteOldLog(context.Background(), 20, 100)
	require.NoError(t, err)
	require.EqualValues(t, 205, deleted)
	require.Equal(t, []int64{100, 100, 5}, batches)
	var count int64
	require.NoError(t, db.Model(&Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = DeleteOldLog(ctx, 30, 100)
	require.ErrorIs(t, err, context.Canceled)
	_, err = DeleteOldLog(context.Background(), 30, -1)
	require.Error(t, err)
}
