package requestguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type countingEvaluator struct {
	calls  atomic.Int64
	result Result
}

func (e *countingEvaluator) Evaluate(context.Context, Snapshot, *operation_setting.RequestGuardSetting, RequestMeta) Result {
	e.calls.Add(1)
	return e.result
}

func (e *countingEvaluator) EvaluateEndpoint(context.Context, Snapshot, *operation_setting.RequestGuardSetting, string, string) Result {
	e.calls.Add(1)
	return e.result
}

type scannerFunc func(context.Context, Snapshot, operation_setting.RequestGuardEndpoint, string, string) (Result, error)

func (f scannerFunc) Evaluate(ctx context.Context, snapshot Snapshot, endpoint operation_setting.RequestGuardEndpoint, secret, requestHost string) (Result, error) {
	return f(ctx, snapshot, endpoint, secret, requestHost)
}

func requestGuardTestSetting() operation_setting.RequestGuardSetting {
	return operation_setting.RequestGuardSetting{
		Enabled: true, Mode: operation_setting.RequestGuardModeEnforce,
		FailurePolicy: operation_setting.RequestGuardFailureClosed,
		InputMode:     operation_setting.RequestGuardInputFullClientControlled,
		MaxInputRunes: 1024, EvaluationTimeoutMs: 500,
		Scope: operation_setting.RequestGuardScope{
			AllGroups: true, Models: []string{"*"}, Protocols: []string{"openai_chat"},
		},
		Bulkhead: operation_setting.RequestGuardBulkhead{MaxConcurrent: 4, MaxPerEndpoint: 2},
		Observe:  operation_setting.RequestGuardObserve{WorkerCount: 1, QueueCapacity: 4},
		Endpoints: []operation_setting.RequestGuardEndpoint{{
			ID: "primary", Enabled: true, Priority: 100, BaseURL: "https://guard.example",
			Model: "guard", Codec: operation_setting.RequestGuardCodecJSONPolicy,
			TimeoutMs: 250, InputLimitRunes: 1024, ProxyPolicy: operation_setting.RequestGuardProxyDisabled,
		}},
	}
}

func withRequestGuardGlobals(t *testing.T) {
	t.Helper()
	previousSetting := operation_setting.GetRequestGuardSetting()
	previousEvaluator := globalEvaluator
	t.Cleanup(func() {
		operation_setting.ApplyRequestGuardSetting(previousSetting)
		globalEvaluator = previousEvaluator
	})
}

func TestPreflightOffAndScopeMissAvoidEvaluationAndExtraction(t *testing.T) {
	withRequestGuardGlobals(t)
	counter := &countingEvaluator{result: Result{Kind: DecisionBlock}}
	globalEvaluator = counter

	off := requestGuardTestSetting()
	off.Enabled = false
	off.Mode = operation_setting.RequestGuardModeOff
	operation_setting.ApplyRequestGuardSetting(off)
	require.Nil(t, Preflight(nil, nil, nil))
	require.Zero(t, counter.calls.Load())

	scopeMiss := requestGuardTestSetting()
	scopeMiss.Scope.Protocols = []string{"anthropic"}
	operation_setting.ApplyRequestGuardSetting(scopeMiss)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, OriginModelName: "gpt-5", UsingGroup: "default"}
	// A nil request would fail extraction if the scope gate were bypassed.
	require.Nil(t, Preflight(nil, info, nil))
	require.Zero(t, counter.calls.Load())
}

func TestPreflightOffPathDoesNotAllocate(t *testing.T) {
	withRequestGuardGlobals(t)
	off := requestGuardTestSetting()
	off.Enabled = false
	off.Mode = operation_setting.RequestGuardModeOff
	operation_setting.ApplyRequestGuardSetting(off)

	allocations := testing.AllocsPerRun(1000, func() {
		if Preflight(nil, nil, nil) != nil {
			panic("disabled RequestGuard must not reject requests")
		}
	})
	require.Zero(t, allocations)
}

