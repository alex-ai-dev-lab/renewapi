package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func withSessionRecoverySetting(t *testing.T) *operation_setting.StreamRecoverySetting {
	t.Helper()
	setting := operation_setting.GetStreamRecoverySetting()
	original := *setting
	setting.SessionRouteRepairEnabled = true
	setting.PostCommitRouteRepairEnabled = true
	setting.PreCommitRetryEnabled = true
	setting.MaxCrossRequestRouteChanges = 2
	t.Cleanup(func() { *setting = original })
	return setting
}

func TestDecideSessionRecoveryCommittedTransientRepairsNextRequestOnly(t *testing.T) {
	withSessionRecoverySetting(t)
	outcome := relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}
	err := types.WithOpenAIError(types.OpenAIError{Type: "server_error", Code: "server_error", Message: "overloaded"}, http.StatusBadGateway)
	decision := DecideSessionRecovery(outcome, err, &dto.OpenAIResponsesRequest{}, false, false)
	require.Equal(t, SessionFailureTransient, decision.FailureClass)
	require.False(t, decision.RetryCurrentRequest)
	require.True(t, decision.EvictAffinity)
	require.True(t, decision.MarkChannelNegative)
	require.Equal(t, "session_channel", decision.FailureScope)
}

func TestDecideSessionRecoveryDoesNotFailoverStateBoundRequest(t *testing.T) {
	withSessionRecoverySetting(t)
	outcome := relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}
	err := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	decision := DecideSessionRecovery(outcome, err, &dto.OpenAIResponsesRequest{PreviousResponseID: "resp_123"}, false, false)
	require.True(t, decision.StatefulRequest)
	require.False(t, decision.EvictAffinity)
	require.False(t, decision.MarkChannelNegative)
	require.Equal(t, "state_bound_no_failover", decision.Reason)
}

func TestDecideSessionRecoveryDoesNotFailoverEncryptedContinuation(t *testing.T) {
	withSessionRecoverySetting(t)
	outcome := relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}
	err := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	for _, request := range []dto.Request{
		&dto.OpenAIResponsesRequest{Input: common.StringToByteSlice(`[{"type":"reasoning","encrypted_content":"opaque"}]`)},
		&dto.OpenAIResponsesCompactionRequest{Input: common.StringToByteSlice(`[{"type":"compaction_summary","encrypted_content":"opaque"}]`)},
	} {
		decision := DecideSessionRecovery(outcome, err, request, false, false)
		require.True(t, decision.StatefulRequest)
		require.False(t, decision.EvictAffinity)
		require.False(t, decision.MarkChannelNegative)
		require.Equal(t, "state_bound_no_failover", decision.Reason)
	}
}

func TestDecideSessionRecoverySkipsDeterministicAndClientFailures(t *testing.T) {
	withSessionRecoverySetting(t)
	invalid := types.WithOpenAIError(types.OpenAIError{Type: "invalid_request_error", Code: "invalid_request", Message: "bad input"}, http.StatusBadGateway)
	decision := DecideSessionRecovery(relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}, invalid, &dto.OpenAIResponsesRequest{}, false, false)
	require.Equal(t, SessionFailureDeterministic, decision.FailureClass)
	require.False(t, decision.EvictAffinity)

	client := DecideSessionRecovery(relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptWriteError, ClientCommitted: true}, nil, &dto.OpenAIResponsesRequest{}, false, false)
	require.Equal(t, SessionFailureClient, client.FailureClass)
	require.False(t, client.MarkChannelNegative)
}

func TestDecideSessionRecoveryMultiKeyUsesKeyScopeFirst(t *testing.T) {
	withSessionRecoverySetting(t)
	outcome := relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}
	err := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	decision := DecideSessionRecovery(outcome, err, &dto.OpenAIResponsesRequest{}, true, true)
	require.True(t, decision.MarkKeyNegative)
	require.False(t, decision.MarkChannelNegative)
	require.False(t, decision.EvictAffinity)
	require.Greater(t, decision.NegativeTTL, time.Duration(0))
}

