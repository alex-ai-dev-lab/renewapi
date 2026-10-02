package model

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestDashboardFlowAndTokensUseRealScopedConsumption(t *testing.T) {
	priorLog, priorDB := LOG_DB, DB
	t.Cleanup(func() { LOG_DB = priorLog; DB = priorDB })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &Channel{}))
	LOG_DB = db
	DB = db
	require.NoError(t, db.Create(&Channel{Id: 8, Name: "upstream"}).Error)
	now := time.Unix(1800001801, 0)
	require.NoError(t, db.Create(&[]Log{
		{UserId: 11, Username: "reader", CreatedAt: now.Unix() - 1, Type: LogTypeConsume, TokenId: 3, TokenName: "client", Group: "default", ChannelId: 8, ModelName: "model-a", PromptTokens: 10, CompletionTokens: 5, Quota: 100},
		{UserId: 11, Username: "reader", CreatedAt: now.Unix() - 3601, Type: LogTypeConsume, TokenId: 3, TokenName: "client", Group: "default", ChannelId: 8, ModelName: "model-a", PromptTokens: 20, CompletionTokens: 10, Quota: 200},
		{UserId: 11, Username: "reader", CreatedAt: now.Unix() - 1, Type: LogTypeError, PromptTokens: 999, Quota: 999},
		{UserId: 22, Username: "other", CreatedAt: now.Unix() - 1, Type: LogTypeConsume, PromptTokens: 1000, Quota: 1000},
	}).Error)
	rows, err := GetQuotaFlow(context.Background(), 11, now.Unix()-86400, now.Unix(), "other")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 2, rows[0].Count)
	require.EqualValues(t, 45, rows[0].TokenUsed)
	require.EqualValues(t, 300, rows[0].Quota)
	require.Zero(t, rows[0].ChannelID)
	require.Equal(t, "client", rows[0].TokenName)
	admin, err := GetQuotaFlow(context.Background(), 0, now.Unix()-86400, now.Unix(), "reader")
	require.NoError(t, err)
	require.Len(t, admin, 1)
	require.Equal(t, "upstream", admin[0].ChannelName)
	summary, err := GetAccountTokenUsage(context.Background(), 11, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, summary.TotalTokens)
	require.EqualValues(t, 45, summary.Last24hTokens)
	require.Len(t, summary.Hourly, 2)
	for _, point := range summary.Hourly {
		require.Zero(t, point.Timestamp%3600)
	}
}
