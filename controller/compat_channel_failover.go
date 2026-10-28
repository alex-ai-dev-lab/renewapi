package controller

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
)

const (
	compatUpstream5xxFailureThreshold = 3
	compatUpstream5xxFailureTTL       = 10 * time.Minute
)

type compatChannelFailureState struct {
	count       int
	lastFailure time.Time
}

var compatChannelFailureTracker = struct {
	sync.Mutex
	items map[string]compatChannelFailureState
}{
	items: make(map[string]compatChannelFailureState),
}

func compatChannelFailureKey(channelID int, info *relaycommon.RelayInfo) string {
	if info == nil {
		return fmt.Sprintf("%d", channelID)
	}
	return fmt.Sprintf("%d|%s|%d|%t", channelID, info.OriginModelName, info.RelayMode, info.IsStream)
}

func clearCompatChannelFailure(channelID int, info *relaycommon.RelayInfo) {
	compatChannelFailureTracker.Lock()
	defer compatChannelFailureTracker.Unlock()
	delete(compatChannelFailureTracker.items, compatChannelFailureKey(channelID, info))
}

func recordCompatChannelFailure(channelID int, info *relaycommon.RelayInfo) int {
	now := time.Now()
	key := compatChannelFailureKey(channelID, info)

	compatChannelFailureTracker.Lock()
	defer compatChannelFailureTracker.Unlock()

	state := compatChannelFailureTracker.items[key]
	if !state.lastFailure.IsZero() && now.Sub(state.lastFailure) > compatUpstream5xxFailureTTL {
		state.count = 0
	}
	state.count++
	state.lastFailure = now
	compatChannelFailureTracker.items[key] = state
	return state.count
}

func shouldTrackCompatUpstream5xxFailure(openaiErr *types.NewAPIError) bool {
	if openaiErr == nil {
		return false
	}
	if service.IsTLSVerificationError(openaiErr) || service.IsInvalidEncryptedReasoningError(openaiErr) {
		return false
	}
	switch openaiErr.StatusCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
