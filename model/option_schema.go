package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func migrateOptionsPrimaryKeyV1() error {
	if !DB.Migrator().HasTable(&Option{}) {
		return DB.AutoMigrate(&Option{})
	}
	columns, err := DB.Migrator().ColumnTypes(&Option{})
	if err != nil {
		return err
	}
	hasKeyPrimary := false
	for _, column := range columns {
		primary, _ := column.PrimaryKey()
		if primary {
			if column.Name() == "key" {
				hasKeyPrimary = true
				continue
			}
			return errors.New("options 表存在非 key 主键，需先核对自定义结构")
		}
	}
	if hasKeyPrimary {
		return nil
	}
	key := clause.Column{Name: "key"}
	var invalid int64
	if err := DB.Model(&Option{}).Where(clause.Eq{Column: key, Value: nil}).Count(&invalid).Error; err != nil {
		return err
	}
	if invalid > 0 {
		return errors.New("options 表存在 NULL key，不能自动建立主键")
	}
	duplicates := DB.Model(&Option{}).Select(key.Name).Group(key.Name).Having("COUNT(*) > 1")
	if err := DB.Table("(?) AS duplicate_options", duplicates).Count(&invalid).Error; err != nil {
		return err
	}
	if invalid > 0 {
		return errors.New("options 表存在重复 key，不能自动选择配置值")
	}
	if DB.Dialector.Name() != "sqlite" {
		return DB.Exec("ALTER TABLE ? ADD PRIMARY KEY (?)", clause.Table{Name: "options"}, key).Error
	}
	if len(columns) != 2 {
		return errors.New("options 表存在额外字段，不能自动重建自定义结构")
	}
	return rebuildSQLiteOptionsPrimaryKey()
}

func rebuildSQLiteOptionsPrimaryKey() error {
	// 固定同一连接切换外键检查；事务内保留值、索引和触发器，不触发外键级联删除。
	return DB.Connection(func(conn *gorm.DB) (err error) {
		var foreignKeys int
		if err = conn.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
			return err
		}
		if foreignKeys != 0 {
			if err = conn.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return err
			}
			defer func() {
				restoreErr := conn.Exec("PRAGMA foreign_keys = ON").Error
				if err == nil {
					err = restoreErr
				}
			}()
		}
		return conn.Transaction(func(tx *gorm.DB) error {
			const temporary = "options_primary_key_v1"
			if tx.Migrator().HasTable(temporary) {
				return fmt.Errorf("迁移暂存表 %s 已存在", temporary)
			}
			var definitions []struct{ SQL string }
			if err := tx.Raw("SELECT sql FROM sqlite_master WHERE tbl_name = ? AND type IN ('index', 'trigger') AND sql IS NOT NULL", "options").Scan(&definitions).Error; err != nil {
				return err
			}
			if err := tx.Table(temporary).Migrator().CreateTable(&Option{}); err != nil {
				return err
			}
			if err := tx.Exec("INSERT INTO options_primary_key_v1 (`key`, value) SELECT `key`, value FROM options").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP TABLE options").Error; err != nil {
				return err
			}
			if err := tx.Migrator().RenameTable(temporary, "options"); err != nil {
				return err
			}
			for _, definition := range definitions {
				if err := tx.Exec(definition.SQL).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}
