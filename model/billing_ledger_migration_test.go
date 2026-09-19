package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// billingLedgerV1Fixture mirrors the pre-RU-A table. Keeping the fixture local
// proves that v2 adds columns to deployed data instead of only creating a new
// table from the current model.
type billingLedgerV1Fixture struct {
	ID             uint64 `gorm:"primaryKey"`
	RequestID      string `gorm:"type:varchar(64);not null;uniqueIndex"`
	Kind           string `gorm:"type:varchar(24);not null;default:'request';index"`
	Mode           string `gorm:"type:varchar(16);not null;index"`
	State          string `gorm:"type:varchar(32);not null;index"`
	DesiredState   string `gorm:"type:varchar(32);index"`
	FundingSource  string `gorm:"type:varchar(24);not null"`
	UserID         int    `gorm:"not null;index"`
	TokenID        int    `gorm:"index"`
	ChannelID      int    `gorm:"index"`
	SubscriptionID int    `gorm:"index"`
	ReservedQuota  int64  `gorm:"not null;default:0"`
	ActualQuota    int64  `gorm:"not null;default:0"`
	AppliedQuota   int64  `gorm:"not null;default:0"`
	DesiredQuota   int64  `gorm:"not null;default:0"`
	CountedQuota   int64  `gorm:"not null;default:0"`
	RequestCounted bool   `gorm:"not null;default:false"`
	TokenUnlimited bool   `gorm:"not null;default:false"`
	Playground     bool   `gorm:"not null;default:false"`
	Version        int64  `gorm:"not null;default:1"`
	Attempts       int    `gorm:"not null;default:0"`
	NextRetryAt    int64  `gorm:"not null;default:0;index"`
	LastError      string `gorm:"type:text"`
	CreatedAt      int64  `gorm:"not null;index"`
	UpdatedAt      int64  `gorm:"not null;index"`
}

func (billingLedgerV1Fixture) TableName() string { return "billing_ledgers" }

func TestMigrateBillingLedgerV2SQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:billing-ledger-v2?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	assertBillingLedgerV2Migration(t, db, true)
}

func TestMigrateBillingLedgerV2ExternalDatabase(t *testing.T) {
	driver := os.Getenv("REQUEST_GUARD_TEST_DRIVER")
	dsn := os.Getenv("REQUEST_GUARD_TEST_DSN")
	if driver == "" || dsn == "" {
		t.Skip("set REQUEST_GUARD_TEST_DRIVER and REQUEST_GUARD_TEST_DSN for MySQL/PostgreSQL integration")
	}

	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	default:
		t.Fatalf("unsupported driver %q", driver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	assertBillingLedgerV2Migration(t, db, false)
}

func assertBillingLedgerV2Migration(t *testing.T, db *gorm.DB, sqliteDB bool) {
	t.Helper()
	previousDB := DB
	previousSQLite := common.UsingSQLite
	DB = db
	common.UsingSQLite = sqliteDB
	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousSQLite
		_ = db.Migrator().DropTable(&BillingLedger{})
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.Migrator().DropTable(&BillingLedger{}))
	require.NoError(t, db.AutoMigrate(&billingLedgerV1Fixture{}))
	legacy := billingLedgerV1Fixture{
		RequestID: "legacy-shadow-ledger", Kind: "request", Mode: "shadow",
		State: BillingLedgerStateReserved, FundingSource: "wallet", UserID: 1,
		TokenID: 1, ReservedQuota: 400, AppliedQuota: 400, DesiredQuota: 400,
		Version: 1, CreatedAt: 1, UpdatedAt: 1,
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, migrateBillingLedgerV2())
	for _, column := range []string{"funding_state", "token_state", "subscription_state", "statistics_state"} {
		require.True(t, db.Migrator().HasColumn(&BillingLedger{}, column), column)
	}
	var migrated BillingLedger
	require.NoError(t, db.Where("request_id = ?", legacy.RequestID).First(&migrated).Error)
	require.EqualValues(t, 400, migrated.AppliedQuota)
	require.Equal(t, BillingComponentStatePending, migrated.FundingState)

	// A pre-v2 shadow row already had its initial reservation applied by the
	// legacy path. The additive fields must not cause that reservation to be
	// replayed, and a later balance-leg failure must still roll back on every
	// supported database.
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &BillingOutbox{}, &Task{}, &Midjourney{}))
	user := User{Id: legacy.UserID, Username: "legacy-ledger-user", Password: "password", Quota: 600}
	token := Token{Id: legacy.TokenID, UserId: legacy.UserID, Key: "sk-legacy-ledger", Status: common.TokenStatusEnabled, RemainQuota: 600, UsedQuota: 400}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&token).Error)

	require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Update("remain_quota", 0).Error)
	_, settleErr := SettleBillingLedger(migrated.ID, 500)
	require.Error(t, settleErr)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.EqualValues(t, 600, user.Quota)
	require.NoError(t, db.First(&migrated, migrated.ID).Error)
	require.Equal(t, BillingLedgerStateReserved, migrated.State)
	require.Equal(t, BillingComponentStatePending, migrated.FundingState)

	require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Update("remain_quota", 100).Error)
	settled, err := SettleBillingLedger(migrated.ID, 500)
	require.NoError(t, err)
	require.Equal(t, BillingLedgerStateSettled, settled.State)
	require.Equal(t, BillingComponentStateApplied, settled.FundingState)
	require.Equal(t, BillingComponentStateApplied, settled.TokenState)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	require.EqualValues(t, 500, user.Quota)
	require.EqualValues(t, 0, token.RemainQuota)
	require.EqualValues(t, 500, token.UsedQuota)
}
