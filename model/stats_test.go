package model

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetModelStatsIncludesAverageFirstToken(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: now,
		Type:      LogTypeConsume,
		ModelName: "gpt-test",
		Quota:     1000,
		Other:     `{"frt":120}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: now,
		Type:      LogTypeConsume,
		ModelName: "gpt-test",
		Quota:     1000,
		Other:     `{"frt":180}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: now,
		Type:      LogTypeError,
		ModelName: "gpt-test",
		Other:     `{"frt":300}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: now,
		Type:      LogTypeConsume,
		ModelName: "broken-json",
		Other:     ``,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		CreatedAt: now,
		Type:      LogTypeManage,
		ModelName: "gpt-test",
		Other:     `{"frt":999}`,
	}).Error)

	stats, err := GetModelStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)

	var got *ModelStat
	for i := range stats {
		if stats[i].ModelName == "gpt-test" {
			got = &stats[i]
			break
		}
	}
	require.NotNil(t, got)
	require.Equal(t, int64(3), got.TotalRequests)
	require.Equal(t, int64(2), got.SuccessRequests)
	require.Equal(t, int64(1), got.FailedRequests)
	require.InDelta(t, 66.666, got.SuccessRate, 0.01)
	require.InDelta(t, 33.333, got.ErrorRate, 0.01)
	require.InDelta(t, 200, got.AvgFirstToken, 0.001)
}

func TestGetOverviewStatsHandlesEmptyLogSet(t *testing.T) {
	truncateTables(t)

	stats, err := GetOverviewStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Zero(t, stats.TotalRequests)
	require.Zero(t, stats.SuccessRequests)
	require.Zero(t, stats.FailedRequests)
	require.Zero(t, stats.SuccessRate)
	require.Zero(t, stats.ErrorRate)
	require.Zero(t, stats.TotalCost)
	require.Zero(t, stats.TotalPromptTokens)
	require.Zero(t, stats.TotalOutputTokens)
	require.Zero(t, stats.AvgFirstTokenTime)
	require.Zero(t, stats.AvgUseTime)
	require.Zero(t, stats.ActiveChannels)
	require.Zero(t, stats.ActiveUsers)
}

func TestStatsQueryErrorsArePropagated(t *testing.T) {
	oldDB := DB
	oldLogDB := LOG_DB
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})

	db, err := gorm.Open(sqlite.Open("file:stats_error?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Log{}, &Channel{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = GetChannelStats(time.Now().Add(-time.Hour))
	require.Error(t, err)
	_, err = GetOverviewStats(time.Now().Add(-time.Hour))
	require.Error(t, err)
}

func TestGetUserStatsUsesFixedQueryCount(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	for userID := 1; userID <= 6; userID++ {
		channelID := 100 + userID
		require.NoError(t, DB.Create(&Channel{Id: channelID, Name: "channel"}).Error)
		require.NoError(t, LOG_DB.Create(&Log{
			UserId:    userID,
			Username:  "user",
			CreatedAt: now,
			Type:      LogTypeConsume,
			Quota:     userID * 100,
			ChannelId: channelID,
		}).Error)
	}

	var queryCount atomic.Int64
	callbackName := "test:stats-query-count"
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(*gorm.DB) {
		queryCount.Add(1)
	}))
	t.Cleanup(func() {
		_ = DB.Callback().Query().Remove(callbackName)
	})

	stats, err := GetUserStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 6)
	require.LessOrEqual(t, queryCount.Load(), int64(3), "query count must not grow with the number of users")
}

func TestGetOverviewStatsIncludesOperationalSignals(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&Channel{Id: 17, Name: "primary"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 18, Name: "fallback"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            1000,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          3,
		ChannelId:        17,
		Other:            `{"frt":100}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		CreatedAt:        now - 30,
		Type:             LogTypeError,
		ModelName:        "gpt-test",
		PromptTokens:     5,
		CompletionTokens: 0,
		UseTime:          9,
		ChannelId:        18,
		Other:            `{"frt":300}`,
	}).Error)

	stats, err := GetOverviewStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)

	require.Equal(t, int64(2), stats.TotalRequests)
	require.Equal(t, int64(1), stats.SuccessRequests)
	require.Equal(t, int64(1), stats.FailedRequests)
	require.InDelta(t, 50, stats.SuccessRate, 0.001)
	require.InDelta(t, 50, stats.ErrorRate, 0.001)
	require.InDelta(t, 200, stats.AvgFirstTokenTime, 0.001)
	require.InDelta(t, 6, stats.AvgUseTime, 0.001)
	require.Equal(t, int64(15), stats.TotalPromptTokens)
	require.Equal(t, int64(20), stats.TotalOutputTokens)
	require.Equal(t, int64(2), stats.ActiveChannels)
	require.Equal(t, int64(2), stats.ActiveUsers)
	require.NotEmpty(t, stats.TopChannels)
	require.NotEmpty(t, stats.TopFailChannels)
	require.NotEmpty(t, stats.SlowestChannels)
	require.NotEmpty(t, stats.TopCostUsers)
	require.NotEmpty(t, stats.Trend)

	var trendRequests int64
	var trendSuccess int64
	var trendFailure int64
	var trendPromptTokens int64
	var trendOutputTokens int64
	var sawLatency bool
	var sawCost bool
	for _, point := range stats.Trend {
		trendRequests += point.Requests
		trendSuccess += point.Success
		trendFailure += point.Failure
		trendPromptTokens += point.TotalPromptTokens
		trendOutputTokens += point.TotalOutputTokens
		sawLatency = sawLatency || point.AvgFirstToken > 0
		sawCost = sawCost || point.TotalCost > 0
		if point.Requests > 0 {
			require.InDelta(t, percent(point.Success, point.Requests), point.SuccessRate, 0.001)
			require.InDelta(t, percent(point.Failure, point.Requests), point.ErrorRate, 0.001)
		}
	}
	require.Equal(t, stats.TotalRequests, trendRequests)
	require.Equal(t, stats.SuccessRequests, trendSuccess)
	require.Equal(t, stats.FailedRequests, trendFailure)
	require.Equal(t, stats.TotalPromptTokens, trendPromptTokens)
	require.Equal(t, stats.TotalOutputTokens, trendOutputTokens)
	require.True(t, sawLatency)
	require.True(t, sawCost)
}

