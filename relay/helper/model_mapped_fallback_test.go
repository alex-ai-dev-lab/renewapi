package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperAdvancesOrderedFallbacks(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_mapping", `{"glm-5.2":["@cf/zhipu-ai/glm-5.2","TCADP/glm-5.2","z-ai/glm-5.2"]}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "glm-5.2",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 91, UpstreamModelName: "glm-5.2"},
	}

	require.NoError(t, ModelMappedHelper(c, info, nil))
	require.Equal(t, "@cf/zhipu-ai/glm-5.2", info.UpstreamModelName)
	from, to, ok := AdvanceModelMappingFallback(info)
	require.True(t, ok)
	require.Equal(t, "@cf/zhipu-ai/glm-5.2", from)
	require.Equal(t, "TCADP/glm-5.2", to)

	info.IsModelMapped = false
	info.UpstreamModelName = info.OriginModelName
	require.NoError(t, ModelMappedHelper(c, info, nil))
	require.Equal(t, "TCADP/glm-5.2", info.UpstreamModelName)
}

func TestAdvanceModelMappingFallbackStopsAtLastCandidate(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta:                    &relaycommon.ChannelMeta{ChannelId: 91},
		ModelMappingFallbackChannelId:  91,
		ModelMappingFallbackCandidates: []string{"a", "b"},
		ModelMappingFallbackIndex:      1,
	}
	_, _, ok := AdvanceModelMappingFallback(info)
	require.False(t, ok)
}
