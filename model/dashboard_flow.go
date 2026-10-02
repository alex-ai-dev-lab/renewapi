package model

import (
	"context"
	"errors"
	"time"
)

// FlowQuotaRow is a read-only aggregation of existing consumption logs. It
// introduces no subscription, routing or billing behavior.
type FlowQuotaRow struct {
	UserID      int    `json:"user_id,omitempty"`
	Username    string `json:"username,omitempty"`
	TokenID     int    `json:"token_id,omitempty"`
	TokenName   string `json:"token_name,omitempty"`
	UseGroup    string `json:"use_group,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	ModelName   string `json:"model_name"`
	TokenUsed   int64  `json:"token_used"`
	Count       int64  `json:"count"`
	Quota       int64  `json:"quota"`
}

func GetQuotaFlow(ctx context.Context, userID int, start, end int64, username string) ([]FlowQuotaRow, error) {
	query := LOG_DB.WithContext(ctx).Model(&Log{}).Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, start, end)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	} else if username != "" {
		var err error
		query, err = applyExplicitLogTextFilter(query, "username", username)
		if err != nil {
			return nil, err
		}
	}
	// Group quoting follows the same database-specific convention as log lists.
	fields := "user_id, username, token_id, token_name, " + logGroupCol + ", channel_id, model_name"
	selection := "user_id, username, token_id, token_name, " + logGroupCol + " AS use_group, channel_id, model_name, SUM(prompt_tokens + completion_tokens) AS token_used, COUNT(*) AS count, SUM(quota) AS quota"
	if userID > 0 {
		fields = "user_id, username, token_id, token_name, " + logGroupCol + ", model_name"
		selection = "user_id, username, token_id, token_name, " + logGroupCol + " AS use_group, model_name, SUM(prompt_tokens + completion_tokens) AS token_used, COUNT(*) AS count, SUM(quota) AS quota"
	}
	rows := []FlowQuotaRow{}
	if err := query.Select(selection).Group(fields).Limit(10001).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 10000 {
		return nil, errors.New("too many flow groups; choose a shorter time range")
	}
	if userID > 0 {
		for i := range rows {
			rows[i].ChannelID = 0
		}
		return rows, nil
	}
	ids := []int{}
	for _, row := range rows {
		if row.ChannelID > 0 {
			ids = append(ids, row.ChannelID)
		}
	}
	if len(ids) > 0 {
		var channels []struct {
			ID   int
			Name string
		}
		if err := DB.WithContext(ctx).Table("channels").Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err != nil {
			return nil, err
		}
		names := map[int]string{}
		for _, channel := range channels {
			names[channel.ID] = channel.Name
		}
		for i := range rows {
			rows[i].ChannelName = names[rows[i].ChannelID]
		}
	}
	return rows, nil
}

type TokenUsagePoint struct {
	Timestamp int64 `json:"timestamp"`
	Tokens    int64 `json:"tokens"`
}
type TokenUsageSummary struct {
	TotalTokens   int64             `json:"total_tokens"`
	Last24hTokens int64             `json:"last_24h_tokens"`
	Hourly        []TokenUsagePoint `json:"hourly"`
}

func GetAccountTokenUsage(ctx context.Context, userID int, now time.Time) (TokenUsageSummary, error) {
	summary := TokenUsageSummary{Hourly: []TokenUsagePoint{}}
	query := LOG_DB.WithContext(ctx).Model(&Log{}).Where("user_id = ? AND type = ?", userID, LogTypeConsume)
	if err := query.Select("COALESCE(SUM(prompt_tokens + completion_tokens), 0)").Scan(&summary.TotalTokens).Error; err != nil {
		return summary, err
	}
	// Integer hour buckets work in SQLite, MySQL and PostgreSQL without relying
	// on a dialect-specific date function.
	since := now.Unix() - 86400
	var points []TokenUsagePoint
	bucket := "created_at - (created_at % 3600)"
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).Where("user_id = ? AND type = ? AND created_at >= ? AND created_at <= ?", userID, LogTypeConsume, since, now.Unix()).Select(bucket + " AS timestamp, SUM(prompt_tokens + completion_tokens) AS tokens").Group(bucket).Order("timestamp").Scan(&points).Error; err != nil {
		return summary, err
	}
	for _, point := range points {
		summary.Last24hTokens += point.Tokens
	}
	summary.Hourly = points
	if summary.Hourly == nil {
		summary.Hourly = []TokenUsagePoint{}
	}
	return summary, nil
}
