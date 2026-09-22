package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponseModelKeepsRoutingAndFirstMismatch(t *testing.T) {
	for _, data := range []string{
		`{"model":"other-model"}`,
		`{"response":{"model":"other-model"}}`,
		`{"message":{"model":"other-model"}}`,
		`{"modelVersion":"other-model"}`,
	} {
		info := &RelayInfo{ClientModelName: "alias", OriginModelName: "alias", ChannelMeta: &ChannelMeta{UpstreamModelName: "gpt-4o"}}
		info.ObserveResponseModelJSON(data)
		info.ObserveResponseModel("gpt-4o")
		require.True(t, info.ResponseModel.Mismatch())
		require.Equal(t, "other-model", info.ResponseModel.ReturnedModel)
		require.Equal(t, "gpt-4o", info.UpstreamModel())
		require.Equal(t, "alias", info.BillingModel())
	}
	require.False(t, (&ResponseModel{UpstreamModel: "gpt-4o", ReturnedModel: "provider/GPT-4o"}).Mismatch())
	require.True(t, (&ResponseModel{UpstreamModel: "gpt-4", ReturnedModel: "gpt-4o"}).Mismatch())
}
