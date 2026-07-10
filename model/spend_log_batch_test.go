package model

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordConsumeLogBatchWritesLogs(t *testing.T) {
	truncateTables(t)
	oldBatchEnabled := os.Getenv("SPEND_LOG_BATCH_ENABLED")
	oldBatchSize := os.Getenv("SPEND_LOG_BATCH_SIZE")
	oldBatchInterval := os.Getenv("SPEND_LOG_BATCH_INTERVAL_MS")
	oldDataExport := common.IsDataExportEnabled()
	t.Cleanup(func() {
		_ = os.Setenv("SPEND_LOG_BATCH_ENABLED", oldBatchEnabled)
		_ = os.Setenv("SPEND_LOG_BATCH_SIZE", oldBatchSize)
		_ = os.Setenv("SPEND_LOG_BATCH_INTERVAL_MS", oldBatchInterval)
		common.SetDataExportEnabled(oldDataExport)
		ResetSpendLogBatchForTest()
	})
	require.NoError(t, os.Setenv("SPEND_LOG_BATCH_ENABLED", "true"))
	require.NoError(t, os.Setenv("SPEND_LOG_BATCH_SIZE", "2"))
	require.NoError(t, os.Setenv("SPEND_LOG_BATCH_INTERVAL_MS", "20"))
	common.SetDataExportEnabled(false)
	ResetSpendLogBatchForTest()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("username", "alice")
	ctx.Set(common.RequestIdKey, "req-1")

	RecordConsumeLog(ctx, 11, RecordConsumeLogParams{
		ChannelId:        7,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "gpt-test",
		TokenName:        "key-a",
		Quota:            100,
		TokenId:          3,
	})
	RecordConsumeLog(ctx, 11, RecordConsumeLogParams{
		ChannelId:        7,
		PromptTokens:     1,
		CompletionTokens: 2,
		ModelName:        "gpt-test",
		TokenName:        "key-a",
		Quota:            10,
		TokenId:          3,
	})

	FlushSpendLogBatchForTest(time.Second)

	var logs []Log
	require.NoError(t, LOG_DB.Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, LogTypeConsume, logs[0].Type)
	require.Equal(t, 100, logs[0].Quota)
	require.Equal(t, 10, logs[1].Quota)
	require.Equal(t, "req-1", logs[0].RequestId)
}
