package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type channelBeforeTestPrompt struct {
	Id   int `gorm:"primaryKey"`
	Name string
	Key  string `gorm:"not null"`
}

func (channelBeforeTestPrompt) TableName() string { return "channels" }

func TestChannelTestPromptLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "prompts.db")), &gorm.Config{})
	require.NoError(t, err)
	assertChannelTestPromptLifecycle(t, db)
}

func TestChannelTestPromptExternalDatabase(t *testing.T) {
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
	assertChannelTestPromptLifecycle(t, db)
}

func assertChannelTestPromptLifecycle(t *testing.T, db *gorm.DB) {
	t.Helper()
	previous := DB
	DB = db
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&Channel{}, &ChannelTestPrompt{}, &Ability{})
		DB = previous
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	require.NoError(t, db.Migrator().DropTable(&Channel{}, &ChannelTestPrompt{}, &Ability{}))
	require.NoError(t, db.AutoMigrate(&channelBeforeTestPrompt{}))
	require.NoError(t, db.Create(&channelBeforeTestPrompt{Id: 1, Name: "旧渠道", Key: "test"}).Error)
	require.NoError(t, migrateChannelTestPromptsV1())
	require.NoError(t, migrateChannelTestPromptsV1())
	var legacyID int
	require.NoError(t, db.Model(&Channel{}).Select("channel_test_prompt_id").Where("id = ?", 1).Scan(&legacyID).Error)
	require.Zero(t, legacyID)
	// 先证明旧表能加列，再使用完整模型验证正式写入路径。
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	got, err := ResolveChannelTestPrompt(nil, "旧全局提示词")
	require.NoError(t, err)
	require.Equal(t, "旧全局提示词", got)
	got, err = ResolveChannelTestPrompt(nil, " ")
	require.NoError(t, err)
	require.Equal(t, "hi", got)

	first := ChannelTestPrompt{Name: "默认", Prompt: "默认内容", Enabled: true, IsDefault: true}
	second := ChannelTestPrompt{Name: "专用", Prompt: "渠道内容", Enabled: true}
	require.NoError(t, SaveChannelTestPrompt(&first, true))
	require.NoError(t, SaveChannelTestPrompt(&second, true))
	channel := Channel{Id: 2, Name: "引用渠道", Key: "test", ChannelTestPromptID: second.ID}
	require.NoError(t, db.Create(&channel).Error)
	got, err = ResolveChannelTestPrompt(&channel, "旧内容")
	require.NoError(t, err)
	require.Equal(t, "渠道内容", got)
	list, err := ListChannelTestPrompts()
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.EqualValues(t, 1, list[1].ReferenceCount)
	require.ErrorIs(t, DeleteChannelTestPrompt(second.ID), ErrChannelTestPromptReferenced)

	second.Enabled = false
	require.NoError(t, SaveChannelTestPrompt(&second, false))
	got, err = ResolveChannelTestPrompt(&channel, "旧内容")
	require.NoError(t, err)
	require.Equal(t, "默认内容", got)
	second.Enabled, second.IsDefault = true, true
	require.NoError(t, SaveChannelTestPrompt(&second, false))
	var count int64
	require.NoError(t, db.Model(&ChannelTestPrompt{}).Where("is_default = ?", true).Count(&count).Error)
	require.EqualValues(t, 1, count)
	slot := 1
	require.Error(t, db.Create(&ChannelTestPrompt{Name: "不能重复默认", Prompt: "x", Enabled: true, IsDefault: true, DefaultSlot: &slot}).Error)

	channel.ChannelTestPromptID = 0
	require.NoError(t, channel.Update())
	require.NoError(t, DeleteChannelTestPrompt(second.ID))
	channel.ChannelTestPromptID = second.ID
	require.ErrorIs(t, channel.Update(), ErrChannelTestPromptNotFound)
	require.ErrorIs(t, db.Create(&Channel{Name: "无效引用", Key: "test", ChannelTestPromptID: second.ID}).Error, ErrChannelTestPromptNotFound)
	require.ErrorIs(t, BatchInsertChannels([]Channel{{Name: "无效复制", Key: "test", ChannelTestPromptID: second.ID}}), ErrChannelTestPromptNotFound)
}

func TestChannelTestPromptConcurrentReferenceAndDelete(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "race.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{})
	require.NoError(t, err)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&Channel{}, &ChannelTestPrompt{}))
	for i := 0; i < 10; i++ {
		prompt := ChannelTestPrompt{Name: fmt.Sprintf("并发%d", i), Prompt: "内容", Enabled: true}
		require.NoError(t, SaveChannelTestPrompt(&prompt, true))
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _ = DeleteChannelTestPrompt(prompt.ID) }()
		go func() {
			defer wg.Done()
			<-start
			_ = db.Create(&Channel{Name: "并发引用", Key: "test", ChannelTestPromptID: prompt.ID}).Error
		}()
		close(start)
		wg.Wait()
		var dangling int64
		require.NoError(t, db.Model(&Channel{}).Where("channel_test_prompt_id > 0 AND channel_test_prompt_id NOT IN (?)", db.Model(&ChannelTestPrompt{}).Select("id")).Count(&dangling).Error)
		require.Zero(t, dangling)
	}
}