func TestUnknownBadResponseUsesShortTTL(t *testing.T) {
	setting := withSessionRecoverySetting(t)
	setting.SessionNegativeTTLSeconds = 90
	setting.UnknownFailureNegativeTTLSeconds = 30
	err := types.NewOpenAIError(errors.New("responses stream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	decision := DecideSessionRecovery(relaycommon.StreamAttemptOutcome{Code: relaycommon.StreamAttemptFailed, ClientCommitted: true}, err, &dto.OpenAIResponsesRequest{}, false, false)
	require.Equal(t, SessionFailureUnknown, decision.FailureClass)
	require.Equal(t, 30*time.Second, decision.NegativeTTL)
}

func TestSessionRecoveryBudgetIsBoundedAndDeduplicated(t *testing.T) {
	withSessionRecoverySetting(t)
	key := "recovery-budget-" + time.Now().Format("150405.000000000")
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60})
	t.Cleanup(func() { _, _ = getChannelAffinityRecoveryChainCache().DeleteMany([]string{key}) })

	used, max, allowed := consumeSessionRecoveryBudget(ctx, 201, "failure-a")
	require.True(t, allowed)
	require.Equal(t, 1, used)
	require.Equal(t, 2, max)
	used, _, allowed = consumeSessionRecoveryBudget(ctx, 201, "failure-a")
	require.True(t, allowed)
	require.Equal(t, 1, used)
	used, _, allowed = consumeSessionRecoveryBudget(ctx, 205, "failure-b")
	require.True(t, allowed)
	require.Equal(t, 2, used)
	used, _, allowed = consumeSessionRecoveryBudget(ctx, 71, "failure-c")
	require.False(t, allowed)
	require.Equal(t, 2, used)
}

func TestObserveSessionRecoverySuccessKeepsChainForMiddlewareCleanup(t *testing.T) {
	setting := withSessionRecoverySetting(t)
	setting.MaxCrossRequestRouteChanges = 2
	key := "recovery-success-" + time.Now().Format("150405.000000000")
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60})
	chain := sessionRecoveryChain{FirstFailureAt: time.Now().Unix(), RouteChanges: 1, LastChannelID: 201, LastFailureToken: "failure-a"}
	require.NoError(t, getChannelAffinityRecoveryChainCache().SetWithTTL(key, chain, time.Minute))
	t.Cleanup(func() { _, _ = getChannelAffinityRecoveryChainCache().DeleteMany([]string{key}) })

	require.True(t, ObserveSessionRecoverySuccess(ctx, 205))
	info, ok := SessionRecoveryLogInfo(ctx)
	require.True(t, ok)
	require.Equal(t, "recovered_next_request", info["action"])
	require.Equal(t, "success", info["result"])
	require.Equal(t, 201, info["previous_channel"])
	require.Equal(t, 205, info["channel_id"])
	_, found, err := getChannelAffinityRecoveryChainCache().Get(key)
	require.NoError(t, err)
	require.True(t, found)
}

func TestCommittedMultiKeyKeyRepairDoesNotConsumeCrossChannelBudget(t *testing.T) {
	withSessionRecoverySetting(t)
	key := "key-only-budget-" + time.Now().Format("150405.000000000")
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60})
	common.SetContextKey(ctx, constant.ContextKeyChannelIsMultiKey, true)
	common.SetContextKey(ctx, constant.ContextKeyChannelMultiKeyIndex, 0)
	status := relaycommon.NewStreamStatus()
	status.MarkClientCommitted()
	result := ApplySessionRecovery(ctx, &relaycommon.RelayInfo{RequestId: "key-only", StreamStatus: status}, &model.Channel{Id: 201}, SessionRecoveryDecision{
		FailureClass: SessionFailureTransient, FailureScope: "session_channel_key", MarkKeyNegative: true, NegativeTTL: time.Minute,
	})
	require.True(t, result.Applied)
	require.True(t, result.KeyNegative)
	require.Zero(t, result.BudgetUsed)
	_, found, err := getChannelAffinityRecoveryChainCache().Get(key)
	require.NoError(t, err)
	require.False(t, found)
}

