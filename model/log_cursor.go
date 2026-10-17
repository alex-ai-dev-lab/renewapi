package model

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrLogOffsetLimit = errors.New("deep log pages require cursor pagination")

var ErrInvalidLogCursor = errors.New("log cursor is invalid or filters changed; return to the latest logs")

// LogQueryFilter is shared by cursor and legacy pagination. ViewerID binds a
// cursor to the authenticated viewer; UserID additionally restricts self scope.
type LogQueryFilter struct {
	ViewerID, UserID, LogType, Channel  int
	LogTypes                            []int
	StartTimestamp, EndTimestamp        int64
	ModelName, Username, TokenName      string
	Group, RequestID, UpstreamRequestID string
}

func logQuery(ctx context.Context, filter LogQueryFilter) (*gorm.DB, error) {
	tx := LOG_DB.WithContext(ctx).Model(&Log{})
	if filter.UserID > 0 {
		tx = tx.Where("logs.user_id = ?", filter.UserID)
	}
	if filter.LogType != LogTypeUnknown {
		tx = tx.Where("logs.type = ?", filter.LogType)
	}
	if len(filter.LogTypes) > 0 {
		tx = tx.Where("logs.type IN ?", filter.LogTypes)
	}
	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", filter.ModelName); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", filter.Username); err != nil {
		return nil, err
	}
	for _, value := range []struct{ column, value string }{
		{"logs.token_name", filter.TokenName}, {"logs.request_id", filter.RequestID},
		{"logs.upstream_request_id", filter.UpstreamRequestID}, {"logs." + logGroupCol, filter.Group},
	} {
		if value.value != "" {
			tx = tx.Where(value.column+" = ?", value.value)
		}
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", filter.EndTimestamp)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	return tx, nil
}

type LogCursorPage struct {
	Pagination     string `json:"pagination"`
	Items          []*Log `json:"items"`
	PageSize       int    `json:"page_size"`
	NextCursor     string `json:"next_cursor,omitempty"`
	PreviousCursor string `json:"previous_cursor,omitempty"`
}

type logCursor struct {
	Version    int
	Scope      string
	SnapshotID int
	ID         int
	CreatedAt  int64
	Newer      bool
}

// AEAD keeps real IDs private (self logs expose only page-local row numbers)
// and rejects edits. Cursors are positions, never authorization credentials.
func logCursorCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("renewapi:log-cursor:v1:" + common.CryptoSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encodeLogCursor(value logCursor) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	aead, err := logCursorCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, data, []byte("logs:v1"))), nil
}

func decodeLogCursor(token string, scope string) (logCursor, error) {
	var value logCursor
	if len(token) > 2048 {
		return value, ErrInvalidLogCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return value, ErrInvalidLogCursor
	}
	aead, err := logCursorCipher()
	if err != nil {
		return value, err
	}
	if len(data) < aead.NonceSize()+aead.Overhead() {
		return value, ErrInvalidLogCursor
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte("logs:v1"))
	if err != nil || common.Unmarshal(plain, &value) != nil || value.Version != 1 || value.Scope != scope || value.ID <= 0 || value.SnapshotID < value.ID {
		return logCursor{}, ErrInvalidLogCursor
	}
	return value, nil
}

func logCursorBoundary(tx *gorm.DB, self bool, value logCursor) *gorm.DB {
	op := "<"
	if value.Newer {
		op = ">"
	}
	if self {
		if value.Newer {
			return tx.Where("logs.id > ? AND logs.id <= ?", value.ID, value.SnapshotID)
		}
		// ID < anchor already implies ID <= snapshot. A second upper bound
		// can make SQLite seek from the snapshot and scan down to the anchor.
		return tx.Where("logs.id "+op+" ?", value.ID)
	}
	tx = tx.Where("logs.id <= ?", value.SnapshotID)
	if tx.Dialector.Name() != "mysql" {
		// Row comparison seeks both index columns, including large groups of
		// logs written in the same second. MySQL 5.7 needs the expanded range.
		return tx.Where("(logs.created_at, logs.id) "+op+" (?, ?)", value.CreatedAt, value.ID)
	}
	return tx.Where("logs.created_at "+op+"= ?", value.CreatedAt).Where("(logs.created_at "+op+" ? OR (logs.created_at = ? AND logs.id "+op+" ?))", value.CreatedAt, value.CreatedAt, value.ID)
}

