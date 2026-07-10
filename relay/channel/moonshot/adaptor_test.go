package moonshot

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestNormalizesKimiK26Temperature(t *testing.T) {
	tests := []struct {
		name     string
		request  *dto.GeneralOpenAIRequest
		info     *relaycommon.RelayInfo
		expected *float64
	}{
		{
			name:     "mapped upstream model",
			request:  &dto.GeneralOpenAIRequest{Model: "alias", Temperature: common.GetPointer(0.7)},
			info:     &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kimi-k2.6"}},
			expected: common.GetPointer(1.0),
		},
		{
			name:     "request model fallback",
			request:  &dto.GeneralOpenAIRequest{Model: "KIMI-K2.6", Temperature: common.GetPointer(0.0)},
			info:     nil,
			expected: common.GetPointer(1.0),
		},
		{
			name:     "omitted temperature stays omitted",
			request:  &dto.GeneralOpenAIRequest{Model: "kimi-k2.6"},
			info:     &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kimi-k2.6"}},
			expected: nil,
		},
		{
			name:     "other model unchanged",
			request:  &dto.GeneralOpenAIRequest{Model: "kimi-k2.5", Temperature: common.GetPointer(0.7)},
			info:     &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kimi-k2.5"}},
			expected: common.GetPointer(0.7),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, test.info, test.request)
			require.NoError(t, err)
			convertedRequest := converted.(*dto.GeneralOpenAIRequest)
			if test.expected == nil {
				require.Nil(t, convertedRequest.Temperature)
				return
			}
			require.NotNil(t, convertedRequest.Temperature)
			require.Equal(t, *test.expected, *convertedRequest.Temperature)
		})
	}
}