func TestGetChannelUserStatsAggregatesUsersForChannel(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&Channel{Id: 77, Name: "rawchat"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 88, Name: "other"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            1000,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          4,
		ChannelId:        77,
		Other:            `{"frt":100}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 30,
		Type:             LogTypeError,
		ModelName:        "gpt-test",
		PromptTokens:     5,
		CompletionTokens: 0,
		UseTime:          8,
		ChannelId:        77,
		Other:            `{"frt":300}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		CreatedAt:        now - 20,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            500,
		PromptTokens:     7,
		CompletionTokens: 9,
		UseTime:          2,
		ChannelId:        77,
		Other:            `{"frt":200}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    13,
		Username:  "charlie",
		CreatedAt: now - 10,
		Type:      LogTypeConsume,
		Quota:     9999,
		ChannelId: 88,
		Other:     `{"frt":50}`,
	}).Error)

	stats, err := GetChannelUserStats(time.Now().Add(-time.Hour), 77)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	require.Equal(t, 77, stats[0].ChannelID)
	require.Equal(t, "rawchat", stats[0].ChannelName)
	require.Equal(t, 11, stats[0].UserID)
	require.Equal(t, "alice", stats[0].Username)
	require.Equal(t, int64(2), stats[0].TotalRequests)
	require.Equal(t, int64(1), stats[0].SuccessRequests)
	require.Equal(t, int64(1), stats[0].FailedRequests)
	require.InDelta(t, 50, stats[0].SuccessRate, 0.001)
	require.InDelta(t, 50, stats[0].ErrorRate, 0.001)
	require.InDelta(t, 200, stats[0].AvgFirstToken, 0.001)
	require.InDelta(t, 6, stats[0].AvgUseTime, 0.001)
	require.Equal(t, int64(15), stats[0].TotalPromptTokens)
	require.Equal(t, int64(20), stats[0].TotalOutputTokens)

	require.Equal(t, 12, stats[1].UserID)
	require.Equal(t, int64(1), stats[1].TotalRequests)
}

func TestGetChannelTrendStatsFiltersSelectedChannel(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            1000,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          4,
		ChannelId:        77,
		Other:            `{"frt":100}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 30,
		Type:             LogTypeError,
		ModelName:        "gpt-test",
		PromptTokens:     5,
		CompletionTokens: 0,
		UseTime:          8,
		ChannelId:        77,
		Other:            `{"frt":300}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		CreatedAt:        now - 20,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            9999,
		PromptTokens:     99,
		CompletionTokens: 88,
		UseTime:          2,
		ChannelId:        88,
		Other:            `{"frt":50}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    13,
		Username:  "ignored",
		CreatedAt: now - 10,
		Type:      LogTypeManage,
		ChannelId: 77,
	}).Error)

	trend, err := GetChannelTrendStats(time.Now().Add(-time.Hour), 77)
	require.NoError(t, err)
	require.NotEmpty(t, trend)

	var requests int64
	var success int64
	var failure int64
	var promptTokens int64
	var outputTokens int64
	var sawLatency bool
	var sawCost bool
	for _, point := range trend {
		requests += point.Requests
		success += point.Success
		failure += point.Failure
		promptTokens += point.TotalPromptTokens
		outputTokens += point.TotalOutputTokens
		sawLatency = sawLatency || point.AvgFirstToken > 0
		sawCost = sawCost || point.TotalCost > 0
		if point.Requests > 0 {
			require.InDelta(t, percent(point.Success, point.Requests), point.SuccessRate, 0.001)
			require.InDelta(t, percent(point.Failure, point.Requests), point.ErrorRate, 0.001)
		}
	}
	require.Equal(t, int64(2), requests)
	require.Equal(t, int64(1), success)
	require.Equal(t, int64(1), failure)
	require.Equal(t, int64(15), promptTokens)
	require.Equal(t, int64(20), outputTokens)
	require.True(t, sawLatency)
	require.True(t, sawCost)

	empty, err := GetChannelTrendStats(time.Now().Add(-time.Hour), 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestGetModelTrendStatsFiltersSelectedModel(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            1000,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          4,
		ChannelId:        77,
		Other:            `{"frt":100}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 30,
		Type:             LogTypeError,
		ModelName:        "gpt-test",
		PromptTokens:     5,
		CompletionTokens: 0,
		UseTime:          8,
		ChannelId:        88,
		Other:            `{"frt":300}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		CreatedAt:        now - 20,
		Type:             LogTypeConsume,
		ModelName:        "claude-test",
		Quota:            9999,
		PromptTokens:     99,
		CompletionTokens: 88,
		UseTime:          2,
		ChannelId:        77,
		Other:            `{"frt":50}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    13,
		Username:  "ignored",
		CreatedAt: now - 10,
		Type:      LogTypeManage,
		ModelName: "gpt-test",
	}).Error)

	trend, err := GetModelTrendStats(time.Now().Add(-time.Hour), "gpt-test")
	require.NoError(t, err)
	require.NotEmpty(t, trend)

	var requests int64
	var success int64
	var failure int64
	var promptTokens int64
	var outputTokens int64
	var sawLatency bool
	var sawCost bool
	for _, point := range trend {
		requests += point.Requests
		success += point.Success
		failure += point.Failure
		promptTokens += point.TotalPromptTokens
		outputTokens += point.TotalOutputTokens
		sawLatency = sawLatency || point.AvgFirstToken > 0
		sawCost = sawCost || point.TotalCost > 0
		if point.Requests > 0 {
			require.InDelta(t, percent(point.Success, point.Requests), point.SuccessRate, 0.001)
			require.InDelta(t, percent(point.Failure, point.Requests), point.ErrorRate, 0.001)
		}
	}
	require.Equal(t, int64(2), requests)
	require.Equal(t, int64(1), success)
	require.Equal(t, int64(1), failure)
	require.Equal(t, int64(15), promptTokens)
	require.Equal(t, int64(20), outputTokens)
	require.True(t, sawLatency)
	require.True(t, sawCost)

	empty, err := GetModelTrendStats(time.Now().Add(-time.Hour), "")
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestGetUserTrendStatsFiltersSelectedUser(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            1000,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          4,
		ChannelId:        77,
		Other:            `{"frt":100}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           11,
		Username:         "alice",
		CreatedAt:        now - 30,
		Type:             LogTypeError,
		ModelName:        "gpt-test",
		PromptTokens:     5,
		CompletionTokens: 0,
		UseTime:          8,
		ChannelId:        88,
		Other:            `{"frt":300}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		CreatedAt:        now - 20,
		Type:             LogTypeConsume,
		ModelName:        "gpt-test",
		Quota:            9999,
		PromptTokens:     99,
		CompletionTokens: 88,
		UseTime:          2,
		ChannelId:        77,
		Other:            `{"frt":50}`,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    11,
		Username:  "ignored",
		CreatedAt: now - 10,
		Type:      LogTypeManage,
		ModelName: "gpt-test",
	}).Error)

	trend, err := GetUserTrendStats(time.Now().Add(-time.Hour), 11)
	require.NoError(t, err)
	require.NotEmpty(t, trend)

	var requests int64
	var success int64
	var failure int64
	var promptTokens int64
	var outputTokens int64
	var sawLatency bool
	var sawCost bool
	for _, point := range trend {
		requests += point.Requests
		success += point.Success
		failure += point.Failure
		promptTokens += point.TotalPromptTokens
		outputTokens += point.TotalOutputTokens
		sawLatency = sawLatency || point.AvgFirstToken > 0
		sawCost = sawCost || point.TotalCost > 0
		if point.Requests > 0 {
			require.InDelta(t, percent(point.Success, point.Requests), point.SuccessRate, 0.001)
			require.InDelta(t, percent(point.Failure, point.Requests), point.ErrorRate, 0.001)
		}
	}
	require.Equal(t, int64(2), requests)
	require.Equal(t, int64(1), success)
	require.Equal(t, int64(1), failure)
	require.Equal(t, int64(15), promptTokens)
	require.Equal(t, int64(20), outputTokens)
	require.True(t, sawLatency)
	require.True(t, sawCost)

	empty, err := GetUserTrendStats(time.Now().Add(-time.Hour), 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}
