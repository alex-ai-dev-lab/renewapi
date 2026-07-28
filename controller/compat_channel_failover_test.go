package controller

import (
	"errors"
	"net/http"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func resetCompatChannelFailureTrackerForTest() {
	compatChannelFailureTracker.Lock()
	defer compatChannelFailureTracker.Unlock()
	compatChannelFailureTracker.items = make(map[string]compatChannelFailureState)
}

func TestCompatUpstream5xxFailureThreshold(t *testing.T) {
	resetCompatChannelFailureTrackerForTest()
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-7",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
	}
	err := types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)

	if !shouldTrackCompatUpstream5xxFailure(err) {
		t.Fatal("502 should be tracked as transient upstream failure")
	}
	for i := 1; i <= compatUpstream5xxFailureThreshold; i++ {
		if got := recordCompatChannelFailure(52, info); got != i {
			t.Fatalf("failure count after attempt %d = %d, want %d", i, got, i)
		}
	}
}

func TestCompatChannelFailureSuccessClearsCount(t *testing.T) {
	resetCompatChannelFailureTrackerForTest()
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-7",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
	}

	recordCompatChannelFailure(56, info)
	clearCompatChannelFailure(56, info)

	if got := recordCompatChannelFailure(56, info); got != 1 {
		t.Fatalf("failure count after success clear = %d, want 1", got)
	}
}

func TestCompatChannelFailureTTLResetsCount(t *testing.T) {
	resetCompatChannelFailureTrackerForTest()
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-7",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
	}
	key := compatChannelFailureKey(52, info)

	recordCompatChannelFailure(52, info)
	compatChannelFailureTracker.Lock()
	compatChannelFailureTracker.items[key] = compatChannelFailureState{
		count:       1,
		lastFailure: time.Now().Add(-compatUpstream5xxFailureTTL - time.Second),
	}
	compatChannelFailureTracker.Unlock()

	if got := recordCompatChannelFailure(52, info); got != 1 {
		t.Fatalf("failure count after TTL expiry = %d, want 1", got)
	}
}

func TestCompatStreamRetryError_ClaudeEmptyAssistantNormalEndRetries(t *testing.T) {
	status := relaycommon.NewStreamStatus()
	status.RecordError("empty claude assistant stream")
	status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	info := &relaycommon.RelayInfo{
		IsStream:          true,
		RelayFormat:       types.RelayFormatClaude,
		StreamStatus:      status,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{},
	}

	err := compatStreamRetryError(info)
	if err == nil {
		t.Fatal("empty claude assistant stream should trigger retry")
	}
	if err.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", err.StatusCode, http.StatusBadGateway)
	}
}

func TestShouldCompatDisableChannel_DoesNotImmediatelyDisableTransientTransport502(t *testing.T) {
	err := types.NewOpenAIError(errors.New("do request failed: unexpected EOF"), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)

	if shouldCompatDisableChannel(err) {
		t.Fatal("transient transport 502 should be tracked/excluded first, not auto-disabled immediately")
	}
	if !shouldTrackCompatUpstream5xxFailure(err) {
		t.Fatal("transient transport 502 should still be tracked for repeated-failure auto-disable")
	}
}

func responsesStreamInfoForTest(status *relaycommon.StreamStatus) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:     true,
		RelayFormat:  types.RelayFormatOpenAIResponses,
		StreamStatus: status,
	}
}

func TestCompatStreamRetryErrorResponsesOutcomeMapping(t *testing.T) {
	empty := relaycommon.NewStreamStatus()
	empty.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	empty.SetTransportEnd(relaycommon.StreamEndReasonEOF, nil)
	empty.Finalize()
	err := compatStreamRetryError(responsesStreamInfoForTest(empty))
	if err == nil || err.GetErrorCode() != types.ErrorCodeEmptyResponse || err.StatusCode != http.StatusBadGateway {
		t.Fatalf("empty outcome mapping = %#v", err)
	}

	incomplete := relaycommon.NewStreamStatus()
	incomplete.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	incomplete.AcceptEvent("response.created")
	incomplete.SetTransportEnd(relaycommon.StreamEndReasonEOF, nil)
	incomplete.Finalize()
	err = compatStreamRetryError(responsesStreamInfoForTest(incomplete))
	if err == nil || err.GetErrorCode() != types.ErrorCodeBadResponseBody || err.StatusCode != http.StatusBadGateway {
		t.Fatalf("incomplete outcome mapping = %#v", err)
	}

	timedOut := relaycommon.NewStreamStatus()
	timedOut.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	timedOut.SetTransportEnd(relaycommon.StreamEndReasonFirstByteTimeout, nil)
	timedOut.Finalize()
	err = compatStreamRetryError(responsesStreamInfoForTest(timedOut))
	if err == nil || err.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("first-byte timeout mapping = %#v", err)
	}
}

func TestShouldRetrySessionScopedStreamHonorsFlagLimitAndCommit(t *testing.T) {
	setting := operation_setting.GetStreamRecoverySetting()
	old := *setting
	t.Cleanup(func() { *setting = old })
	setting.Enabled = true
	setting.EmptyStreamRetryLimit = 1

	status := relaycommon.NewStreamStatus()
	status.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	status.SetTransportEnd(relaycommon.StreamEndReasonEOF, nil)
	status.Finalize()
	info := responsesStreamInfoForTest(status)
	if !shouldRetrySessionScopedStream(info, 0) {
		t.Fatal("enabled pre-commit empty stream should retry")
	}
	if shouldRetrySessionScopedStream(info, 1) {
		t.Fatal("retry limit must be enforced")
	}
	setting.Enabled = false
	if shouldRetrySessionScopedStream(info, 0) {
		t.Fatal("disabled recovery must not retry")
	}
	setting.Enabled = true
	status.MarkClientCommitted()
	if shouldRetrySessionScopedStream(info, 0) {
		t.Fatal("committed response must never retry")
	}
}

func TestClientScopedStreamFailureIsNotSessionRecovery(t *testing.T) {
	status := relaycommon.NewStreamStatus()
	status.SetPolicy(relaycommon.StreamTerminationPolicyOpenAIResponses)
	status.SetTransportEnd(relaycommon.StreamEndReasonWriteError, errors.New("broken pipe"))
	status.Finalize()
	info := responsesStreamInfoForTest(status)
	require.True(t, isClientScopedStreamFailure(info))
	require.False(t, isSessionScopedStreamFailure(info))
	require.False(t, shouldRetrySessionScopedStream(info, 0))
}
