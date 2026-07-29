package service

import (
	"net/http/httptest"
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
		RoutingModelName:  "gpt-5.6-sol",
		MappedModelName:   "provider/gpt-5.6-sol",
		BillingModelName:  "gpt-5.5",
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{UpstreamModelName: "provider/gpt-5.6-sol-final"},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	require.Equal(t, "gpt-5.5", other["client_model_name"])
	require.Equal(t, "gpt-5.6-sol", other["routing_model_name"])
	require.Equal(t, "provider/gpt-5.6-sol", other["mapped_model_name"])
	require.Equal(t, "provider/gpt-5.6-sol-final", other["upstream_model_name"])
	require.Equal(t, "gpt-5.5", other["billing_model_name"])
}

func TestGenerateTextOtherInfoRecordsExplicitIdentityWhenModelsMatch(t *testing.T) {
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
	require.Equal(t, "gpt-5.5", other["client_model_name"])
	require.Equal(t, "gpt-5.5", other["routing_model_name"])
	require.Equal(t, "gpt-5.5", other["billing_model_name"])
}

func TestGenerateTextOtherInfoIncludesSuccessfulSessionRecovery(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(ginKeySessionRecoveryLogInfo, map[string]interface{}{
		"action": "recovered_next_request", "previous_channel": 201, "channel_id": 205, "result": "success",
	})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	adminInfo := other["admin_info"].(map[string]interface{})
	recovery := adminInfo["session_recovery"].(map[string]interface{})
	require.Equal(t, "recovered_next_request", recovery["action"])
	require.Equal(t, "success", recovery["result"])
}
