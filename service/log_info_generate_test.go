package service

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoRecordsCompactionRoutingModels(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	now := time.Now()
	info := &relaycommon.RelayInfo{
		OriginModelName:   "gpt-5.6-sol",
		ClientModelName:   "gpt-5.5",
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	require.Equal(t, "gpt-5.5", other["client_model_name"])
	require.Equal(t, "gpt-5.6-sol", other["routing_model_name"])
}

func TestGenerateTextOtherInfoOmitsDuplicateClientModel(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	now := time.Now()
	info := &relaycommon.RelayInfo{
		OriginModelName:   "gpt-5.5",
		ClientModelName:   "gpt-5.5",
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	require.NotContains(t, other, "client_model_name")
	require.NotContains(t, other, "routing_model_name")
}
