package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelModelCapabilityTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		ReloadChannelModelCapabilityCache()
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ChannelModelCapability{}))
	DB = db
	common.MemoryCacheEnabled = true
	ReloadChannelModelCapabilityCache()
}

func TestChannelModelCapabilityUpsertNormalizesAndCaches(t *testing.T) {
	setupChannelModelCapabilityTestDB(t)
	require.NoError(t, UpsertChannelModelCapability(ChannelModelCapability{
		ChannelId:        71,
		ModelName:        " GPT-5.5 ",
		Capability:       " Responses.Compaction ",
		Status:           ChannelCapabilityStatusSupported,
		CapabilityValue:  "NATIVE_V2",
		NativeStatus:     ChannelCapabilityStatusSupported,
		RouteFingerprint: "fingerprint",
	}))

	record, found := GetChannelModelCapability(71, "gpt-5.5", ChannelCapabilityResponsesCompaction)
	require.True(t, found)
	require.Equal(t, "gpt-5.5", record.ModelName)
	require.Equal(t, "responses.compaction", record.Capability)
	require.Equal(t, "native_v2", record.CapabilityValue)
	require.Positive(t, record.CreatedTime)
	require.Positive(t, record.UpdatedTime)
}
