package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type optionWithoutPrimaryKey struct {
	Key   string `gorm:"type:varchar(191);uniqueIndex"`
	Value string
}

func (optionWithoutPrimaryKey) TableName() string { return "options" }

func TestOptionsExistingPrimaryKeyMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	assertOptionsPrimaryKeyMigration(t, db)
}

func TestOptionsPrimaryKeyExternalDatabase(t *testing.T) {
	dsn := os.Getenv("REQUEST_GUARD_TEST_DSN")
	if dsn == "" {
		t.Skip("需要独立的 MySQL/PostgreSQL 测试数据库")
	}
	var dialect gorm.Dialector
	switch os.Getenv("REQUEST_GUARD_TEST_DRIVER") {
	case "mysql":
		dialect = mysql.Open(dsn)
	case "postgres":
		dialect = postgres.Open(dsn)
	default:
		t.Fatal("未指定测试数据库类型")
	}
	db, err := gorm.Open(dialect, &gorm.Config{})
	require.NoError(t, err)
	assertOptionsPrimaryKeyMigration(t, db)
}

func assertOptionsPrimaryKeyMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB, oldOptions := DB, common.OptionMap
	oldRatio, oldPrice := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	DB, common.OptionMap = db, map[string]string{}
	t.Cleanup(func() {
		DB, common.OptionMap = oldDB, oldOptions
		_ = ratio_setting.UpdateModelRatioByJSONString(oldRatio)
		_ = ratio_setting.UpdateModelPriceByJSONString(oldPrice)
		_ = db.Migrator().DropTable(&Option{})
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	require.NoError(t, db.Migrator().DropTable(&Option{}))
	require.NoError(t, db.AutoMigrate(&optionWithoutPrimaryKey{}))
	require.NoError(t, db.Create(&optionWithoutPrimaryKey{Key: "ModelRatio", Value: `{"test-model":1}`}).Error)
	require.NoError(t, migrateOptionsPrimaryKeyV1())
	require.NoError(t, migrateOptionsPrimaryKeyV1())
	columns, err := db.Migrator().ColumnTypes(&Option{})
	require.NoError(t, err)
	for _, column := range columns {
		if column.Name() == "key" {
			primary, _ := column.PrimaryKey()
			require.True(t, primary, "旧 options 表必须具备实际主键")
		}
	}
	require.NoError(t, UpdateOption("ModelPrice", `{"priced-model":0.5}`))
	require.NoError(t, UpdateOption("ModelPrice", `{"priced-model":0.8}`))
	var stored Option
	require.NoError(t, db.Where(map[string]any{"key": "ModelRatio"}).First(&stored).Error)
	require.Equal(t, `{"test-model":1}`, stored.Value)
	require.NoError(t, db.Where(map[string]any{"key": "ModelPrice"}).First(&Option{}).Error)
	err = UpdateOptionsBulk(map[string]string{"ModelPrice": `{"priced-model":1}`, "ModelRequestRateLimitGroup": "{invalid"})
	require.Error(t, err)
	stored = Option{}
	require.NoError(t, db.Where(map[string]any{"key": "ModelPrice"}).First(&stored).Error)
	require.Equal(t, `{"priced-model":0.8}`, stored.Value)
	price, ok := ratio_setting.GetModelPrice("priced-model", false)
	require.True(t, ok)
	require.Equal(t, 0.8, price)
}

func TestOptionsPrimaryKeyPreservesSQLiteReferencesAndMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous; _ = sqlDB.Close() })
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(&optionWithoutPrimaryKey{}))
	require.NoError(t, db.Create(&optionWithoutPrimaryKey{Key: "preserved", Value: "old"}).Error)
	require.NoError(t, db.Exec("CREATE TABLE option_refs (option_key text REFERENCES options(`key`) ON DELETE CASCADE)").Error)
	require.NoError(t, db.Exec("INSERT INTO option_refs VALUES ('preserved')").Error)
	require.NoError(t, db.Exec("CREATE INDEX options_value_idx ON options(value)").Error)
	require.NoError(t, db.Exec("CREATE TABLE option_updates (value text)").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER options_update_trigger AFTER UPDATE ON options BEGIN INSERT INTO option_updates(value) VALUES (NEW.value); END").Error)
	require.NoError(t, migrateOptionsPrimaryKeyV1())
	var count, foreignKeys int64
	require.NoError(t, db.Table("option_refs").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error)
	require.EqualValues(t, 1, foreignKeys)
	require.True(t, db.Migrator().HasIndex(&Option{}, "options_value_idx"))
	require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "preserved"}).Update("value", "new").Error)
	var value string
	require.NoError(t, db.Table("option_updates").Select("value").Scan(&value).Error)
	require.Equal(t, "new", value)
}

func TestOptionsPrimaryKeyRejectsAmbiguousLegacyRows(t *testing.T) {
	for _, scenario := range []string{"null", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			previous := DB
			DB = db
			t.Cleanup(func() { DB = previous; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
			require.NoError(t, db.Exec("CREATE TABLE options (`key` text, value text)").Error)
			if scenario == "null" {
				require.NoError(t, db.Exec("INSERT INTO options VALUES (NULL, 'preserved')").Error)
			} else {
				require.NoError(t, db.Exec("INSERT INTO options VALUES ('same', 'first'), ('same', 'second')").Error)
			}
			var before, after int64
			require.NoError(t, db.Table("options").Count(&before).Error)
			require.Error(t, migrateOptionsPrimaryKeyV1())
			require.NoError(t, db.Table("options").Count(&after).Error)
			require.Equal(t, before, after)
			require.False(t, db.Migrator().HasTable("options_primary_key_v1"))
		})
	}
}
