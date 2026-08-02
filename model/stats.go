package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type statsQuery struct {
	logDB  *gorm.DB
	mainDB *gorm.DB
}

func newStatsQuery(ctx context.Context) statsQuery {
	if ctx == nil {
		ctx = context.Background()
	}
	return statsQuery{
		logDB:  LOG_DB.WithContext(ctx),
		mainDB: DB.WithContext(ctx),
	}
}

// OverviewStats is the high-level dashboard payload.
type OverviewStats struct {
	TotalRequests     int64         `json:"total_requests"`
	SuccessRequests   int64         `json:"success_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	SuccessRate       float64       `json:"success_rate"`
	ErrorRate         float64       `json:"error_rate"`
	RequestsPerMinute float64       `json:"requests_per_minute"`
	AvgFirstTokenTime float64       `json:"avg_first_token_time"`
	AvgUseTime        float64       `json:"avg_use_time"`
	TotalCost         float64       `json:"total_cost"`
	TotalPromptTokens int64         `json:"total_prompt_tokens"`
	TotalOutputTokens int64         `json:"total_output_tokens"`
	ActiveChannels    int64         `json:"active_channels"`
	ActiveUsers       int64         `json:"active_users"`
	Trend             []TrendPoint  `json:"trend"`
	TopChannels       []ChannelStat `json:"top_channels"`
	TopFailChannels   []ChannelStat `json:"top_failing_channels"`
	SlowestChannels   []ChannelStat `json:"slowest_channels"`
	TopModels         []ModelStat   `json:"top_models"`
	TopCostUsers      []UserStat    `json:"top_cost_users"`
}

type TrendPoint struct {
	Timestamp         int64   `json:"timestamp"`
	Requests          int64   `json:"requests"`
	Success           int64   `json:"success"`
	Failure           int64   `json:"failure"`
	SuccessRate       float64 `json:"success_rate"`
	ErrorRate         float64 `json:"error_rate"`
	AvgFirstToken     float64 `json:"avg_first_token"`
	AvgUseTime        float64 `json:"avg_use_time"`
	TotalCost         float64 `json:"total_cost"`
	TotalPromptTokens int64   `json:"total_prompt_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
}

type ChannelStat struct {
	ChannelID         int     `json:"channel_id"`
	ChannelName       string  `json:"channel_name"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	ErrorRate         float64 `json:"error_rate"`
	AvgFirstToken     float64 `json:"avg_first_token"`
	AvgUseTime        float64 `json:"avg_use_time"`
	TotalCost         float64 `json:"total_cost"`
	TotalPromptTokens int64   `json:"total_prompt_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
}

type ModelStat struct {
	ModelName         string  `json:"model_name"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	ErrorRate         float64 `json:"error_rate"`
	AvgFirstToken     float64 `json:"avg_first_token"`
	AvgUseTime        float64 `json:"avg_use_time"`
	TotalCost         float64 `json:"total_cost"`
	TotalPromptTokens int64   `json:"total_prompt_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
}

type UserStat struct {
	UserID            int     `json:"user_id"`
	Username          string  `json:"username"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	ErrorRate         float64 `json:"error_rate"`
	AvgFirstToken     float64 `json:"avg_first_token"`
	AvgUseTime        float64 `json:"avg_use_time"`
	TotalCost         float64 `json:"total_cost"`
	TotalPromptTokens int64   `json:"total_prompt_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TopChannelID      int     `json:"top_channel_id"`
	TopChannelName    string  `json:"top_channel_name"`
}

