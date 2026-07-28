package channelconfig

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := model.DB
	oldMemoryCache := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:channelconfig?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelStatus{}, &model.ConfigAudit{}, &model.ModelEndpoint{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCache
	})
	return db
}

func createConfigTestChannel(t *testing.T, db *gorm.DB) model.Channel {
	t.Helper()
	channel := model.Channel{Type: 1, Key: "sk-original-secret", Status: common.ChannelStatusEnabled, Name: "before", Models: "gpt-test", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(db))
	require.Equal(t, int64(1), channel.ConfigVersion)
	return channel
}

func TestUpdateUsesCASAndRedactsAuditSecrets(t *testing.T) {
	db := setupChannelConfigTestDB(t)
	channel := createConfigTestChannel(t, db)
	expected := channel.ConfigVersion
	channel.Name = "after"
	channel.Key = "sk-replacement-secret"

	result, err := Update(UpdateCommand{Channel: &channel, ExpectedConfigVersion: &expected,
		PresentFields: map[string]bool{"name": true, "key": true}, UpdateKey: true,
		Audit: AuditMetadata{OperatorID: 7, RequestID: "req-1", Reason: "rotate credential"}})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Channel.ConfigVersion)
	require.True(t, result.CacheSynchronized)

	var audit model.ConfigAudit
	require.NoError(t, db.First(&audit).Error)
	require.NotContains(t, audit.Diff, "sk-original-secret")
	require.NotContains(t, audit.Diff, "sk-replacement-secret")
	require.Contains(t, audit.Diff, `"redacted":true`)
	require.Contains(t, audit.Diff, `"field":"name"`)

	channel.Name = "stale write"
	_, err = Update(UpdateCommand{Channel: &channel, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"name": true}})
	require.ErrorIs(t, err, model.ErrChannelConfigConflict)
}

func TestEndpointFailureRollsBackChannelAndAudit(t *testing.T) {
	db := setupChannelConfigTestDB(t)
	channel := createConfigTestChannel(t, db)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_endpoint_insert BEFORE INSERT ON model_endpoints BEGIN SELECT RAISE(FAIL, 'forced endpoint failure'); END`).Error)
	expected := channel.ConfigVersion
	channel.Name = "must roll back"
	channelType := 1
	endpoints := []*model.ModelEndpoint{{ChannelId: channel.Id, Model: "gpt-test", BaseURL: "https://example.com", ChannelType: &channelType}}

	_, err := Update(UpdateCommand{Channel: &channel, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"name": true}, ModelEndpoints: &endpoints})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "forced endpoint failure"))

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "before", stored.Name)
	require.Equal(t, int64(1), stored.ConfigVersion)
	var auditCount int64
	require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}
