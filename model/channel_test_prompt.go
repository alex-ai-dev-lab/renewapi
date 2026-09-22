package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelTestPrompt struct {
	ID             int    `json:"id" gorm:"primaryKey"`
	Name           string `json:"name" gorm:"size:100;not null"`
	Prompt         string `json:"prompt" gorm:"type:text;not null"`
	Description    string `json:"description" gorm:"type:text"`
	Enabled        bool   `json:"enabled" gorm:"not null"`
	IsDefault      bool   `json:"is_default" gorm:"not null;default:false"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
	ReferenceCount int64  `json:"reference_count" gorm:"-"`
	// 可空唯一槽位让三种数据库都能阻止并发设置多个默认项。
	DefaultSlot *int `json:"-" gorm:"uniqueIndex"`
}

var ErrChannelTestPromptReferenced = errors.New("测试提示词仍被渠道引用，请先修改渠道配置")
var ErrChannelTestPromptNotFound = errors.New("测试提示词不存在")

func migrateChannelTestPromptsV1() error {
	if err := DB.AutoMigrate(&ChannelTestPrompt{}); err != nil {
		return err
	}
	if !DB.Migrator().HasColumn(&Channel{}, "ChannelTestPromptID") {
		if err := DB.Migrator().AddColumn(&Channel{}, "ChannelTestPromptID"); err != nil {
			return err
		}
	}
	if !DB.Migrator().HasIndex(&Channel{}, "ChannelTestPromptID") {
		return DB.Migrator().CreateIndex(&Channel{}, "ChannelTestPromptID")
	}
	return nil
}

// 渠道引用写入和删除均先锁定同一提示词，避免检查后删除产生悬空引用。
func lockChannelTestPromptTx(tx *gorm.DB, id int) (*ChannelTestPrompt, error) {
	if id <= 0 {
		return nil, ErrChannelTestPromptNotFound
	}
	if tx.Dialector.Name() == "sqlite" {
		if err := tx.Model(&ChannelTestPrompt{}).Where("id = ?", id).UpdateColumn("name", gorm.Expr("name")).Error; err != nil {
			return nil, err
		}
	}
	var prompt ChannelTestPrompt
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&prompt, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChannelTestPromptNotFound
	}
	return &prompt, err
}

func validateChannelTestPromptTx(tx *gorm.DB, id int) error {
	if id == 0 {
		return nil
	}
	_, err := lockChannelTestPromptTx(tx, id)
	return err
}

func (channel *Channel) BeforeSave(tx *gorm.DB) error {
	return validateChannelTestPromptTx(tx, channel.ChannelTestPromptID)
}

func ListChannelTestPrompts() ([]ChannelTestPrompt, error) {
	prompts := make([]ChannelTestPrompt, 0)
	if err := DB.Order("is_default DESC, id ASC").Find(&prompts).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		ChannelTestPromptID int
		Count               int64
	}
	if err := DB.Model(&Channel{}).Select("channel_test_prompt_id, COUNT(*) AS count").Where("channel_test_prompt_id > ?", 0).Group("channel_test_prompt_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byID := make(map[int]int64, len(counts))
	for _, count := range counts {
		byID[count.ChannelTestPromptID] = count.Count
	}
	for i := range prompts {
		prompts[i].ReferenceCount = byID[prompts[i].ID]
	}
	return prompts, nil
}

func SaveChannelTestPrompt(prompt *ChannelTestPrompt, create bool) error {
	prompt.Name = strings.TrimSpace(prompt.Name)
	prompt.Prompt = strings.TrimSpace(prompt.Prompt)
	prompt.Description = strings.TrimSpace(prompt.Description)
	if prompt.Name == "" || len([]rune(prompt.Name)) > 100 {
		return fmt.Errorf("提示词名称必须为 1 至 100 个字符")
	}
	if prompt.Prompt == "" || len(prompt.Prompt) > 65536 {
		return fmt.Errorf("测试提示词不能为空且不能超过 64 KiB")
	}
	if len([]rune(prompt.Description)) > 2000 {
		return fmt.Errorf("提示词说明不能超过 2000 个字符")
	}
	if !prompt.Enabled {
		prompt.IsDefault = false
	}
	prompt.DefaultSlot = nil
	if prompt.IsDefault {
		slot := 1
		prompt.DefaultSlot = &slot
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if !create {
			before, err := lockChannelTestPromptTx(tx, prompt.ID)
			if err != nil {
				return err
			}
			prompt.CreatedAt = before.CreatedAt
		}
		if prompt.IsDefault {
			if err := tx.Model(&ChannelTestPrompt{}).Where("is_default = ?", true).Updates(map[string]any{"is_default": false, "default_slot": nil}).Error; err != nil {
				return err
			}
		}
		if create {
			prompt.ID, prompt.CreatedAt, prompt.UpdatedAt = 0, 0, 0
			return tx.Create(prompt).Error
		}
		return tx.Model(prompt).Select("name", "prompt", "description", "enabled", "is_default", "default_slot", "updated_at").Updates(prompt).Error
	})
}

func DeleteChannelTestPrompt(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if _, err := lockChannelTestPromptTx(tx, id); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&Channel{}).Where("channel_test_prompt_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrChannelTestPromptReferenced
		}
		return tx.Delete(&ChannelTestPrompt{}, id).Error
	})
}

func ResolveChannelTestPrompt(channel *Channel, legacy string) (string, error) {
	var prompt ChannelTestPrompt
	if channel != nil && channel.ChannelTestPromptID > 0 {
		err := DB.Where("id = ? AND enabled = ?", channel.ChannelTestPromptID, true).First(&prompt).Error
		if err == nil {
			return prompt.Prompt, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}
	err := DB.Where("is_default = ? AND enabled = ?", true, true).First(&prompt).Error
	if err == nil {
		return prompt.Prompt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if legacy = strings.TrimSpace(legacy); legacy != "" {
		return legacy, nil
	}
	return "hi", nil
}