func TestLegacyFailureCannotPoisonConcurrentV2Success(t *testing.T) {
	withSessionRecoverySetting(t)
	key := "legacy-concurrent-success-" + time.Now().Format("150405.000000000")
	legacyCache := getChannelAffinityCache()
	v2Cache := getChannelAffinityRecordCache()
	require.NoError(t, legacyCache.SetWithTTL(key, 201, time.Minute))
	t.Cleanup(func() {
		_, _ = legacyCache.DeleteMany([]string{key})
		_, _ = v2Cache.DeleteMany([]string{key})
		clearChannelSessionNegative(buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key}), 201)
		_, _ = getChannelAffinityRecoveryChainCache().DeleteMany([]string{key})
	})
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60})
	legacyRecord, found, err := getAffinityRecord(key)
	require.NoError(t, err)
	require.True(t, found)
	rememberMatchedAffinity(ctx, key, legacyRecord)
	newRecord := ChannelAffinityRecord{ChannelID: 201, Generation: "new-success", RecordedAt: time.Now().UnixNano()}
	require.NoError(t, v2Cache.SetWithTTL(key, newRecord, time.Minute))

	status := relaycommon.NewStreamStatus()
	status.MarkClientCommitted()
	result := ApplySessionRecovery(ctx, &relaycommon.RelayInfo{RequestId: "legacy-failure", StreamStatus: status}, &model.Channel{Id: 201}, SessionRecoveryDecision{
		FailureClass: SessionFailureTransient, FailureScope: "session_channel", EvictAffinity: true, MarkChannelNegative: true, NegativeTTL: time.Minute,
	})
	require.True(t, result.AffinityCASMiss)
	require.False(t, result.ChannelNegative)
	require.False(t, ShouldAvoidChannelForSession(ctx, 201))
	current, found, err := v2Cache.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, newRecord, current)
	_, chainFound, err := getChannelAffinityRecoveryChainCache().Get(key)
	require.NoError(t, err)
	require.False(t, chainFound)
}

func TestPreCommitRecoveryCanMarkSequentialFailedChannels(t *testing.T) {
	withSessionRecoverySetting(t)
	key := "sequential-precommit-" + time.Now().Format("150405.000000000")
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60})
	record := ChannelAffinityRecord{ChannelID: 201, Generation: "initial", RecordedAt: time.Now().UnixNano()}
	require.NoError(t, getChannelAffinityRecordCache().SetWithTTL(key, record, time.Minute))
	require.NoError(t, getChannelAffinityCache().SetWithTTL(key, 201, time.Minute))
	rememberMatchedAffinity(ctx, key, record)
	t.Cleanup(func() {
		_, _ = getChannelAffinityRecordCache().DeleteMany([]string{key})
		_, _ = getChannelAffinityCache().DeleteMany([]string{key})
		clearChannelSessionNegative(ctx, 201)
		clearChannelSessionNegative(ctx, 205)
	})
	decision := SessionRecoveryDecision{
		FailureClass: SessionFailureProtocol, FailureScope: "session_channel", EvictAffinity: true, MarkChannelNegative: true, NegativeTTL: time.Minute,
	}
	info := &relaycommon.RelayInfo{RequestId: "sequential-precommit", StreamStatus: relaycommon.NewStreamStatus()}
	first := ApplySessionRecovery(ctx, info, &model.Channel{Id: 201}, decision)
	require.True(t, first.AffinityEvicted)
	require.True(t, first.ChannelNegative)
	second := ApplySessionRecovery(ctx, info, &model.Channel{Id: 205}, decision)
	require.False(t, second.AffinityCASMiss)
	require.True(t, second.ChannelNegative)
	require.True(t, ShouldAvoidChannelForSession(ctx, 201))
	require.True(t, ShouldAvoidChannelForSession(ctx, 205))
}

func TestResolveChannelAffinityMultiKeySkipsSessionNegativeKey(t *testing.T) {
	withSessionRecoverySetting(t)
	key := "key-negative-" + time.Now().Format("150405.000000000")
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{CacheKey: channelAffinityCacheNamespace + ":" + key, TTLSeconds: 60, KeyFingerprint: "fp"})
	common.SetContextKey(ctx, constant.ContextKeyChannelPreferredMultiKeyChannelId, 75)
	common.SetContextKey(ctx, constant.ContextKeyChannelPreferredMultiKeyIndex, 0)
	require.True(t, MarkChannelSessionKeyNegative(ctx, 75, 0))
	t.Cleanup(func() {
		negativeKey := channelAffinityKeyNegativeKey(ctx, 75, 0)
		_, _ = getChannelAffinityKeyNegativeCache().DeleteMany([]string{negativeKey})
	})
	channel := &model.Channel{Id: 75, Key: "key-a\nkey-b", ChannelInfo: model.ChannelInfo{IsMultiKey: true}}
	selected, index, used, err := ResolveChannelAffinityMultiKey(ctx, channel)
	require.Nil(t, err)
	require.True(t, used)
	require.Equal(t, 1, index)
	require.Equal(t, "key-b", selected)
}
