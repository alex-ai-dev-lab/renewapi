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
	if driver == "" || dsn == "" { t.Skip("set CHANNELCONFIG_TEST_DRIVER and CHANNELCONFIG_TEST_DSN for MySQL/PostgreSQL integration") }
	var dialector gorm.Dialector
	switch driver { case "mysql": dialector = mysql.Open(dsn); case "postgres": dialector = postgres.Open(dsn); default: t.Fatalf("unsupported driver %q", driver) }
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelStatus{}, &model.ModelEndpoint{}, &model.ConfigAudit{}))
	oldDB, oldCache := model.DB, common.MemoryCacheEnabled
	model.DB, common.MemoryCacheEnabled = db, false
	t.Cleanup(func() { model.DB, common.MemoryCacheEnabled = oldDB, oldCache })

	channel := model.Channel{Type: 1, Key: "sk-external-test", Status: common.ChannelStatusEnabled, Name: "external-" + driver, Models: "gpt-test", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(db))
	t.Cleanup(func() {
		_ = db.Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).Delete(&model.ConfigAudit{}).Error
		_ = db.Where("channel_id = ?", channel.Id).Delete(&model.ModelEndpoint{}).Error
		_ = db.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error
		_ = db.Delete(&model.Channel{}, channel.Id).Error
	})
	expected := channel.ConfigVersion
	channel.Name += "-updated"
	endpoints := []*model.ModelEndpoint{{ChannelId: channel.Id, Model: "gpt-test", BaseURL: "https://example.com/v1"}}
	result, err := Update(UpdateCommand{Channel: &channel, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"name": true, "model_endpoints": true}, ModelEndpoints: &endpoints})
	require.NoError(t, err)
	require.Equal(t, expected+1, result.Channel.ConfigVersion)
}