func TestPreflightRejectsRecursiveInternalRequestBeforeEvaluation(t *testing.T) {
	withRequestGuardGlobals(t)
	counter := &countingEvaluator{result: Result{Kind: DecisionAllow}}
	globalEvaluator = counter
	operation_setting.ApplyRequestGuardSetting(requestGuardTestSetting())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set(internalRequestHeader, "1")
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	apiErr := Preflight(ctx, nil, nil)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusLoopDetected, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.Zero(t, counter.calls.Load())
}

func TestExtractTypedProtocolsAndUnicodeTruncation(t *testing.T) {
	openAI := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeText, Text: "你好🙂abc"}, {Type: "image_url", ImageUrl: "data:image/png;base64,secret"}}},
	}}
	snapshot, err := Extract(openAI, nil, 3, operation_setting.RequestGuardInputFullClientControlled)
	require.NoError(t, err)
	require.Equal(t, 3, snapshot.RuneCount)
	require.True(t, snapshot.Truncated)
	require.Equal(t, "你好🙂", snapshot.Segments[0].Text)
	require.NotContains(t, snapshot.Text(), "base64")

	responses := &dto.OpenAIResponsesRequest{
		Instructions: []byte(`"system rule"`),
		Input:        []byte(`[{"role":"user","content":[{"type":"input_text","text":"response text"},{"type":"input_image","image_url":"data:image/png;base64,secret"}]}]`),
	}
	snapshot, err = Extract(responses, nil, 100, operation_setting.RequestGuardInputFullClientControlled)
	require.NoError(t, err)
	require.Contains(t, snapshot.Text(), "system rule")
	require.Contains(t, snapshot.Text(), "response text")
	require.NotContains(t, snapshot.Text(), "base64")

	claude := &dto.ClaudeRequest{System: "claude system", Messages: []dto.ClaudeMessage{{Role: "user", Content: "claude text"}}}
	snapshot, err = Extract(claude, nil, 100, operation_setting.RequestGuardInputFullClientControlled)
	require.NoError(t, err)
	require.Contains(t, snapshot.Text(), "claude system")
	require.Contains(t, snapshot.Text(), "claude text")

	system := dto.GeminiChatContent{Role: "system", Parts: []dto.GeminiPart{{Text: "gemini system"}}}
	gemini := &dto.GeminiChatRequest{SystemInstructions: &system, Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "gemini text"}}}}}
	snapshot, err = Extract(gemini, nil, 100, operation_setting.RequestGuardInputFullClientControlled)
	require.NoError(t, err)
	require.Contains(t, snapshot.Text(), "gemini system")
	require.Contains(t, snapshot.Text(), "gemini text")
}

func TestCodecsDecodeStrictly(t *testing.T) {
	qwen := qwen3GuardCodec{}
	result, err := qwen.Decode("unsafe\ncategories: violence, fraud")
	require.NoError(t, err)
	require.Equal(t, DecisionBlock, result.Kind)
	require.Equal(t, []string{"violence", "fraud"}, result.Categories)
	_, err = qwen.Decode("maybe")
	require.ErrorIs(t, err, ErrInvalidResponse)

	policy := jsonPolicyCodec{}
	result, err = policy.Decode(`{"decision":"flag","categories":["abuse"],"reason_code":"review","confidence":0.75}`)
	require.NoError(t, err)
	require.Equal(t, DecisionFlag, result.Kind)
	require.Equal(t, "review", result.ReasonCode)

	invalid := []string{
		"```json\n{\"decision\":\"allow\",\"categories\":[],\"reason_code\":\"safe\"}\n```",
		`{"decision":"allow","reason_code":"safe"}`,
		`{"decision":"allow","categories":[],"reason_code":"safe","extra":true}`,
		`{"decision":"allow","categories":[],"reason_code":"safe","confidence":2}`,
	}
	for _, content := range invalid {
		_, err := policy.Decode(content)
		require.ErrorIs(t, err, ErrInvalidResponse, content)
	}
}

