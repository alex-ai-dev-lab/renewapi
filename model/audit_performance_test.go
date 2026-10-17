package model

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Evidence uses synthetic data and reports latency; it has no machine-specific SLA.
func TestAuditQueryPerformanceEvidence(t *testing.T) {
	setupAuditLogQueries(t)
	now := time.Now().Unix()
	rows := make([]Log, 100000)
	for i := range rows {
		rows[i] = Log{UserId: 71, CreatedAt: now - int64(i%600)*86400, Type: LogTypeConsume, ChannelId: i % 5, ModelName: "audit", Other: `{}`}
	}
	require.NoError(t, LOG_DB.CreateInBatches(rows, 100).Error)
	for _, filter := range []LogQueryFilter{{ViewerID: 71, UserID: 71}, {ViewerID: 1}} {
		scope, err := logCursorScope(filter, 20)
		require.NoError(t, err)
		var anchors []Log
		order := "id DESC"
		if filter.UserID == 0 {
			order = "created_at DESC, id DESC"
		}
		require.NoError(t, LOG_DB.Order(order).Offset(49999).Limit(1).Find(&anchors).Error)
		require.Len(t, anchors, 1)
		anchor := anchors[0]
		position := logCursor{Version: 1, Scope: scope, SnapshotID: rows[len(rows)-1].Id, ID: anchor.Id, CreatedAt: anchor.CreatedAt}
		deep, err := encodeLogCursor(position)
		require.NoError(t, err)
		position.Newer = true
		back, err := encodeLogCursor(position)
		require.NoError(t, err)
		for _, sample := range []struct{ name, cursor string }{{"first", ""}, {"position_50000", deep}, {"back_from_50000", back}} {
			samples := make([]float64, 21)
			for i := range samples {
				started := time.Now()
				// Normalize ten reads per sample to avoid sub-millisecond timer
				// quantization on Windows. These are batch averages, not an SLA.
				for range 10 {
					page, err := GetLogCursorPage(context.Background(), filter, sample.cursor, 20)
					require.NoError(t, err)
					require.Len(t, page.Items, 20)
				}
				samples[i] = float64(time.Since(started).Nanoseconds()) / float64(time.Millisecond) / 10
			}
			sort.Float64s(samples)
			t.Logf("cursor_logs rows=100000 batch=10 self=%t page=%s p50_ms=%.3f p95_ms=%.3f", filter.UserID > 0, sample.name, samples[10], samples[19])
		}
	}

	samples := make([]float64, 7)
	for i := range samples {
		started := time.Now()
		stats, err := GetOverviewStats(time.Time{})
		require.NoError(t, err)
		require.EqualValues(t, 100000, stats.TotalRequests)
		samples[i] = float64(time.Since(started).Microseconds()) / 1000
	}
	sort.Float64s(samples)
	t.Logf("overview rows=100000 p50_ms=%.3f p95_ms=%.3f", samples[3], samples[6])
}
