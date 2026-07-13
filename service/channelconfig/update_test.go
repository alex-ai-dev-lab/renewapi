package channelconfig

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpdateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelModelStatus{},
		&model.ModelEndpoint{},
		&model.ConfigAudit{},
	))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	return db
}

func createUpdateChannel(t *testing.T, name string) model.Channel {
	t.Helper()
	channel := model.Channel{
		Type:        1,
		Key:         "sk-test",
		Status:      common.ChannelStatusEnabled,
		Name:        name,
		Models:      "gpt-5.5",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))
	return channel
}

func updateCommand(channel *model.Channel, present ...string) UpdateCommand {
	fields := make(map[string]bool, len(present))
	for _, field := range present {
		fields[field] = true
	}
	return UpdateCommand{
		Channel:       channel,
		PresentFields: fields,
		Audit:         AuditMetadata{OperatorID: 7, RequestID: "request-test"},
	}
}

func TestUpdateCommitsChannelAbilitiesEndpointsAuditAndVersion(t *testing.T) {
	db := setupUpdateTestDB(t)
	channel := createUpdateChannel(t, "before")
	expected := channel.ConfigVersion
	channel.Name = "after"
	channel.Models = "gpt-5.5,gpt-5.6"
	endpoints := []*model.ModelEndpoint{{Model: "gpt-5.6", BaseURL: "https://example.com/v1"}}
	command := updateCommand(&channel, "name", "models", "model_endpoints")
	command.ExpectedConfigVersion = &expected
	command.ModelEndpoints = &endpoints

	result, err := Update(command)
	require.NoError(t, err)
	require.False(t, result.NoOp)
	require.Equal(t, expected+1, result.Channel.ConfigVersion)
	require.True(t, result.ChangeSet.AbilityChanged)
	require.True(t, result.ChangeSet.EndpointsChanged)

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "after", stored.Name)
	require.Equal(t, expected+1, stored.ConfigVersion)

	var abilities []model.Ability
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Order("model asc").Find(&abilities).Error)
	require.Len(t, abilities, 2)

	var audits []model.ConfigAudit
	require.NoError(t, db.Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).Find(&audits).Error)
	require.Len(t, audits, 1)
	require.Equal(t, expected+1, audits[0].ConfigVersion)
	require.NotContains(t, audits[0].Diff, channel.Key)
}

func TestUpdatePreservesClearsAndSkipsUnchangedEndpoints(t *testing.T) {
	db := setupUpdateTestDB(t)
	channel := createUpdateChannel(t, "before")
	require.NoError(t, db.Create(&model.ModelEndpoint{
		ChannelId: channel.Id,
		Model:     "gpt-5.5",
		BaseURL:   "https://old.example/v1",
	}).Error)

	channel.Name = "preserve"
	result, err := Update(updateCommand(&channel, "name"))
	require.NoError(t, err)
	require.False(t, result.ChangeSet.EndpointsChanged)
	var endpoints []model.ModelEndpoint
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&endpoints).Error)
	require.Len(t, endpoints, 1)

	channel = *result.Channel
	same := []*model.ModelEndpoint{{Model: "gpt-5.5", BaseURL: "https://old.example/v1"}}
	command := updateCommand(&channel, "model_endpoints")
	command.ModelEndpoints = &same
	result, err = Update(command)
	require.NoError(t, err)
	require.True(t, result.NoOp)

	channel = *result.Channel
	empty := []*model.ModelEndpoint{}
	command = updateCommand(&channel, "model_endpoints")
	command.ModelEndpoints = &empty
	result, err = Update(command)
	require.NoError(t, err)
	require.True(t, result.ChangeSet.EndpointsChanged)
	endpoints = nil
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&endpoints).Error)
	require.Empty(t, endpoints)
}

func TestUpdateRejectsStaleVersion(t *testing.T) {
	setupUpdateTestDB(t)
	channel := createUpdateChannel(t, "before")
	stale := channel.ConfigVersion
	channel.Name = "first"
	command := updateCommand(&channel, "name")
	command.ExpectedConfigVersion = &stale
	first, err := Update(command)
	require.NoError(t, err)

	staleChannel := *first.Channel
	staleChannel.Name = "stale"
	command = updateCommand(&staleChannel, "name")
	command.ExpectedConfigVersion = &stale
	_, err = Update(command)
	require.ErrorIs(t, err, model.ErrChannelConfigConflict)
}

func TestUpdateNoOpDoesNotWriteAuditOrIncrementVersion(t *testing.T) {
	db := setupUpdateTestDB(t)
	channel := createUpdateChannel(t, "same")
	version := channel.ConfigVersion
	result, err := Update(updateCommand(&channel, "name"))
	require.NoError(t, err)
	require.True(t, result.NoOp)
	require.Equal(t, version, result.Channel.ConfigVersion)
	var auditCount int64
	require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestUpdateRollsBackWhenEndpointOrAuditWriteFails(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		table string
	}{
		{name: "endpoint", table: "model_endpoints"},
		{name: "audit", table: "config_audits"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupUpdateTestDB(t)
			channel := createUpdateChannel(t, "before")
			callbackName := "test:reject_" + testCase.table
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register(
				callbackName,
				func(tx *gorm.DB) {
					if tx.Statement != nil && tx.Statement.Table == testCase.table {
						tx.AddError(errors.New("forced write failure"))
					}
				},
			))
			t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

			channel.Name = "must rollback"
			endpoints := []*model.ModelEndpoint{{Model: "gpt-5.6", BaseURL: "https://new.example/v1"}}
			command := updateCommand(&channel, "name", "model_endpoints")
			command.ModelEndpoints = &endpoints
			_, err := Update(command)
			require.ErrorContains(t, err, "forced write failure")

			var stored model.Channel
			require.NoError(t, db.First(&stored, channel.Id).Error)
			require.Equal(t, "before", stored.Name)
			var auditCount int64
			require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&auditCount).Error)
			require.Zero(t, auditCount)
		})
	}
}
