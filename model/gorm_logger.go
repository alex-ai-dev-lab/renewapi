package model

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
	sqlitedriver "github.com/glebarez/go-sqlite"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultSlowThresholdMS = 200
	maxSlowThresholdMS     = 60 * 60 * 1000
)

func newGormConfig(prepareStmt bool) *gorm.Config {
	return &gorm.Config{
		PrepareStmt: prepareStmt,
		Logger:      newGormLogger(os.Stdout),
	}
}

func newGormLogger(w io.Writer) logger.Interface {
	slowThresholdMS := common.GetEnvOrDefault("SQL_SLOW_THRESHOLD_MS", defaultSlowThresholdMS)
	if slowThresholdMS < 0 || slowThresholdMS > maxSlowThresholdMS {
		common.SysError(fmt.Sprintf("invalid SQL_SLOW_THRESHOLD_MS %d (allowed 0-%d), using default %d", slowThresholdMS, maxSlowThresholdMS, defaultSlowThresholdMS))
		slowThresholdMS = defaultSlowThresholdMS
	}
	return logger.New(&sanitizedLogWriter{delegate: log.New(w, "\r\n", log.LstdFlags)}, logger.Config{
		SlowThreshold:             time.Duration(slowThresholdMS) * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      !common.DebugEnabled,
		Colorful:                  true,
	})
}

type sanitizedLogWriter struct {
	delegate *log.Logger
}

func (s *sanitizedLogWriter) Printf(format string, args ...interface{}) {
	if !common.DebugEnabled {
		for i, arg := range args {
			if err, ok := arg.(error); ok {
				args[i] = sanitizeDBError(err)
			}
		}
	}
	s.delegate.Printf(format, args...)
}

func sanitizeDBError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return fmt.Errorf("mysql error %d", mysqlErr.Number)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("postgres error SQLSTATE %s", pgErr.Code)
	}
	var sqliteErr *sqlitedriver.Error
	if errors.As(err, &sqliteErr) {
		return fmt.Errorf("sqlite error %d", sqliteErr.Code())
	}
	return err
}

func SanitizeDBError(err error) error {
	return sanitizeDBError(err)
}
