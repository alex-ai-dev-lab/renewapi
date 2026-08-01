package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
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
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 91},
		ModelMappingRoute: relaycommon.ModelMappingRouteCursor{
			ChannelId:  91,
			Candidates: []string{"a", "b"},
			Index:      1,
		},
	}
	_, _, ok := AdvanceModelMappingFallback(info)
	require.False(t, ok)
}

func TestModelMappedHelperResponsesCompactPrefersExplicitVirtualModelMapping(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_mapping", `{"gpt-5.4-openai-compact":"provider/gpt-5.4","gpt-5.4":"fallback/gpt-5.4"}`)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: "gpt-5.4-openai-compact",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 91, UpstreamModelName: "gpt-5.4-openai-compact"},
	}

	require.NoError(t, ModelMappedHelper(c, info, nil))
	require.Equal(t, "provider/gpt-5.4", info.UpstreamModelName)
}