func TestEvaluatorFailoverDeadlineAndBulkhead(t *testing.T) {
	setting := requestGuardTestSetting()
	setting.Endpoints = append(setting.Endpoints, operation_setting.RequestGuardEndpoint{
		ID: "fallback", Enabled: true, Priority: 50, BaseURL: "https://fallback.example",
		Model: "guard-fallback", Codec: operation_setting.RequestGuardCodecJSONPolicy,
		TimeoutMs: 250, InputLimitRunes: 1024, ProxyPolicy: operation_setting.RequestGuardProxyDisabled,
	})
	var callsMu sync.Mutex
	calls := []string{}
	evaluator := &defaultEvaluator{limits: &bulkhead{perEndpoint: make(map[string]int)}}
	evaluator.scanner = scannerFunc(func(_ context.Context, _ Snapshot, endpoint operation_setting.RequestGuardEndpoint, _, _ string) (Result, error) {
		callsMu.Lock()
		calls = append(calls, endpoint.ID)
		callsMu.Unlock()
		if endpoint.ID == "primary" {
			return Result{}, newScanError(ErrEndpointUnavailable, "network_error", 0)
		}
		return Result{Kind: DecisionAllow, ReasonCode: "safe"}, nil
	})
	result := evaluator.Evaluate(context.Background(), Snapshot{RuneCount: 1}, &setting, RequestMeta{})
	require.Equal(t, DecisionAllow, result.Kind)
	require.Equal(t, "fallback", result.EndpointID)
	require.Equal(t, []string{"primary", "fallback"}, calls)

	timeoutSetting := requestGuardTestSetting()
	timeoutSetting.EvaluationTimeoutMs = 20
	timeoutSetting.Endpoints[0].TimeoutMs = 100
	evaluator.scanner = scannerFunc(func(ctx context.Context, _ Snapshot, _ operation_setting.RequestGuardEndpoint, _, _ string) (Result, error) {
		<-ctx.Done()
		return Result{}, newScanError(ErrEndpointUnavailable, "timeout", 0)
	})
	result = evaluator.Evaluate(context.Background(), Snapshot{RuneCount: 1}, &timeoutSetting, RequestMeta{})
	require.Equal(t, DecisionUnavailable, result.Kind)
	require.Equal(t, "evaluation_timeout", result.ReasonCode)
	require.Equal(t, "timeout", result.ErrorClass)

	bulkheadSetting := requestGuardTestSetting()
	bulkheadSetting.Bulkhead.MaxConcurrent = 1
	bulkheadSetting.Bulkhead.MaxPerEndpoint = 1
	blockedEvaluator := &defaultEvaluator{
		scanner: scannerFunc(func(context.Context, Snapshot, operation_setting.RequestGuardEndpoint, string, string) (Result, error) {
			t.Fatal("scanner must not run while the bulkhead is full")
			return Result{}, nil
		}),
		limits: &bulkhead{global: 1, perEndpoint: map[string]int{"primary": 1}},
	}
	result = blockedEvaluator.Evaluate(context.Background(), Snapshot{RuneCount: 1}, &bulkheadSetting, RequestMeta{})
	require.Equal(t, DecisionUnavailable, result.Kind)
	require.Equal(t, "bulkhead_full", result.ErrorClass)
}

func TestObserveQueueDropAndFailurePolicies(t *testing.T) {
	setting := requestGuardTestSetting()
	setting.Observe.QueueCapacity = 1
	before := CurrentMetrics().ObserveDropped
	queuedJobs.Store(1)
	t.Cleanup(func() {
		queuedJobs.Store(0)
		setQueueDepth(0)
	})
	require.False(t, enqueueObserve(observeJob{Setting: &setting}))
	require.Equal(t, before+1, CurrentMetrics().ObserveDropped)

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:requestguard-policy?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.RequestGuardEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	setting.FailurePolicy = operation_setting.RequestGuardFailureOpen
	require.Nil(t, handleEnforceResult(RequestMeta{Mode: operation_setting.RequestGuardModeEnforce}, Snapshot{}, Result{Kind: DecisionUnavailable, ReasonCode: "timeout"}, &setting))

	setting.FailurePolicy = operation_setting.RequestGuardFailureClosed
	apiErr := handleEnforceResult(RequestMeta{Mode: operation_setting.RequestGuardModeEnforce}, Snapshot{}, Result{Kind: DecisionUnavailable, ReasonCode: "timeout"}, &setting)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))

	blocked := handleEnforceResult(RequestMeta{Mode: operation_setting.RequestGuardModeEnforce}, Snapshot{}, Result{Kind: DecisionBlock, ReasonCode: "unsafe"}, &setting)
	require.NotNil(t, blocked)
	require.Equal(t, http.StatusForbidden, blocked.StatusCode)
	require.True(t, types.IsSkipRetryError(blocked))
}

