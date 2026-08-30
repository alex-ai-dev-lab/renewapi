package gemini

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

type geminiStreamCompletionTracker struct {
	expected int
	seen     map[int64]struct{}
	finished map[int64]struct{}
}

func newGeminiStreamCompletionTracker(info *relaycommon.RelayInfo) *geminiStreamCompletionTracker {
	return &geminiStreamCompletionTracker{
		expected: expectedGeminiStreamCandidateCount(info),
		seen:     make(map[int64]struct{}),
		finished: make(map[int64]struct{}),
	}
}

func expectedGeminiStreamCandidateCount(info *relaycommon.RelayInfo) int {
	if info != nil {
		if req, ok := info.Request.(*dto.GeminiChatRequest); ok && req != nil &&
			req.GenerationConfig.CandidateCount != nil && *req.GenerationConfig.CandidateCount > 0 {
			return *req.GenerationConfig.CandidateCount
		}
	}
	return 1
}

func (t *geminiStreamCompletionTracker) Observe(response *dto.GeminiChatResponse) {
	if t == nil || response == nil {
		return
	}
	for _, candidate := range response.Candidates {
		t.seen[candidate.Index] = struct{}{}
		if candidate.FinishReason == nil {
			continue
		}
		reason := strings.TrimSpace(*candidate.FinishReason)
		if reason == "" || strings.EqualFold(reason, "FINISH_REASON_UNSPECIFIED") {
			continue
		}
		t.finished[candidate.Index] = struct{}{}
	}
}

func (t *geminiStreamCompletionTracker) Complete() bool {
	if t == nil || len(t.seen) == 0 || len(t.finished) < t.expected {
		return false
	}
	for index := range t.seen {
		if _, ok := t.finished[index]; !ok {
			return false
		}
	}
	return true
}

func (t *geminiStreamCompletionTracker) Summary() string {
	if t == nil {
		return "expected=1 seen=0 finished=0"
	}
	return fmt.Sprintf("expected=%d seen=%d finished=%d", t.expected, len(t.seen), len(t.finished))
}

func geminiStreamOutcomeError(info *relaycommon.RelayInfo, tracker *geminiStreamCompletionTracker, promptBlockReason string, callbackStopped bool) *types.NewAPIError {
	if promptBlockReason != "" {
		return types.NewOpenAIError(
			errors.New("request blocked by Gemini API: "+promptBlockReason),
			types.ErrorCodePromptBlocked,
			http.StatusBadRequest,
		)
	}
	if callbackStopped {
		// Existing callbacks use false for downstream write failures. Keep those
		// client-scoped failures out of provider health/retry classification.
		return nil
	}
	if info == nil || info.StreamStatus == nil {
		return types.NewOpenAIError(
			errors.New("missing Gemini stream status"),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	outcome := info.StreamStatus.Outcome()
	switch outcome.Code {
	case relaycommon.StreamAttemptClientGone, relaycommon.StreamAttemptWriteError:
		return nil
	case relaycommon.StreamAttemptEmptyResponse:
		return types.NewOpenAIError(
			fmt.Errorf("empty Gemini stream response from upstream: %s", outcome.Summary),
			types.ErrorCodeEmptyResponse,
			http.StatusBadGateway,
		)
	case relaycommon.StreamAttemptBadResponseBody, relaycommon.StreamAttemptIncomplete, relaycommon.StreamAttemptFailed:
		return types.NewOpenAIError(
			fmt.Errorf("invalid Gemini stream response from upstream: %s", outcome.Summary),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	case relaycommon.StreamAttemptTimeout:
		if outcome.TransportEnd == relaycommon.StreamEndReasonFirstByteTimeout {
			return types.NewOpenAIError(
				fmt.Errorf("Gemini stream first byte timeout after %ds", common.RelayFirstByteTimeout),
				types.ErrorCodeChannelResponseTimeExceeded,
				http.StatusGatewayTimeout,
			)
		}
		return types.NewOpenAIError(
			fmt.Errorf("Gemini stream timeout: %s", outcome.Summary),
			types.ErrorCodeChannelResponseTimeExceeded,
			http.StatusGatewayTimeout,
		)
	}

	if !tracker.Complete() {
		return types.NewOpenAIError(
			fmt.Errorf("incomplete Gemini stream response from upstream: %s; %s", outcome.Summary, tracker.Summary()),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	return nil
}
