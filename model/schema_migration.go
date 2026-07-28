package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	errSchemaMigrationNotApplied = errors.New("schema migration not applied")
	ErrSchemaMigrationsPending   = errors.New("schema migrations are pending")
	ErrSchemaMigrationLocked     = errors.New("schema migration is locked by another instance")
)

type SchemaMigration struct {
	Key        string `gorm:"primaryKey;size:191;column:migration_key"`
	Checksum   string `gorm:"size:64;not null;default:''"`
	AppVersion string `gorm:"size:64;not null;default:''"`
	AppliedAt  int64  `gorm:"not null"`
	DurationMS int64  `gorm:"not null;default:0"`
}

type schemaMigrationLock struct {
	Name       string `gorm:"primaryKey;size:191;column:lock_name"`
	Owner      string `gorm:"size:191;not null"`
	AcquiredAt int64  `gorm:"not null"`
	ExpiresAt  int64  `gorm:"not null"`
}

type schemaMigrationDefinition struct {
	Key      string
	Revision string
	Apply    func() error
}

type SchemaMigrationStatus struct {
	Key             string
	Checksum        string
	Applied         bool
	AppliedChecksum string
	AppVersion      string
	AppliedAt       int64
	DurationMS      int64
}

func (schemaMigrationLock) TableName() string {
	return "schema_migration_locks"
}

func ensureSchemaMigrationsTable() error {
	if !DB.Migrator().HasTable(&SchemaMigration{}) {
		return DB.AutoMigrate(&SchemaMigration{})
	}
	columns := []struct {
		field string
		ddl   string
	}{
		{field: "Checksum", ddl: "checksum text NOT NULL DEFAULT ''"},
		{field: "AppVersion", ddl: "app_version varchar(64) NOT NULL DEFAULT ''"},
		{field: "DurationMS", ddl: "duration_ms bigint NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		if DB.Migrator().HasColumn(&SchemaMigration{}, column.field) {
			continue
		}
		if common.UsingSQLite || DB.Dialector.Name() == "sqlite" {
			if err := DB.Exec("ALTER TABLE schema_migrations ADD COLUMN " + column.ddl).Error; err != nil {
				return err
			}
		} else if err := DB.Migrator().AddColumn(&SchemaMigration{}, column.field); err != nil {
			return err
		}
	}
	return nil
}

func ensureSchemaMigrationLockTable() error {
	return DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migration_locks (
		lock_name varchar(191) PRIMARY KEY,
		owner varchar(191) NOT NULL,
		acquired_at bigint NOT NULL,
		expires_at bigint NOT NULL
	)`).Error
}

func schemaMigrationChecksum(def schemaMigrationDefinition) string {
	sum := sha256.Sum256([]byte(def.Key + "\n" + def.Revision))
	return fmt.Sprintf("%x", sum[:])
}

func runSchemaMigrationOnce(key string, migrate func() error) error {
	return runSchemaMigration(schemaMigrationDefinition{Key: key, Revision: "legacy-v1", Apply: migrate})
}

func runSchemaMigration(def schemaMigrationDefinition) error {
	if def.Key == "" {
		return errors.New("schema migration key is empty")
	}
	if def.Apply == nil {
		return fmt.Errorf("schema migration %s has no apply function", def.Key)
	}
	if err := ensureSchemaMigrationsTable(); err != nil {
		return err
	}

	expectedChecksum := schemaMigrationChecksum(def)
	var existing SchemaMigration
	err := DB.Where("migration_key = ?", def.Key).First(&existing).Error
	if err == nil {
		if existing.Checksum != "" && existing.Checksum != expectedChecksum {
			return fmt.Errorf("schema migration %s checksum mismatch: database=%s binary=%s", def.Key, existing.Checksum, expectedChecksum)
		}
		if existing.Checksum == "" || existing.AppVersion == "" {
			return DB.Model(&SchemaMigration{}).Where("migration_key = ?", def.Key).Updates(map[string]interface{}{
				"checksum":    expectedChecksum,
				"app_version": common.Version,
			}).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	startedAt := time.Now()
	if err := def.Apply(); errors.Is(err, errSchemaMigrationNotApplied) {
		return nil
	} else if err != nil {
		return err
	}

	record := SchemaMigration{
		Key:        def.Key,
		Checksum:   expectedChecksum,
		AppVersion: common.Version,
		AppliedAt:  time.Now().Unix(),
		DurationMS: time.Since(startedAt).Milliseconds(),
	}
	if err := DB.Create(&record).Error; err != nil {
		return err
	}
	common.SysLog("schema migration applied: " + def.Key)
	return nil
}

func withSchemaMigrationLock(fn func() error) error {
	if err := ensureSchemaMigrationLockTable(); err != nil {
		return err
	}
	now := time.Now().Unix()
	lock := schemaMigrationLock{
		Name:       "main",
		Owner:      uuid.NewString(),
		AcquiredAt: now,
		ExpiresAt:  now + int64((10 * time.Minute).Seconds()),
	}
	if err := DB.Create(&lock).Error; err != nil {
		result := DB.Where("lock_name = ? AND expires_at < ?", lock.Name, now).Delete(&schemaMigrationLock{})
		if result.Error != nil || result.RowsAffected == 0 {
			return ErrSchemaMigrationLocked
		}
		if err := DB.Create(&lock).Error; err != nil {
			return ErrSchemaMigrationLocked
		}
	}
	defer func() {
		if err := DB.Where("lock_name = ? AND owner = ?", lock.Name, lock.Owner).Delete(&schemaMigrationLock{}).Error; err != nil {
			common.SysLog("failed to release schema migration lock: " + err.Error())
		}
	}()
	return fn()
}

func GetSchemaMigrationStatus() ([]SchemaMigrationStatus, error) {
	defs := mainSchemaMigrationDefinitions()
	statuses := make([]SchemaMigrationStatus, 0, len(defs))
	applied := map[string]SchemaMigration{}
	if DB.Migrator().HasTable(&SchemaMigration{}) {
		selectColumns := "migration_key, applied_at"
		if DB.Migrator().HasColumn(&SchemaMigration{}, "checksum") &&
			DB.Migrator().HasColumn(&SchemaMigration{}, "app_version") &&
			DB.Migrator().HasColumn(&SchemaMigration{}, "duration_ms") {
			selectColumns += ", checksum, app_version, duration_ms"
		}
		var records []SchemaMigration
		if err := DB.Select(selectColumns).Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			applied[record.Key] = record
		}
	}
	for _, def := range defs {
		record, ok := applied[def.Key]
		statuses = append(statuses, SchemaMigrationStatus{
			Key:             def.Key,
			Checksum:        schemaMigrationChecksum(def),
			Applied:         ok && record.Checksum == schemaMigrationChecksum(def),
			AppliedChecksum: record.Checksum,
			AppVersion:      record.AppVersion,
			AppliedAt:       record.AppliedAt,
			DurationMS:      record.DurationMS,
		})
	}
	return statuses, nil
}

func CheckSchemaMigrations() error {
	statuses, err := GetSchemaMigrationStatus()
	if err != nil {
		return err
	}
	for _, status := range statuses {
		if !status.Applied {
			return fmt.Errorf("%w: %s", ErrSchemaMigrationsPending, status.Key)
		}
	}
	return nil
}
