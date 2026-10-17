package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAuditTokenStatusFilterSharesPaginationAndCount(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	for i := 0; i < 25; i++ {
		status := common.TokenStatusEnabled
		if i%3 == 0 {
			status = common.TokenStatusDisabled
		}
		require.NoError(t, DB.Create(&Token{UserId: fixture.user.Id, Key: common.GetRandomString(40), Name: "filter", Status: status}).Error)
	}
	statuses := []int{common.TokenStatusDisabled}
	total, err := CountUserTokens(fixture.user.Id, statuses...)
	require.NoError(t, err)
	require.EqualValues(t, 9, total)
	rows, err := GetAllUserTokens(fixture.user.Id, 5, 5, statuses...)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	search, found, err := SearchUserTokens(fixture.user.Id, "filter", "", 5, 5, statuses...)
	require.NoError(t, err)
	require.Equal(t, total, found)
	require.Len(t, search, 4)
	for _, row := range search {
		require.Equal(t, common.TokenStatusDisabled, row.Status)
	}
}

func TestAuditAllTimeTrendCovers600DaysAndOverviewAggregatesOnce(t *testing.T) {
	setupAuditLogQueries(t)
	var logs []Log
	now := time.Now().Unix()
	for i := 0; i < 600; i++ {
		logs = append(logs, Log{CreatedAt: now - int64(i)*86400, Type: LogTypeConsume, ChannelId: 7, UserId: 1, ModelName: "audit", Other: `{"frt":123.5}`})
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)
	var aggregates int
	require.NoError(t, LOG_DB.Callback().Row().Before("gorm:row").Register("test:audit-group", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "GROUP BY channel_id ORDER BY") {
			aggregates++
		}
	}))
	t.Cleanup(func() { _ = LOG_DB.Callback().Row().Remove("test:audit-group") })
	stats, err := GetOverviewStats(time.Time{})
	require.NoError(t, err)
	require.Equal(t, 1, aggregates)
	require.InDelta(t, 123.5, stats.AvgFirstTokenTime, 0.001)
	require.LessOrEqual(t, len(stats.Trend), 500)
	var total int64
	for _, point := range stats.Trend {
		total += point.Requests
	}
	require.EqualValues(t, 600, total)
	require.GreaterOrEqual(t, stats.Trend[len(stats.Trend)-1].Timestamp, now-2*86400)
}

func TestAuditUserLogBoundedCountKeepsFullRows(t *testing.T) {
	setupAuditLogQueries(t)
	rows := make([]Log, LogSearchCountLimit+10)
	for i := range rows {
		rows[i] = Log{UserId: 71, CreatedAt: int64(i), Type: LogTypeConsume, Content: "preserved"}
	}
	require.NoError(t, LOG_DB.CreateInBatches(rows, 100).Error)
	logs, total, err := GetUserLogsWithContext(context.Background(), 71, 0, 0, 0, "", "", 0, 5, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, LogSearchCountLimit+1, total)
	require.Len(t, logs, 5)
	require.Equal(t, 71, logs[0].UserId)
	require.Equal(t, "preserved", logs[0].Content)
}

// The same cases run against BILLING_TEST_DRIVER / BILLING_TEST_DSN in an
// explicitly disposable database, including the log database connection.
func setupAuditLogQueries(t *testing.T) {
	setupBillingLedgerTest(t)
	require.NoError(t, LOG_DB.Migrator().DropTable(&Log{}))
	require.NoError(t, LOG_DB.AutoMigrate(&Log{}))
	t.Cleanup(func() { _ = LOG_DB.Migrator().DropTable(&Log{}) })
}
