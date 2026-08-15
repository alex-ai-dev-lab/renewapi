package channel

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoRequestPropagatesInboundCancellation(t *testing.T) {
	resetAPIRequestHTTPClientState(t)
	upstreamCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
		close(upstreamCanceled)
	}))
	defer server.Close()

	requestContext, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	upstreamRequest, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	_, err = doRequest(ctx, upstreamRequest, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.Error(t, err)
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not inherit inbound cancellation")
	}
}

func TestApplyUpstreamBodyMetadataSupportsMultipartReplay(t *testing.T) {
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	require.NoError(t, writer.WriteField("model", "whisper-1"))
	part, err := writer.CreateFormFile("file", "sample.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("multipart replay"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(payload.Bytes())
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/audio/transcriptions", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: size,
		UpstreamRequestGetBody:  getBody,
	})

	require.Equal(t, size, req.ContentLength)
	require.NotNil(t, req.GetBody)
	for range 2 {
		replay, replayErr := req.GetBody()
		require.NoError(t, replayErr)
		form, parseErr := multipart.NewReader(replay, writer.Boundary()).ReadForm(1024)
		require.NoError(t, parseErr)
		require.Equal(t, []string{"whisper-1"}, form.Value["model"])
		require.Len(t, form.File["file"], 1)
		file, openErr := form.File["file"][0].Open()
		require.NoError(t, openErr)
		contents, readErr := io.ReadAll(file)
		require.NoError(t, readErr)
		require.NoError(t, file.Close())
		require.Equal(t, "multipart replay", string(contents))
		require.NoError(t, form.RemoveAll())
		require.NoError(t, replay.Close())
	}
}

func resetAPIRequestHTTPClientState(t *testing.T) {
	t.Helper()

	oldTLSInsecureSkipVerify := common.TLSInsecureSkipVerify
	oldRelayTimeout := common.RelayTimeout

	common.TLSInsecureSkipVerify = false
	common.RelayTimeout = 0
	service.ResetProxyClientCache()
	service.InitHttpClient()

	t.Cleanup(func() {
		common.TLSInsecureSkipVerify = oldTLSInsecureSkipVerify
		common.RelayTimeout = oldRelayTimeout
		service.ResetProxyClientCache()
		service.InitHttpClient()
	})
}

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestDoRequestLeavesNonTLSTransportErrorsUnhinted(t *testing.T) {
	resetAPIRequestHTTPClientState(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	req, err := http.NewRequest(http.MethodGet, "https://example.com/v1/chat/completions", nil)
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{
			Proxy: "http://127.0.0.1:9",
		},
	}

	_, err = doRequest(ctx, req, info)
	require.Error(t, err)

	var newAPIErr *types.NewAPIError
	require.ErrorAs(t, err, &newAPIErr)
	require.Equal(t, "upstream error: do request failed", newAPIErr.Error())
	require.NotContains(t, newAPIErr.Error(), "TLS")
	require.NotContains(t, newAPIErr.Error(), "跳过上游 TLS 证书校验")
}

func TestDoRequestAddsHintOnlyForTLSVerificationErrors(t *testing.T) {
	resetAPIRequestHTTPClientState(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	_, err = doRequest(ctx, req, info)
	require.Error(t, err)

	var newAPIErr *types.NewAPIError
	require.ErrorAs(t, err, &newAPIErr)
	require.Contains(t, newAPIErr.Error(), "跳过上游 TLS 证书校验")
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestApplyDefaultUpstreamUserAgentOpenAI(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeOpenAI,
		},
	}

	applyDefaultUpstreamUserAgent(req, info)
	require.Equal(t, defaultCodexCLIUserAgent, req.Header.Get("User-Agent"))
}

func TestApplyOneHeaderRuleActions(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionSetIfAbsent, Value: "first"})
	require.Equal(t, "first", headers.Get("X-Test"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionSetIfAbsent, Value: "second"})
	require.Equal(t, "first", headers.Get("X-Test"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionReplace, Value: "replaced"})
	require.Equal(t, "replaced", headers.Get("X-Test"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Missing", Action: model.HeaderActionReplace, Value: "nope"})
	require.Empty(t, headers.Get("X-Missing"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionSetFixed, Value: "fixed"})
	require.Equal(t, "fixed", headers.Get("X-Test"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionKeep, Value: "ignored"})
	require.Equal(t, "fixed", headers.Get("X-Test"))

	applyOneHeaderRule(headers, model.HeaderRule{Enabled: true, Name: "X-Test", Action: model.HeaderActionDelete})
	require.Empty(t, headers.Get("X-Test"))
}

func TestResolveHeaderRuleCategoryCodexResponses(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses}
	require.Equal(t, "codex", resolveHeaderRuleCategory(info, req))
}

func TestApplyDefaultUpstreamUserAgentClaude(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeAnthropic,
		},
	}

	applyDefaultUpstreamUserAgent(req, info)
	require.Equal(t, defaultClaudeCLIUserAgent, req.Header.Get("User-Agent"))
}

func TestApplyDefaultUpstreamUserAgentKeepsExistingHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	req.Header.Set("User-Agent", "custom-agent")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeOpenAI,
		},
	}

	applyDefaultUpstreamUserAgent(req, info)
	require.Equal(t, "custom-agent", req.Header.Get("User-Agent"))
}

func TestApplyManagedUpstreamUserAgentOverridesPassthroughUserAgent(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	req.Header.Set("User-Agent", "client-codex-agent")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeOpenAI,
			ChannelSetting: dto.ChannelSettings{
				UserAgentOverride: "managed-codex-agent",
			},
		},
	}

	applyManagedUpstreamUserAgent(req, info)
	require.Equal(t, "managed-codex-agent", req.Header.Get("User-Agent"))
}

func TestShouldForceCodexIdentityHonorsChannelSetting(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	require.True(t, shouldForceCodexIdentity(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{RequiresCodexIdentity: &enabled},
		},
	}, "https://example.com/v1/chat/completions"))

	require.False(t, shouldForceCodexIdentity(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{RequiresCodexIdentity: &disabled},
		},
	}, "https://new.sharedchat.cc/codex/v1/chat/completions"))
}

func TestShouldForceCodexIdentityDefaultsToOpenAIOnly(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{},
		},
	}

	require.True(t, shouldForceCodexIdentity(info, "https://new.sharedchat.cc/codex/v1/chat/completions"))
	require.True(t, shouldForceCodexIdentity(info, "https://example.com/backend-api/codex/responses"))
	require.True(t, shouldForceCodexIdentity(info, "https://api.openai-compatible.test/v1/chat/completions"))

	claudeInfo := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{},
		},
	}
	require.False(t, shouldForceCodexIdentity(claudeInfo, "https://api.anthropic-compatible.test/v1/messages"))
}
