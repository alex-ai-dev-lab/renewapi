package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSumUsedQuotaFiltersRequestIDs(t *testing.T) {
	oldLogDB := LOG_DB
	t.Cleanup(func() { LOG_DB = oldLogDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db

	now := time.Now().Unix()
	logs := []Log{
		{CreatedAt: now, Type: LogTypeConsume, Quota: 100, PromptTokens: 10, CompletionTokens: 5, RequestId: "request-a", UpstreamRequestId: "upstream-a"},
		{CreatedAt: now, Type: LogTypeConsume, Quota: 200, PromptTokens: 20, CompletionTokens: 10, RequestId: "request-a", UpstreamRequestId: "upstream-b"},
		{CreatedAt: now, Type: LogTypeConsume, Quota: 400, PromptTokens: 40, CompletionTokens: 20, RequestId: "request-b", UpstreamRequestId: "upstream-a"},
	}
	require.NoError(t, db.Create(&logs).Error)

	byRequest, err := SumUsedQuota(LogTypeUnknown, 0, 0, "", "", "", 0, "", "request-a", "")
	require.NoError(t, err)
	require.Equal(t, Stat{Quota: 300, Rpm: 2, Tpm: 45}, byRequest)

	byUpstream, err := SumUsedQuota(LogTypeUnknown, 0, 0, "", "", "", 0, "", "", "upstream-a")
	require.NoError(t, err)
	require.Equal(t, Stat{Quota: 500, Rpm: 2, Tpm: 75}, byUpstream)

	byBoth, err := SumUsedQuota(LogTypeUnknown, 0, 0, "", "", "", 0, "", "request-a", "upstream-a")
	require.NoError(t, err)
	require.Equal(t, Stat{Quota: 100, Rpm: 1, Tpm: 15}, byBoth)
}
