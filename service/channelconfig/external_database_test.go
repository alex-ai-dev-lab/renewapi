package channelconfig

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUpdateTransactionOnExternalDatabase(t *testing.T) {
	driver := os.Getenv("CHANNELCONFIG_TEST_DRIVER")
	dsn := os.Getenv("CHANNELCONFIG_TEST_DSN")
	if driver == "" || dsn == "" {
		t.Skip("external database test is enabled by CHANNELCONFIG_TEST_DRIVER and CHANNELCONFIG_TEST_DSN")
	}

	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	default:
		t.Fatalf("unsupported external database driver %q", driver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelModelStatus{},
		&model.ModelEndpoint{},
		&model.ConfigAudit{},
	))

	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	channel := createUpdateChannel(t, "external-"+driver)
	t.Cleanup(func() {
		_ = db.Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).Delete(&model.ConfigAudit{}).Error
		_ = db.Where("channel_id = ?", channel.Id).Delete(&model.ModelEndpoint{}).Error
		_ = db.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error
		_ = db.Delete(&model.Channel{}, channel.Id).Error
	})

	expectedVersion := channel.ConfigVersion
	channel.Name = "external-" + driver + "-updated"
	channel.Models = "gpt-5.5,gpt-5.6"
	endpoints := []*model.ModelEndpoint{{Model: "gpt-5.6", BaseURL: "https://example.com/v1"}}
	command := updateCommand(&channel, "name", "models", "model_endpoints")
	command.ExpectedConfigVersion = &expectedVersion
	command.ModelEndpoints = &endpoints

	result, err := Update(command)
	require.NoError(t, err)
	require.Equal(t, expectedVersion+1, result.Channel.ConfigVersion)
	require.True(t, result.ChangeSet.AbilityChanged)
	require.True(t, result.ChangeSet.EndpointsChanged)

	var auditCount int64
	require.NoError(t, db.Model(&model.ConfigAudit{}).
		Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).
		Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
}
