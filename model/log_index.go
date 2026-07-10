package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const logCreatedAtIDIndexName = "idx_created_at_id"

func ensureLogCreatedAtIDIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("log database is nil")
	}
	if !db.Migrator().HasTable(&Log{}) {
		return nil
	}

	columns, actualName, err := getLogCreatedAtIDIndexColumns(db)
	if err != nil {
		return fmt.Errorf("inspect log indexes: %w", err)
	}
	if len(columns) == 2 && strings.EqualFold(columns[0], "created_at") && strings.EqualFold(columns[1], "id") {
		return nil
	}
	if actualName != "" {
		if err := db.Migrator().DropIndex(&Log{}, actualName); err != nil {
			return fmt.Errorf("drop stale log index %s: %w", actualName, err)
		}
	}

	if err := db.Migrator().CreateIndex(&Log{}, logCreatedAtIDIndexName); err != nil {
		return fmt.Errorf("create log index %s: %w", logCreatedAtIDIndexName, err)
	}
	return nil
}

func getLogCreatedAtIDIndexColumns(db *gorm.DB) ([]string, string, error) {
	type columnRow struct {
		ColumnName string `gorm:"column:column_name"`
	}
	var rows []columnRow
	var err error
	switch db.Dialector.Name() {
	case "sqlite":
		err = db.Raw("SELECT name AS column_name FROM pragma_index_info(?) ORDER BY seqno", logCreatedAtIDIndexName).Scan(&rows).Error
	case "mysql":
		err = db.Raw(`SELECT COLUMN_NAME AS column_name
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'logs' AND INDEX_NAME = ?
ORDER BY SEQ_IN_INDEX`, logCreatedAtIDIndexName).Scan(&rows).Error
	case "postgres":
		err = db.Raw(`SELECT attribute.attname AS column_name
FROM pg_class AS table_class
JOIN pg_index AS index_meta ON table_class.oid = index_meta.indrelid
JOIN pg_class AS index_class ON index_class.oid = index_meta.indexrelid
JOIN LATERAL unnest(index_meta.indkey) WITH ORDINALITY AS key(attnum, position) ON true
JOIN pg_attribute AS attribute ON attribute.attrelid = table_class.oid AND attribute.attnum = key.attnum
WHERE table_class.oid = to_regclass('logs') AND index_class.relname = ?
ORDER BY key.position`, logCreatedAtIDIndexName).Scan(&rows).Error
	default:
		indexes, getErr := db.Migrator().GetIndexes(&Log{})
		if getErr != nil {
			return nil, "", getErr
		}
		for _, index := range indexes {
			if strings.EqualFold(index.Name(), logCreatedAtIDIndexName) {
				return index.Columns(), index.Name(), nil
			}
		}
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	columns := make([]string, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, row.ColumnName)
	}
	actualName := ""
	if len(columns) > 0 {
		actualName = logCreatedAtIDIndexName
	}
	return columns, actualName, nil
}