type ChannelUserStat struct {
	ChannelID         int     `json:"channel_id"`
	ChannelName       string  `json:"channel_name"`
	UserID            int     `json:"user_id"`
	Username          string  `json:"username"`
	TotalRequests     int64   `json:"total_requests"`
	SuccessRequests   int64   `json:"success_requests"`
	FailedRequests    int64   `json:"failed_requests"`
	SuccessRate       float64 `json:"success_rate"`
	ErrorRate         float64 `json:"error_rate"`
	AvgFirstToken     float64 `json:"avg_first_token"`
	AvgUseTime        float64 `json:"avg_use_time"`
	TotalCost         float64 `json:"total_cost"`
	TotalPromptTokens int64   `json:"total_prompt_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
}

func GetOverviewStats(startTime time.Time) (*OverviewStats, error) {
	return GetOverviewStatsWithContext(context.Background(), startTime)
}

func GetOverviewStatsWithContext(ctx context.Context, startTime time.Time) (*OverviewStats, error) {
	queryDB := newStatsQuery(ctx)
	stats := &OverviewStats{}
	var err error

	query := `
		SELECT
			COUNT(*) AS total_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failed_requests,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens,
			COUNT(DISTINCT CASE WHEN channel_id > 0 THEN channel_id END) AS active_channels,
			COUNT(DISTINCT CASE WHEN user_id > 0 THEN user_id END) AS active_users
		FROM logs
		WHERE type IN (?, ?)
	`
	args := []interface{}{LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}

	var avgFirstToken sql.NullFloat64
	var avgUseTime sql.NullFloat64
	var totalQuota int64
	if err := queryDB.logDB.Raw(query, args...).Row().Scan(
		&stats.TotalRequests,
		&stats.SuccessRequests,
		&stats.FailedRequests,
		&avgFirstToken,
		&avgUseTime,
		&totalQuota,
		&stats.TotalPromptTokens,
		&stats.TotalOutputTokens,
		&stats.ActiveChannels,
		&stats.ActiveUsers,
	); err != nil {
		return nil, err
	}
	stats.SuccessRate = percent(stats.SuccessRequests, stats.TotalRequests)
	stats.ErrorRate = percent(stats.FailedRequests, stats.TotalRequests)
	stats.TotalCost = quotaToUSD(totalQuota)
	stats.RequestsPerMinute, err = requestsPerMinute(queryDB.logDB, stats.TotalRequests, startTime)
	if err != nil {
		return nil, err
	}
	if avgFirstToken.Valid {
		stats.AvgFirstTokenTime = avgFirstToken.Float64
	}
	if avgUseTime.Valid {
		stats.AvgUseTime = avgUseTime.Float64
	}

	if stats.Trend, err = getTrendData(queryDB, startTime); err != nil {
		return nil, err
	}
	if stats.TopChannels, err = getTopChannels(queryDB, startTime, 10); err != nil {
		return nil, err
	}
	if stats.TopFailChannels, err = getChannelsBy(queryDB, startTime, 8, "error_rate DESC, total_requests DESC"); err != nil {
		return nil, err
	}
	if stats.SlowestChannels, err = getChannelsBy(queryDB, startTime, 8, "avg_first_token DESC, total_requests DESC"); err != nil {
		return nil, err
	}
	if stats.TopModels, err = getTopModels(queryDB, startTime, 10); err != nil {
		return nil, err
	}
	if stats.TopCostUsers, err = getUsersBy(queryDB, startTime, 8, "total_quota DESC, total_requests DESC"); err != nil {
		return nil, err
	}
	return stats, nil
}

func GetChannelStats(startTime time.Time) ([]ChannelStat, error) {
	return GetChannelStatsWithContext(context.Background(), startTime)
}

func GetChannelStatsWithContext(ctx context.Context, startTime time.Time) ([]ChannelStat, error) {
	return getTopChannels(newStatsQuery(ctx), startTime, 50)
}

func GetModelStats(startTime time.Time) ([]ModelStat, error) {
	return GetModelStatsWithContext(context.Background(), startTime)
}

func GetModelStatsWithContext(ctx context.Context, startTime time.Time) ([]ModelStat, error) {
	return getTopModels(newStatsQuery(ctx), startTime, 50)
}

func GetUserStats(startTime time.Time) ([]UserStat, error) {
	return GetUserStatsWithContext(context.Background(), startTime)
}

func GetUserStatsWithContext(ctx context.Context, startTime time.Time) ([]UserStat, error) {
	return getUsersBy(newStatsQuery(ctx), startTime, 50, "total_quota DESC, total_requests DESC")
}

func GetChannelUserStats(startTime time.Time, channelID int) ([]ChannelUserStat, error) {
	return GetChannelUserStatsWithContext(context.Background(), startTime, channelID)
}

func GetChannelUserStatsWithContext(ctx context.Context, startTime time.Time, channelID int) ([]ChannelUserStat, error) {
	if channelID <= 0 {
		return []ChannelUserStat{}, nil
	}
	queryDB := newStatsQuery(ctx)

	var stats []ChannelUserStat
	query := `
		SELECT
			channel_id,
			user_id,
			COALESCE(NULLIF(MAX(username), ''), 'Unknown') AS username,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failed_requests,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS success_rate,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS error_rate,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens
		FROM logs
		WHERE channel_id = ? AND type IN (?, ?)
	`
	args := []interface{}{LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError, channelID, LogTypeConsume, LogTypeError}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}
	query += " GROUP BY channel_id, user_id ORDER BY total_quota DESC, total_requests DESC LIMIT 100"

	rows, err := queryDB.logDB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stat ChannelUserStat
		var avgFirstToken sql.NullFloat64
		var avgUseTime sql.NullFloat64
		var totalQuota int64
		if err := rows.Scan(
			&stat.ChannelID,
			&stat.UserID,
			&stat.Username,
			&stat.TotalRequests,
			&stat.SuccessRequests,
			&stat.FailedRequests,
			&stat.SuccessRate,
			&stat.ErrorRate,
			&avgFirstToken,
			&avgUseTime,
			&totalQuota,
			&stat.TotalPromptTokens,
			&stat.TotalOutputTokens,
		); err != nil {
			return nil, err
		}
		if avgFirstToken.Valid {
			stat.AvgFirstToken = avgFirstToken.Float64
		}
		if avgUseTime.Valid {
			stat.AvgUseTime = avgUseTime.Float64
		}
		stat.TotalCost = quotaToUSD(totalQuota)
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	channelNames, err := getChannelNameMap(queryDB, []int{channelID})
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].ChannelName = channelNameOrUnknown(channelNames, stats[i].ChannelID)
	}
	return stats, nil
}

func GetChannelTrendStats(startTime time.Time, channelID int) ([]TrendPoint, error) {
	return GetChannelTrendStatsWithContext(context.Background(), startTime, channelID)
}

func GetChannelTrendStatsWithContext(ctx context.Context, startTime time.Time, channelID int) ([]TrendPoint, error) {
	if channelID <= 0 {
		return []TrendPoint{}, nil
	}
	return getTrendDataFiltered(newStatsQuery(ctx), startTime, channelID, "", 0)
}

func GetModelTrendStats(startTime time.Time, modelName string) ([]TrendPoint, error) {
	return GetModelTrendStatsWithContext(context.Background(), startTime, modelName)
}

func GetModelTrendStatsWithContext(ctx context.Context, startTime time.Time, modelName string) ([]TrendPoint, error) {
	if modelName == "" {
		return []TrendPoint{}, nil
	}
	return getTrendDataFiltered(newStatsQuery(ctx), startTime, 0, modelName, 0)
}

func GetUserTrendStats(startTime time.Time, userID int) ([]TrendPoint, error) {
	return GetUserTrendStatsWithContext(context.Background(), startTime, userID)
}

func GetUserTrendStatsWithContext(ctx context.Context, startTime time.Time, userID int) ([]TrendPoint, error) {
	if userID <= 0 {
		return []TrendPoint{}, nil
	}
	return getTrendDataFiltered(newStatsQuery(ctx), startTime, 0, "", userID)
}

func getUsersBy(queryDB statsQuery, startTime time.Time, limit int, orderBy string) ([]UserStat, error) {
	var stats []UserStat
	query := `
		SELECT
			user_id,
			COALESCE(NULLIF(MAX(username), ''), 'Unknown') AS username,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failed_requests,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS success_rate,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS error_rate,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens
		FROM logs
	`

	query += " WHERE type IN (?, ?)"
	args := []interface{}{LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}
	query += " GROUP BY user_id ORDER BY " + orderBy + " LIMIT ?"
	args = append(args, limit)

	rows, err := queryDB.logDB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stat UserStat
		var avgFirstToken sql.NullFloat64
		var avgUseTime sql.NullFloat64
		var totalQuota int64
		if err := rows.Scan(
			&stat.UserID,
			&stat.Username,
			&stat.TotalRequests,
			&stat.SuccessRequests,
			&stat.FailedRequests,
			&stat.SuccessRate,
			&stat.ErrorRate,
			&avgFirstToken,
			&avgUseTime,
			&totalQuota,
			&stat.TotalPromptTokens,
			&stat.TotalOutputTokens,
		); err != nil {
			return nil, err
		}
		if avgFirstToken.Valid {
			stat.AvgFirstToken = avgFirstToken.Float64
		}
		if avgUseTime.Valid {
			stat.AvgUseTime = avgUseTime.Float64
		}
		stat.TotalCost = quotaToUSD(totalQuota)
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	userIDs := make([]int, 0, len(stats))
	for _, stat := range stats {
		userIDs = append(userIDs, stat.UserID)
	}
	topChannels, err := getUserTopChannelIDs(queryDB, startTime, userIDs)
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].TopChannelID = topChannels[stats[i].UserID]
	}
	channelIDs := make([]int, 0, len(stats))
	for _, stat := range stats {
		if stat.TopChannelID > 0 {
			channelIDs = append(channelIDs, stat.TopChannelID)
		}
	}
	channelNames, err := getChannelNameMap(queryDB, channelIDs)
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].TopChannelName = channelNameOrUnknown(channelNames, stats[i].TopChannelID)
	}
	return stats, nil
}

func getTrendData(queryDB statsQuery, startTime time.Time) ([]TrendPoint, error) {
	return getTrendDataFiltered(queryDB, startTime, 0, "", 0)
}

func getTrendDataFiltered(queryDB statsQuery, startTime time.Time, channelID int, modelName string, userID int) ([]TrendPoint, error) {
	var trend []TrendPoint
	interval := trendIntervalSeconds(startTime)
	query := `
		SELECT
			(created_at / ?) * ? AS timestamp,
			COUNT(*) AS requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failure,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens
		FROM logs
		WHERE type IN (?, ?)
	`
	args := []interface{}{interval, interval, LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError}
	if channelID > 0 {
		query += " AND channel_id = ?"
		args = append(args, channelID)
	}
	if modelName != "" {
		query += " AND model_name = ?"
		args = append(args, modelName)
	}
	if userID > 0 {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}
	query += " GROUP BY timestamp ORDER BY timestamp ASC LIMIT 500"

	rows, err := queryDB.logDB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var point TrendPoint
		var avgFirstToken sql.NullFloat64
		var avgUseTime sql.NullFloat64
		var totalQuota int64
		if err := rows.Scan(
			&point.Timestamp,
			&point.Requests,
			&point.Success,
			&point.Failure,
			&avgFirstToken,
			&avgUseTime,
			&totalQuota,
			&point.TotalPromptTokens,
			&point.TotalOutputTokens,
		); err != nil {
			return nil, err
		}
		point.SuccessRate = percent(point.Success, point.Requests)
		point.ErrorRate = percent(point.Failure, point.Requests)
		if avgFirstToken.Valid {
			point.AvgFirstToken = avgFirstToken.Float64
		}
		if avgUseTime.Valid {
			point.AvgUseTime = avgUseTime.Float64
		}
		point.TotalCost = quotaToUSD(totalQuota)
		trend = append(trend, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trend, nil
}

func getTopChannels(queryDB statsQuery, startTime time.Time, limit int) ([]ChannelStat, error) {
	return getChannelsBy(queryDB, startTime, limit, "total_requests DESC")
}

func getChannelsBy(queryDB statsQuery, startTime time.Time, limit int, orderBy string) ([]ChannelStat, error) {
	var stats []ChannelStat
	query := `
		SELECT
			channel_id,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failed_requests,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS success_rate,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS error_rate,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens
		FROM logs
	`

	query += " WHERE type IN (?, ?)"
	args := []interface{}{LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}
	query += " GROUP BY channel_id ORDER BY " + orderBy + " LIMIT ?"
	args = append(args, limit)

	rows, err := queryDB.logDB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stat ChannelStat
		var totalQuota int64
		var avgFirstToken sql.NullFloat64
		var avgUseTime sql.NullFloat64
		if err := rows.Scan(
			&stat.ChannelID,
			&stat.TotalRequests,
			&stat.SuccessRequests,
			&stat.FailedRequests,
			&stat.SuccessRate,
			&stat.ErrorRate,
			&avgFirstToken,
			&avgUseTime,
			&totalQuota,
			&stat.TotalPromptTokens,
			&stat.TotalOutputTokens,
		); err != nil {
			return nil, err
		}
		if avgFirstToken.Valid {
			stat.AvgFirstToken = avgFirstToken.Float64
		}
		if avgUseTime.Valid {
			stat.AvgUseTime = avgUseTime.Float64
		}
		stat.TotalCost = quotaToUSD(totalQuota)
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	channelIDs := make([]int, 0, len(stats))
	for _, stat := range stats {
		channelIDs = append(channelIDs, stat.ChannelID)
	}
	channelNames, err := getChannelNameMap(queryDB, channelIDs)
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].ChannelName = channelNameOrUnknown(channelNames, stats[i].ChannelID)
	}
	return stats, nil
}

func getTopModels(queryDB statsQuery, startTime time.Time, limit int) ([]ModelStat, error) {
	var stats []ModelStat
	query := `
		SELECT
			model_name,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_requests,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS failed_requests,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS success_rate,
			CAST(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS error_rate,
			AVG(` + frtExprFor(queryDB.logDB) + `) AS avg_first_token,
			AVG(use_time) AS avg_use_time,
			COALESCE(SUM(quota), 0) AS total_quota,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_output_tokens
		FROM logs
	`

	query += " WHERE type IN (?, ?)"
	args := []interface{}{LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError, LogTypeConsume, LogTypeError}
	if !startTime.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, startTime.Unix())
	}
	query += " GROUP BY model_name ORDER BY total_requests DESC LIMIT ?"
	args = append(args, limit)

	rows, err := queryDB.logDB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stat ModelStat
		var totalQuota int64
		var avgFirstToken sql.NullFloat64
		var avgUseTime sql.NullFloat64
		if err := rows.Scan(
			&stat.ModelName,
			&stat.TotalRequests,
			&stat.SuccessRequests,
			&stat.FailedRequests,
			&stat.SuccessRate,
			&stat.ErrorRate,
			&avgFirstToken,
			&avgUseTime,
			&totalQuota,
			&stat.TotalPromptTokens,
			&stat.TotalOutputTokens,
		); err != nil {
			return nil, err
		}
		if avgFirstToken.Valid {
			stat.AvgFirstToken = avgFirstToken.Float64
		}
		if avgUseTime.Valid {
			stat.AvgUseTime = avgUseTime.Float64
		}
		stat.TotalCost = quotaToUSD(totalQuota)
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func getUserTopChannelIDs(queryDB statsQuery, startTime time.Time, userIDs []int) (map[int]int, error) {
	topChannels := make(map[int]int, len(userIDs))
	if len(userIDs) == 0 {
		return topChannels, nil
	}
	type userChannelTotal struct {
		UserID       int
		ChannelID    int
		TotalQuota   int64
		RequestCount int64
	}
	var totals []userChannelTotal
	query := queryDB.logDB.Table("logs").
		Select("user_id, channel_id, COALESCE(SUM(quota), 0) AS total_quota, COUNT(*) AS request_count").
		Where("user_id IN ?", userIDs).
		Where("type IN ?", []int{LogTypeConsume, LogTypeError})
	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime.Unix())
	}
	if err := query.Group("user_id, channel_id").
		Order("user_id ASC, total_quota DESC, request_count DESC, channel_id ASC").
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	for _, total := range totals {
		if _, exists := topChannels[total.UserID]; !exists {
			topChannels[total.UserID] = total.ChannelID
		}
	}
	return topChannels, nil
}

func getChannelNameMap(queryDB statsQuery, ids []int) (map[int]string, error) {
	names := map[int]string{}
	if len(ids) == 0 {
		return names, nil
	}
	type row struct {
		ID   int
		Name string
	}
	var rows []row
	if err := queryDB.mainDB.Table("channels").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	return names, nil
}

func channelNameOrUnknown(names map[int]string, id int) string {
	if name := names[id]; name != "" {
		return name
	}
	return "Unknown"
}

func statsBaseQuery(startTime time.Time) *gorm.DB {
	query := LOG_DB.Table("logs").Where("type IN ?", []int{LogTypeConsume, LogTypeError})
	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime.Unix())
	}
	return query
}

func frtExpr() string {
	return frtExprFor(LOG_DB)
}

func frtExprWithAlias(alias string) string {
	return frtExprForAlias(LOG_DB, alias)
}

func frtExprFor(db *gorm.DB) string {
	return frtExprForAlias(db, "")
}

func frtExprForAlias(db *gorm.DB, alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	switch db.Dialector.Name() {
	case "mysql":
		return "CASE WHEN JSON_VALID(" + prefix + "other) THEN CAST(JSON_UNQUOTE(JSON_EXTRACT(" + prefix + "other, '$.frt')) AS DECIMAL(18,3)) ELSE NULL END"
	case "postgres":
		return "CAST(SUBSTRING(" + prefix + "other FROM '\"frt\"[[:space:]]*:[[:space:]]*(-?[0-9]+([.][0-9]+)?)') AS DOUBLE PRECISION)"
	default:
		return "CASE WHEN json_valid(" + prefix + "other) THEN CAST(json_extract(" + prefix + "other, '$.frt') AS REAL) ELSE NULL END"
	}
}

func trendIntervalSeconds(startTime time.Time) int64 {
	if startTime.IsZero() {
		return 24 * 3600
	}
	diff := time.Since(startTime)
	switch {
	case diff <= 48*time.Hour:
		return 3600
	case diff <= 31*24*time.Hour:
		return 6 * 3600
	default:
		return 24 * 3600
	}
}

func quotaToUSD(quota int64) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func requestsPerMinute(logDB *gorm.DB, total int64, startTime time.Time) (float64, error) {
	if total <= 0 {
		return 0, nil
	}
	if startTime.IsZero() {
		var bounds struct {
			MinCreatedAt int64
			MaxCreatedAt int64
		}
		if err := logDB.Table("logs").
			Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
			Select("MIN(created_at) AS min_created_at, MAX(created_at) AS max_created_at").
			Scan(&bounds).Error; err != nil {
			return 0, err
		} else if bounds.MaxCreatedAt <= bounds.MinCreatedAt {
			return 0, nil
		}
		minutes := float64(bounds.MaxCreatedAt-bounds.MinCreatedAt) / 60
		if minutes <= 0 {
			return float64(total), nil
		}
		return float64(total) / minutes, nil
	}
	minutes := time.Since(startTime).Minutes()
	if minutes <= 0 {
		return float64(total), nil
	}
	return float64(total) / minutes, nil
}

func percent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}
