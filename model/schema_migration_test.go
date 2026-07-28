package model

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunSchemaMigrationOnceSkipsAfterSuccess(t *testing.T) {
	oldDB := DB
	oldUsingSQLite := common.UsingSQLite
	t.Cleanup(func() {
		DB = oldDB
		common.UsingSQLite = oldUsingSQLite
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true

	runs := 0
	migrate := func() error {
		runs++
		return nil
	}

	require.NoError(t, runSchemaMigrationOnce("test:once:v1", migrate))
	require.NoError(t, runSchemaMigrationOnce("test:once:v1", migrate))
	require.Equal(t, 1, runs)
	require.True(t, DB.Migrator().HasTable(&SchemaMigration{}))
}

func TestCheckSchemaMigrationsDoesNotCreateTables(t *testing.T) {
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db

	err = CheckSchemaMigrations()
	require.ErrorIs(t, err, ErrSchemaMigrationsPending)
	require.False(t, DB.Migrator().HasTable(&SchemaMigration{}), "startup checks must not execute DDL")
	require.False(t, DB.Migrator().HasTable(&schemaMigrationLock{}), "startup checks must not create lock tables")
}

func TestSchemaMigrationChecksumMismatchIsRejected(t *testing.T) {
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, ensureSchemaMigrationsTable())

	def := mainSchemaMigrationDefinitions()[0]
	require.NoError(t, DB.Create(&SchemaMigration{
		Key:        def.Key,
		Checksum:   "wrong-checksum",
		AppVersion: "old",
		AppliedAt:  time.Now().Unix(),
	}).Error)

	err = runSchemaMigration(def)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checksum mismatch")
}

func TestSchemaMigrationLockRejectsConcurrentOwner(t *testing.T) {
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, ensureSchemaMigrationLockTable())
	now := time.Now().Unix()
	require.NoError(t, DB.Create(&schemaMigrationLock{
		Name:       "main",
		Owner:      "other-instance",
		AcquiredAt: now,
		ExpiresAt:  now + 60,
	}).Error)

	err = withSchemaMigrationLock(func() error {
		t.Fatal("migration body must not run while another owner holds the lease")
		return nil
	})
	require.True(t, errors.Is(err, ErrSchemaMigrationLocked))
}

func TestLegacyMigrationMetadataIsUpgradedWithoutRerun(t *testing.T) {
	oldDB := DB
	oldVersion := common.Version
	t.Cleanup(func() {
		DB = oldDB
		common.Version = oldVersion
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.Version = "test-version"
	require.NoError(t, DB.Exec(`CREATE TABLE schema_migrations (
		migration_key varchar(191) PRIMARY KEY,
		applied_at bigint NOT NULL
	)`).Error)
	require.NoError(t, DB.Exec("INSERT INTO schema_migrations (migration_key, applied_at) VALUES (?, ?)", "legacy:v1", time.Now().Unix()).Error)

	runs := 0
	require.NoError(t, runSchemaMigrationOnce("legacy:v1", func() error {
		runs++
		return nil
	}))
	require.Zero(t, runs)
	var record SchemaMigration
	require.NoError(t, DB.Where("migration_key = ?", "legacy:v1").First(&record).Error)
	require.NotEmpty(t, record.Checksum)
	require.Equal(t, "test-version", record.AppVersion)
}