func GetLogCursorPage(ctx context.Context, filter LogQueryFilter, token string, size int) (*LogCursorPage, error) {
	if size < 1 || size > 100 || filter.ViewerID <= 0 {
		return nil, ErrInvalidLogCursor
	}
	scope, err := logCursorScope(filter, size)
	if err != nil {
		return nil, err
	}
	position := logCursor{Version: 1, Scope: scope}
	if token != "" {
		position, err = decodeLogCursor(token, scope)
		if err != nil {
			return nil, err
		}
	} else {
		var ids []int
		if err := LOG_DB.WithContext(ctx).Model(&Log{}).Order("id DESC").Limit(1).Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			position.SnapshotID = ids[0]
		}
	}
	return getLogCursorPage(ctx, filter, position, token != "", size)
}

func logCursorScope(filter LogQueryFilter, size int) (string, error) {
	scopeBytes, err := common.Marshal(struct {
		Filter LogQueryFilter
		Size   int
	}{filter, size})
	if err != nil {
		return "", err
	}
	scopeHash := sha256.Sum256(scopeBytes)
	return base64.RawURLEncoding.EncodeToString(scopeHash[:]), nil
}

func getLogCursorPage(ctx context.Context, filter LogQueryFilter, position logCursor, hasCursor bool, size int) (*LogCursorPage, error) {
	base, err := logQuery(ctx, filter)
	if err != nil {
		return nil, err
	}
	query := base.Session(&gorm.Session{})
	self := filter.UserID > 0
	if hasCursor {
		query = logCursorBoundary(query, self, position)
	} else {
		query = query.Where("logs.id <= ?", position.SnapshotID)
	}
	direction := "DESC"
	if position.Newer {
		direction = "ASC"
	}
	order := "logs.id " + direction
	if !self {
		order = "logs.created_at " + direction + ", " + order
	}
	page := &LogCursorPage{Pagination: "cursor", PageSize: size, Items: []*Log{}}
	if err := query.Order(order).Limit(size + 1).Find(&page.Items).Error; err != nil {
		return nil, err
	}
	more := len(page.Items) > size
	if more {
		page.Items = page.Items[:size]
	}
	if position.Newer {
		slices.Reverse(page.Items)
	}
	if len(page.Items) == 0 {
		// Deletion may empty a bookmarked page. Its newer boundary still lets
		// the viewer leave it; the UI also offers an unconditional latest reset.
		if hasCursor {
			position.Newer = true
			page.PreviousCursor, err = encodeLogCursor(position)
		}
		return page, err
	}
	first, last := page.Items[0], page.Items[len(page.Items)-1]
	boundary := func(row *Log, newer bool) logCursor {
		return logCursor{Version: 1, Scope: position.Scope, SnapshotID: position.SnapshotID, ID: row.Id, CreatedAt: row.CreatedAt, Newer: newer}
	}
	hasNext, hasPrevious := more, false
	if hasCursor {
		opposite := boundary(first, true)
		if position.Newer {
			opposite = boundary(last, false)
		}
		var ids []int
		if err := logCursorBoundary(base.Session(&gorm.Session{}), self, opposite).Select("logs.id").Limit(1).Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		if position.Newer {
			hasPrevious, hasNext = more, len(ids) > 0
		} else {
			hasPrevious = len(ids) > 0
		}
	}
	if hasNext {
		page.NextCursor, err = encodeLogCursor(boundary(last, false))
		if err != nil {
			return nil, err
		}
	}
	if hasPrevious {
		page.PreviousCursor, err = encodeLogCursor(boundary(first, true))
		if err != nil {
			return nil, err
		}
	}
	if self {
		formatUserLogs(page.Items, 0)
	} else if err := enrichLogChannels(ctx, page.Items); err != nil {
		return nil, err
	}
	return page, nil
}

// Legacy clients keep the response shape within a bounded page/count window.
// Cursor clients do not execute COUNT or OFFSET at all.
func countLogsBounded(ctx context.Context, tx *gorm.DB) (int64, error) {
	bounded := tx.Session(&gorm.Session{}).Select("logs.id").Limit(LogSearchCountLimit + 1)
	var total int64
	err := LOG_DB.WithContext(ctx).Table("(?) AS matched_logs", bounded).Count(&total).Error
	return total, err
}
