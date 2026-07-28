package model

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSanitizeDBErrorStripsDriverMessage(t *testing.T) {
	tests := []struct {
		name, want, leaked string
		err                error
	}{
		{"mysql", "mysql error 1062", "secret-value", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'secret-value'"}},
		{"postgres", "postgres error SQLSTATE 23505", "secret-value", &pgconn.PgError{Code: "23505", Detail: "secret-value"}},
		{"wrapped", "mysql error 1064", "secret-value", fmt.Errorf("exec failed: %w", &mysql.MySQLError{Number: 1064, Message: "secret-value"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := sanitizeDBError(test.err)
			require.EqualError(t, got, test.want)
			require.NotContains(t, got.Error(), test.leaked)
		})
	}
}

func TestGormLoggerEndToEndSanitizedOutput(t *testing.T) {
	previousDebug := common.DebugEnabled
	t.Cleanup(func() { common.DebugEnabled = previousDebug })
	execQuery := func() string {
		var buf bytes.Buffer
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: newGormLogger(&buf)})
		require.NoError(t, err)
		db.Exec("SELECT * FROM missing_table WHERE email = ?", "secret@example.com")
		return buf.String()
	}
	common.DebugEnabled = false
	out := execQuery()
	require.Contains(t, out, "email = ?")
	require.NotContains(t, out, "secret@example.com")
	require.Contains(t, out, "sqlite error")
	common.DebugEnabled = true
	require.Contains(t, execQuery(), "secret@example.com")
}
