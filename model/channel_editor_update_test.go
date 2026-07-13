package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createEditorUpdateChannel(t *testing.T, name string) Channel {
	t.Helper()
	channel := Channel{
		Type:        1,
		Key:         "sk-test",
		Status:      common.ChannelStatusEnabled,
		Name:        name,
		Models:      "gpt-5.5",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))
	return channel
}

func TestUpdateChannelEditorStateCommitsChannelAbilitiesAndEndpoints(t *testing.T) {
	setupChannelUpdateTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ModelEndpoint{}))
	channel := createEditorUpdateChannel(t, "before")

	channel.Name = "after"
	channel.Models = "gpt-5.5,gpt-5.6"
	endpoints := []*ModelEndpoint{
		{Model: "gpt-5.6", BaseURL: "https://example.com/v1"},
	}
	require.NoError(t, UpdateChannelEditorState(&channel, endpoints, nil, false))

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	require.Equal(t, "after", stored.Name)

	var abilities []Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Order("model asc").Find(&abilities).Error)
	require.Len(t, abilities, 2)
	require.Equal(t, "gpt-5.5", abilities[0].Model)
	require.Equal(t, "gpt-5.6", abilities[1].Model)

	var storedEndpoints []ModelEndpoint
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&storedEndpoints).Error)
	require.Len(t, storedEndpoints, 1)
	require.Equal(t, "gpt-5.6", storedEndpoints[0].Model)
}

func TestUpdateChannelEditorStatePreservesOrClearsEndpointsExplicitly(t *testing.T) {
	setupChannelUpdateTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ModelEndpoint{}))
	channel := createEditorUpdateChannel(t, "before")
	require.NoError(t, DB.Create(&ModelEndpoint{
		ChannelId: channel.Id,
		Model:     "gpt-5.5",
		BaseURL:   "https://old.example/v1",
	}).Error)

	channel.Name = "preserve"
	require.NoError(t, UpdateChannelEditorState(&channel, nil, nil, false))
	var endpoints []ModelEndpoint
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&endpoints).Error)
	require.Len(t, endpoints, 1)
	require.Equal(t, "gpt-5.5", endpoints[0].Model)

	channel.Name = "clear"
	require.NoError(t, UpdateChannelEditorState(&channel, []*ModelEndpoint{}, nil, false))
	endpoints = nil
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&endpoints).Error)
	require.Empty(t, endpoints)
}

func TestUpdateChannelEditorStateRollsBackWhenEndpointWriteFails(t *testing.T) {
	setupChannelUpdateTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ModelEndpoint{}))
	channel := createEditorUpdateChannel(t, "before")
	require.NoError(t, DB.Create(&ModelEndpoint{
		ChannelId: channel.Id,
		Model:     "gpt-5.5",
		BaseURL:   "https://old.example/v1",
	}).Error)

	callbackName := "test:reject_model_endpoint_create"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(
		callbackName,
		func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Table == "model_endpoints" {
				tx.AddError(errors.New("forced endpoint failure"))
			}
		},
	))
	t.Cleanup(func() {
		_ = DB.Callback().Create().Remove(callbackName)
	})

	channel.Name = "must rollback"
	err := UpdateChannelEditorState(&channel, []*ModelEndpoint{
		{Model: "gpt-5.6", BaseURL: "https://new.example/v1"},
	}, nil, false)
	require.ErrorContains(t, err, "forced endpoint failure")

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	require.Equal(t, "before", stored.Name)

	var storedEndpoints []ModelEndpoint
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&storedEndpoints).Error)
	require.Len(t, storedEndpoints, 1)
	require.Equal(t, "gpt-5.5", storedEndpoints[0].Model)
}

func TestUpdateChannelEditorStateRollsBackWhenAuditWriteFails(t *testing.T) {
	setupChannelUpdateTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ModelEndpoint{}, &ConfigAudit{}))
	channel := createEditorUpdateChannel(t, "before")
	require.NoError(t, DB.Create(&ModelEndpoint{
		ChannelId: channel.Id,
		Model:     "gpt-5.5",
		BaseURL:   "https://old.example/v1",
	}).Error)

	callbackName := "test:reject_config_audit_create"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(
		callbackName,
		func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Table == "config_audits" {
				tx.AddError(errors.New("forced audit failure"))
			}
		},
	))
	t.Cleanup(func() {
		_ = DB.Callback().Create().Remove(callbackName)
	})

	channel.Name = "must rollback"
	err := UpdateChannelEditorState(&channel, []*ModelEndpoint{
		{Model: "gpt-5.6", BaseURL: "https://new.example/v1"},
	}, &ConfigAudit{
		ResourceType: "channel",
		ResourceId:   channel.Id,
		Action:       "update",
		Diff:         `{}`,
		CreatedAt:    common.GetTimestamp(),
	}, false)
	require.ErrorContains(t, err, "forced audit failure")

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	require.Equal(t, "before", stored.Name)

	var storedEndpoints []ModelEndpoint
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&storedEndpoints).Error)
	require.Len(t, storedEndpoints, 1)
	require.Equal(t, "gpt-5.5", storedEndpoints[0].Model)

	var auditCount int64
	require.NoError(t, DB.Model(&ConfigAudit{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}
