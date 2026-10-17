package model

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLogCursorBidirectionalSnapshotAndDeletedAnchor(t *testing.T) {
	setupAuditLogQueries(t)
	for i := 1; i <= 7; i++ {
		require.NoError(t, LOG_DB.Create(&Log{Id: i, UserId: 71, CreatedAt: 100, Type: LogTypeConsume}).Error)
	}
	filter := LogQueryFilter{ViewerID: 1, LogTypes: []int{LogTypeConsume}}
	page, err := GetLogCursorPage(context.Background(), filter, "", 3)
	require.NoError(t, err)
	require.Equal(t, []int{7, 6, 5}, cursorLogIDs(page))
	require.Empty(t, page.PreviousCursor)
	// Backdated inserts must not leak into this snapshot; deleting the anchor
	// must not make the next query depend on looking that row up again.
	require.NoError(t, LOG_DB.Create(&Log{Id: 8, CreatedAt: 99, Type: LogTypeConsume}).Error)
	require.NoError(t, LOG_DB.Where("id = ?", 5).Delete(&Log{}).Error)
	next, err := GetLogCursorPage(context.Background(), filter, page.NextCursor, 3)
	require.NoError(t, err)
	require.Equal(t, []int{4, 3, 2}, cursorLogIDs(next))
	back, err := GetLogCursorPage(context.Background(), filter, next.PreviousCursor, 3)
	require.NoError(t, err)
	require.Equal(t, []int{7, 6}, cursorLogIDs(back))
	require.Empty(t, back.PreviousCursor)
	last, err := GetLogCursorPage(context.Background(), filter, next.NextCursor, 3)
	require.NoError(t, err)
	require.Equal(t, []int{1}, cursorLogIDs(last))
	require.Empty(t, last.NextCursor)
}

func TestLogCursorSelfScopeFiltersAndSanitization(t *testing.T) {
	setupAuditLogQueries(t)
	for i := 1; i <= 8; i++ {
		require.NoError(t, LOG_DB.Create(&Log{Id: i, UserId: 71 + i%2, ModelName: "audit", TokenName: "token", Type: LogTypeConsume, UpstreamRequestId: "private", Other: `{"admin_info":{"secret":"private"}}`}).Error)
	}
	filter := LogQueryFilter{ViewerID: 71, UserID: 71, ModelName: "audit", TokenName: "token"}
	page, err := GetLogCursorPage(context.Background(), filter, "", 2)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	for _, row := range page.Items {
		require.Equal(t, 71, row.UserId)
		require.Empty(t, row.UpstreamRequestId)
		require.NotContains(t, row.Other, "private")
	}
	next, err := GetLogCursorPage(context.Background(), filter, page.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, next.Items, 2)
	require.Empty(t, next.NextCursor)
	for _, change := range []LogQueryFilter{
		{ViewerID: 72, UserID: 72, ModelName: "audit", TokenName: "token"},
		{ViewerID: 71, UserID: 71, ModelName: "different", TokenName: "token"},
		{ViewerID: 71, UserID: 0, ModelName: "audit", TokenName: "token"},
	} {
		_, err := GetLogCursorPage(context.Background(), change, page.NextCursor, 2)
		require.ErrorIs(t, err, ErrInvalidLogCursor)
	}
	_, err = GetLogCursorPage(context.Background(), filter, page.NextCursor, 3)
	require.ErrorIs(t, err, ErrInvalidLogCursor)
	_, err = GetLogCursorPage(context.Background(), filter, "!"+page.NextCursor[1:], 2)
	require.ErrorIs(t, err, ErrInvalidLogCursor)
}

func TestLogCursorSkipsCountAndOffsetAndLegacyRejectsDeepPages(t *testing.T) {
	setupAuditLogQueries(t)
	for i := 1; i <= 12; i++ {
		require.NoError(t, LOG_DB.Create(&Log{Id: i, UserId: 71, CreatedAt: int64(i / 3)}).Error)
	}
	var statements []string
	require.NoError(t, LOG_DB.Callback().Query().After("gorm:query").Register("test:cursor-sql", func(tx *gorm.DB) { statements = append(statements, strings.ToLower(tx.Statement.SQL.String())) }))
	t.Cleanup(func() { _ = LOG_DB.Callback().Query().Remove("test:cursor-sql") })
	filter := LogQueryFilter{ViewerID: 71, UserID: 71}
	first, err := GetLogCursorPage(context.Background(), filter, "", 3)
	require.NoError(t, err)
	_, err = GetLogCursorPage(context.Background(), filter, first.NextCursor, 3)
	require.NoError(t, err)
	require.NotEmpty(t, statements)
	for _, sql := range statements {
		require.NotContains(t, sql, "offset")
		require.NotContains(t, sql, "count(")
	}
	_, _, err = GetUserLogsWithContext(context.Background(), 71, 0, 0, 0, "", "", 50000, 20, "", "", "")
	require.ErrorIs(t, err, ErrLogOffsetLimit)
	_, _, err = GetAllLogsWithContext(context.Background(), 0, 0, 0, "", "", "", 50000, 20, 0, "", "", "")
	require.ErrorIs(t, err, ErrLogOffsetLimit)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = GetLogCursorPage(ctx, filter, "", 3)
	require.ErrorIs(t, err, context.Canceled)
}

func cursorLogIDs(page *LogCursorPage) []int {
	ids := make([]int, len(page.Items))
	for i, row := range page.Items {
		ids[i] = row.Id
	}
	return ids
}

func TestLogCursorCrossesTimestampGroupsInBothDirections(t *testing.T) {
	setupAuditLogQueries(t)
	expected := make([]Log, 31)
	for i := range expected {
		expected[i] = Log{Id: i + 1, CreatedAt: int64(i % 5), UserId: 71}
	}
	require.NoError(t, LOG_DB.Create(&expected).Error)
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].CreatedAt == expected[j].CreatedAt {
			return expected[i].Id > expected[j].Id
		}
		return expected[i].CreatedAt > expected[j].CreatedAt
	})
	filter := LogQueryFilter{ViewerID: 1}
	var cursor string
	for offset := 0; offset < len(expected); offset += 4 {
		page, err := GetLogCursorPage(context.Background(), filter, cursor, 4)
		require.NoError(t, err)
		end := min(offset+4, len(expected))
		for i, row := range page.Items {
			require.Equal(t, expected[offset+i].Id, row.Id)
		}
		require.Len(t, page.Items, end-offset)
		if offset > 0 {
			back, err := GetLogCursorPage(context.Background(), filter, page.PreviousCursor, 4)
			require.NoError(t, err)
			require.Len(t, back.Items, 4)
			for i, row := range back.Items {
				require.Equal(t, expected[offset-4+i].Id, row.Id)
			}
		}
		cursor = page.NextCursor
	}
	require.Empty(t, cursor)
}
