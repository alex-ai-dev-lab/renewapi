package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestUserLogsRedactHistoricalUpstreamDetails(t *testing.T) {
	logs := []*Log{{Type: LogTypeError, Content: "private.example rejected key", ChannelName: "private-channel", UpstreamRequestId: "private-id", Other: `{"error_code":"plugin-internal","risk_reason":"private.example","admin_info":{"real_error":"private-key"},"stream_status":{"end_error":"private-key"}}`}}
	formatUserLogs(logs, 0)
	data, err := common.Marshal(logs)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private")
	require.NotContains(t, string(data), "plugin-internal")
	require.Equal(t, types.PublicModelCapacityMessage, logs[0].Content)
}