func TestObserveWorkerStaysReadyUntilProcessShutdown(t *testing.T) {
	previous := processCtx.Load()
	ctx, cancel := context.WithCancel(context.Background())
	SetProcessContext(ctx)
	t.Cleanup(func() {
		cancel()
		if previous != nil {
			SetProcessContext(previous.Context)
		}
	})

	ensureObserveWorkers(getObserveQueue(), 1)
	require.Equal(t, int64(1), CurrentMetrics().Workers)
	time.Sleep(1100 * time.Millisecond)
	require.Equal(t, int64(1), CurrentMetrics().Workers)

	cancel()
	require.Eventually(t, func() bool {
		return CurrentMetrics().Workers == 0
	}, time.Second, 10*time.Millisecond)
}

func TestOpenAICompatibleScannerProbeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer probe-secret", r.Header.Get("Authorization"))
		require.Equal(t, "1", r.Header.Get("X-RequestGuard-Internal"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"decision\":\"allow\",\"categories\":[],\"reason_code\":\"safe\"}"}}]}`))
	}))
	defer server.Close()

	scanner := &openAICompatibleScanner{}
	endpoint := operation_setting.RequestGuardEndpoint{
		ID: "probe", Enabled: true, BaseURL: server.URL, Model: "guard",
		Codec: operation_setting.RequestGuardCodecJSONPolicy, AllowPrivateIP: true,
		ProxyPolicy: operation_setting.RequestGuardProxyDisabled,
	}
	result, err := scanner.Evaluate(context.Background(), Snapshot{Segments: []Segment{{Role: "user", Text: "health check"}}, RuneCount: 12}, endpoint, "probe-secret", "example.com")
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Kind)
	require.Equal(t, http.StatusOK, result.HTTPStatus)
	require.Empty(t, result.ErrorClass)
}

func TestRecursiveGuardEndpointIsBlocked(t *testing.T) {
	scanner := &openAICompatibleScanner{}
	endpoint := operation_setting.RequestGuardEndpoint{
		BaseURL: "https://api.example.com:9443", Model: "guard", Codec: operation_setting.RequestGuardCodecJSONPolicy,
		ProxyPolicy: operation_setting.RequestGuardProxyDisabled,
	}
	_, err := scanner.Evaluate(context.Background(), Snapshot{}, endpoint, "", "API.EXAMPLE.COM:443")
	require.Error(t, err)
	class, _ := scanErrorDetails(err)
	require.Equal(t, "recursive_endpoint", class)
	require.True(t, errors.Is(err, ErrEndpointUnavailable))
}

func TestSnapshotTextIsBounded(t *testing.T) {
	builder := newSnapshotBuilder(5)
	require.False(t, builder.append("user", strings.Repeat("界", 10)))
	snapshot := builder.snapshot()
	require.Equal(t, 5, snapshot.RuneCount)
	require.True(t, snapshot.Truncated)
	require.Equal(t, strings.Repeat("界", 5), snapshot.Segments[0].Text)

	builder = newSnapshotBuilder(5)
	require.True(t, builder.append("user", "界界界界界"))
	require.True(t, builder.append("assistant", "   "))
	require.False(t, builder.append("assistant", "additional text"))
	snapshot = builder.snapshot()
	require.Equal(t, 5, snapshot.RuneCount)
	require.True(t, snapshot.Truncated)
	require.Len(t, snapshot.Segments, 1)
}

func init() {
	gin.SetMode(gin.TestMode)
}
