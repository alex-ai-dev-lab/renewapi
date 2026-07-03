package controller

import (
	"errors"
	"net/http"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
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
