package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestOaiStreamPreflightBlocksBeforeReleasingFirstBytes(t *testing.T) {
	blob := strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=", 60)
	body := `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":"` + blob + `"}}]}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	enabled := true
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "gpt-test",
		DisablePing:     true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         88,
			UpstreamModelName: "gpt-test",
			ChannelSetting: dto.ChannelSettings{
				AntiPoisonEnabled:    &enabled,
				AntiPoisonStreamMode: operation_setting.AntiPoisonStreamPreflightFirstBytes,
				AntiPoisonOpaqueScan: operation_setting.AntiPoisonModeScoreStrict,
			},
		},
	}

	_, err := OaiStreamHandler(ctx, info, resp)

	if err == nil {
		t.Fatalf("expected preflight opaque validation error")
	}
	if got := rec.Body.String(); strings.Contains(got, blob) {
		t.Fatalf("preflight leaked risky first bytes: %q", got[:min(len(got), 120)])
	}
}
