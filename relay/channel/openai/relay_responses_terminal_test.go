package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateResponsesStreamRequiresCompletedTerminal(t *testing.T) {
	body := `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	finalResp, usage, apiErr := aggregateResponsesStreamToResponse(ctx, info, resp)
	require.Nil(t, finalResp)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "responses stream missing response.completed")
}
