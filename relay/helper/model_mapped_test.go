package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newCompactMappingTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx
}

func TestModelMappedHelperResponsesCompactStripsVirtualSuffix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newCompactMappingTestContext()
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5.4-openai-compact"}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: "gpt-5.4-openai-compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.4-openai-compact",
		},
	}

	require.NoError(t, ModelMappedHelper(ctx, info, request))
	require.Equal(t, "gpt-5.4", info.UpstreamModelName)
	require.Equal(t, "gpt-5.4", request.Model)
	require.False(t, info.IsModelMapped)
}

func TestModelMappedHelperResponsesCompactHonorsFullNameMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newCompactMappingTestContext()
	ctx.Set("model_mapping", `{"gpt-5.4-openai-compact":"provider/gpt-5.4"}`)
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5.4-openai-compact"}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: "gpt-5.4-openai-compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.4-openai-compact",
		},
	}

	require.NoError(t, ModelMappedHelper(ctx, info, request))
	require.Equal(t, "provider/gpt-5.4", info.UpstreamModelName)
	require.Equal(t, "provider/gpt-5.4", request.Model)
	require.True(t, info.IsModelMapped)
}

func TestModelMappedHelperResponsesCompactFallsBackToSuffixFreeMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newCompactMappingTestContext()
	ctx.Set("model_mapping", `{"gpt-5.4":"provider/gpt-5.4"}`)
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5.4-openai-compact"}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: "gpt-5.4-openai-compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.4-openai-compact",
		},
	}

	require.NoError(t, ModelMappedHelper(ctx, info, request))
	require.Equal(t, "provider/gpt-5.4", info.UpstreamModelName)
	require.Equal(t, "provider/gpt-5.4", request.Model)
	require.True(t, info.IsModelMapped)
}
