// Package sqlite provides SQLite tuning for production workloads.
package sqlite

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const defaultBusyTimeoutMilliseconds = 120000

// WithConnectionPragmas adds SQLite pragmas to the DSN so every pooled
// connection receives the same settings. Executing PRAGMA through a gorm.DB
// only affects the connection that happens to be checked out at that moment.
func WithConnectionPragmas(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator +
		"_pragma=busy_timeout(" + strconv.Itoa(defaultBusyTimeoutMilliseconds) + ")&" +
		"_pragma=journal_mode(WAL)&" +
		"_pragma=synchronous(NORMAL)"
}

// TunePragmas applies production-ready SQLite settings.
// Call this after opening a SQLite database but before use.
func TunePragmas(db *gorm.DB) error {
	return tunePragmas(
		db,
		common.GetEnvOrDefault("SQLITE_MAX_IDLE_CONNS", 1),
		common.GetEnvOrDefault("SQLITE_MAX_OPEN_CONNS", 1),
	)
}

// TuneLogPragmas configures the log database separately from the primary
// database. Log/stat queries are read-heavy and should not queue behind the
// primary database's channel and user writes.
func TuneLogPragmas(db *gorm.DB) error {
	return tunePragmas(
		db,
		common.GetEnvOrDefault("SQLITE_LOG_MAX_IDLE_CONNS", 4),
		common.GetEnvOrDefault("SQLITE_LOG_MAX_OPEN_CONNS", 4),
	)
}

func tunePragmas(db *gorm.DB, maxIdleConns, maxOpenConns int) error {
	if db == nil {
		return nil
	}
	if maxOpenConns < 1 {
		maxOpenConns = 1
	}
	if maxIdleConns < 1 {
		maxIdleConns = 1
	}
	if maxIdleConns > maxOpenConns {
		maxIdleConns = maxOpenConns
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// The log pool can run independent WAL readers while the primary pool is
	// handling channel/user writes. The DSN-level pragmas above keep lock
	// handling consistent for connections created after this function returns.
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	// WAL mode for better concurrency (readers don't block writers)
	_ = db.Exec("PRAGMA journal_mode=WAL").Error

	// synchronous=NORMAL is safe with WAL and faster than FULL
	_ = db.Exec("PRAGMA synchronous=NORMAL").Error

	// busy_timeout: wait up to two minutes for a short-lived writer lock.
	_ = db.Exec("PRAGMA busy_timeout=" + strconv.Itoa(defaultBusyTimeoutMilliseconds)).Error

	return nil
}
