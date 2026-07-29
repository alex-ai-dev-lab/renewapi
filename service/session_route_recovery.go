package service

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type SessionFailureClass string

const (
	SessionFailureNone          SessionFailureClass = "none"
	SessionFailureTransient     SessionFailureClass = "transient_upstream"
	SessionFailureProtocol      SessionFailureClass = "protocol_failure"
	SessionFailureDeterministic SessionFailureClass = "deterministic_request"
	SessionFailureClient        SessionFailureClass = "client_failure"
	SessionFailureUnknown       SessionFailureClass = "unknown_upstream"
)

type SessionRecoveryDecision struct {
	FailureClass        SessionFailureClass
	FailureScope        string
	RetryCurrentRequest bool
	EvictAffinity       bool
	MarkChannelNegative bool
	MarkKeyNegative     bool
	CountChannelHealth  bool
	NegativeTTL         time.Duration
	StatefulRequest     bool
	Reason              string
}

type SessionRecoveryResult struct {
	Applied         bool
	AffinityEvicted bool
	AffinityCASMiss bool
	ChannelNegative bool
	KeyNegative     bool
	BudgetUsed      int
	BudgetMax       int
	BudgetExhausted bool
	Action          string
	BillingPolicy   string
}

func responsesRequestStateBound(request dto.Request) bool {
	var previousResponseID string
	var conversation, input []byte
	switch req := request.(type) {
	case *dto.OpenAIResponsesRequest:
		if req == nil {
			return false
		}
		previousResponseID = req.PreviousResponseID
		conversation = req.Conversation
		input = req.Input
	case *dto.OpenAIResponsesCompactionRequest:
		if req == nil {
			return false
		}
		previousResponseID = req.PreviousResponseID
		input = req.Input
	default:
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" {
		return true
	}
	conversationText := strings.TrimSpace(string(conversation))
	if conversationText != "" && conversationText != "null" && conversationText != "{}" {
		return true
	}
	return hasEncryptedContentJSON(input) || hasReasoningItemJSON(input)
}

func classifySessionFailure(outcome relaycommon.StreamAttemptOutcome, relayErr *types.NewAPIError) SessionFailureClass {
	if outcome.Code == relaycommon.StreamAttemptClientGone || outcome.Code == relaycommon.StreamAttemptWriteError {
		return SessionFailureClient
	}
	if relayErr == nil {
		return SessionFailureUnknown
	}
	code := strings.ToLower(strings.TrimSpace(string(relayErr.GetErrorCode())))
	message := strings.ToLower(relayErr.Error())
	for _, marker := range []string{"invalid_request", "context_length", "content_policy", "moderation", "sensitive", "permission", "model_not_found"} {
		if strings.Contains(code, marker) || strings.Contains(message, marker) {
			return SessionFailureDeterministic
		}
	}
	if relayErr.GetErrorCode() == types.ErrorCodeBadResponse {
		return SessionFailureUnknown
	}
	if relayErr.StatusCode >= http.StatusBadRequest && relayErr.StatusCode < http.StatusInternalServerError &&
		relayErr.StatusCode != http.StatusRequestTimeout && relayErr.StatusCode != http.StatusConflict &&
		relayErr.StatusCode != http.StatusTooEarly && relayErr.StatusCode != http.StatusTooManyRequests {
		return SessionFailureDeterministic
	}
	if relayErr.StatusCode == http.StatusTooManyRequests || relayErr.StatusCode >= http.StatusInternalServerError ||
		strings.Contains(code, "overload") || strings.Contains(code, "rate_limit") || strings.Contains(code, "server_error") {
		return SessionFailureTransient
	}
	if outcome.Code == relaycommon.StreamAttemptBadResponseBody || outcome.Code == relaycommon.StreamAttemptIncomplete {
		return SessionFailureProtocol
	}
	return SessionFailureUnknown
}

func DecideSessionRecovery(outcome relaycommon.StreamAttemptOutcome, relayErr *types.NewAPIError, request dto.Request, isMultiKey, hasUntriedKey bool) SessionRecoveryDecision {
	setting := operation_setting.GetStreamRecoverySetting()
	class := classifySessionFailure(outcome, relayErr)
	decision := SessionRecoveryDecision{FailureClass: class, FailureScope: "session_channel", StatefulRequest: responsesRequestStateBound(request)}
	if class == SessionFailureClient {
		decision.FailureScope, decision.Reason = "client", "client_failure"
		return decision
	}
	if class == SessionFailureDeterministic {
		decision.FailureScope, decision.Reason = "request", "deterministic_request"
		return decision
	}
	if decision.StatefulRequest {
		decision.FailureScope, decision.Reason = "state_bound", "state_bound_no_failover"
		return decision
	}
	if setting == nil || !setting.SessionRouteRepairEnabled {
		decision.Reason = "route_repair_disabled"
		return decision
	}
	if outcome.ClientCommitted && !setting.PostCommitRouteRepairEnabled {
		decision.Reason = "post_commit_repair_disabled"
		return decision
	}
	decision.RetryCurrentRequest = !outcome.ClientCommitted && outcome.RetryableBeforeCommit && setting.PreCommitRetryOn()
	decision.MarkKeyNegative = isMultiKey
	decision.MarkChannelNegative = !isMultiKey || !hasUntriedKey
	decision.EvictAffinity = decision.MarkChannelNegative
	if decision.MarkKeyNegative && !decision.MarkChannelNegative {
		decision.FailureScope = "session_channel_key"
	}
	decision.CountChannelHealth = class == SessionFailureTransient || class == SessionFailureProtocol
	ttl := setting.SessionNegativeTTLSeconds
	if class == SessionFailureUnknown && setting.UnknownFailureNegativeTTLSeconds > 0 {
		ttl = setting.UnknownFailureNegativeTTLSeconds
	}
	if ttl <= 0 {
		ttl = 30
	}
	decision.NegativeTTL = time.Duration(ttl) * time.Second
	decision.Reason = "session_route_repair"
	return decision
}

func consumeSessionRecoveryBudget(c *gin.Context, channelID int, failureToken string) (used, max int, allowed bool) {
	setting := operation_setting.GetStreamRecoverySetting()
	if setting == nil {
		return 0, 0, false
	}
	max = setting.MaxCrossRequestRouteChanges
	if max <= 0 {
		return 0, max, false
	}
	windowSeconds := setting.RecoveryChainWindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	key := currentChannelAffinityKey(c)
	if key == "" {
		return 0, max, false
	}
	cache := getChannelAffinityRecoveryChainCache()
	for attempts := 0; attempts < 8; attempts++ {
		current, found, err := cache.Get(key)
		if err != nil {
			common.SysError(fmt.Sprintf("session recovery budget get failed: %v", err))
			return 0, max, false
		}
		if found && current.LastChannelID == channelID && current.LastFailureToken == failureToken {
			return current.RouteChanges, max, current.RouteChanges <= max
		}
		if found && current.RouteChanges >= max {
			return current.RouteChanges, max, false
		}
		next := current
		if !found {
			next.FirstFailureAt = time.Now().Unix()
		}
		next.RouteChanges++
		next.LastChannelID = channelID
		next.LastFailureToken = failureToken
		var expected *sessionRecoveryChain
		if found {
			expected = &current
		}
		swapped, swapErr := cache.CompareAndSwap(key, expected, next, time.Duration(windowSeconds)*time.Second, sessionRecoveryChainEqual)
		if swapErr != nil {
			common.SysError(fmt.Sprintf("session recovery budget update failed: %v", swapErr))
			return 0, max, false
		}
		if swapped {
			return next.RouteChanges, max, true
		}
	}
	return 0, max, false
}

func affinityObservationStillCurrent(c *gin.Context, observed ChannelAffinityRecord, observedOK bool) bool {
	key := currentChannelAffinityKey(c)
	if key == "" {
		return false
	}
	current, found, err := getAffinityRecord(key)
	if err != nil {
		common.SysError(fmt.Sprintf("session recovery affinity recheck failed: %v", err))
		return false
	}
	if !observedOK {
		return !found
	}
	return found && affinityRecordEqual(current, observed)
}

func ApplySessionRecovery(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel, decision SessionRecoveryDecision) SessionRecoveryResult {
	result := SessionRecoveryResult{Action: decision.Reason}
	if c == nil || info == nil || channel == nil || (!decision.EvictAffinity && !decision.MarkChannelNegative && !decision.MarkKeyNegative) {
		recordSessionRecoveryResult(c, decision, result)
		return result
	}
	matched, matchedObserved := observedAffinityRecord(c)
	if matchedObserved && matched.ChannelID != channel.Id {
		result.AffinityCASMiss, result.Action = true, "affinity_cas_miss"
		recordSessionRecoveryResult(c, decision, result)
		return result
	}
	if !affinityObservationStillCurrent(c, matched, matchedObserved) {
		result.AffinityCASMiss, result.Action = true, "affinity_cas_miss"
		recordSessionRecoveryResult(c, decision, result)
		return result
	}
	usesCrossChannelBudget := decision.EvictAffinity || decision.MarkChannelNegative
	if info.ClientResponseCommitted() && usesCrossChannelBudget {
		failureToken := affinityFingerprint(fmt.Sprintf("%s:%d:%s", info.RequestId, channel.Id, matched.Generation))
		result.BudgetUsed, result.BudgetMax, result.Applied = consumeSessionRecoveryBudget(c, channel.Id, failureToken)
		if !result.Applied {
			result.BudgetExhausted, result.Action = true, "recovery_budget_exhausted"
			recordSessionRecoveryResult(c, decision, result)
			return result
		}
	} else {
		result.Applied = true
	}
	if decision.MarkKeyNegative && common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		result.KeyNegative = MarkChannelSessionKeyNegative(c, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex))
	}
	if decision.EvictAffinity {
		result.AffinityEvicted = EvictCurrentChannelAffinityIfMatches(c, channel.Id)
		result.AffinityCASMiss = matchedObserved && !result.AffinityEvicted
	}
	if decision.MarkChannelNegative && (!matchedObserved || result.AffinityEvicted) {
		result.ChannelNegative = MarkChannelSessionNegativeWithTTL(c, channel.Id, decision.NegativeTTL)
	}
	if result.AffinityEvicted {
		// The cache key remains useful for this request's later attempts, but the
		// consumed generation must not be compared against a different channel.
		c.Set(ginKeyChannelAffinityMatchedV2, nil)
	}
	if info.ClientResponseCommitted() {
		result.BillingPolicy = "committed_failure_refund_once"
	}
	result.Action = "route_repaired"
	recordSessionRecoveryResult(c, decision, result)
	return result
}

func recordSessionRecoveryResult(c *gin.Context, decision SessionRecoveryDecision, result SessionRecoveryResult) {
	if c == nil {
		return
	}
	c.Set(ginKeySessionRecoveryLogInfo, map[string]interface{}{
		"failure_class": decision.FailureClass, "failure_scope": decision.FailureScope,
		"action": result.Action, "affinity_evicted": result.AffinityEvicted,
		"affinity_cas_miss": result.AffinityCASMiss, "channel_negative": result.ChannelNegative,
		"key_negative": result.KeyNegative, "budget_used": result.BudgetUsed, "budget_max": result.BudgetMax,
		"billing_policy": result.BillingPolicy,
	})
}

func ObserveSessionRecoverySuccess(c *gin.Context, channelID int) bool {
	if c == nil || channelID <= 0 {
		return false
	}
	key := currentChannelAffinityKey(c)
	if key == "" {
		return false
	}
	chain, found, err := getChannelAffinityRecoveryChainCache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("session recovery success observation failed: %v", err))
		return false
	}
	if !found || chain.RouteChanges <= 0 {
		return false
	}
	maxChanges := 0
	if setting := operation_setting.GetStreamRecoverySetting(); setting != nil {
		maxChanges = setting.MaxCrossRequestRouteChanges
	}
	c.Set(ginKeySessionRecoveryLogInfo, map[string]interface{}{
		"action":           "recovered_next_request",
		"result":           "success",
		"previous_channel": chain.LastChannelID,
		"channel_id":       channelID,
		"budget_used":      chain.RouteChanges,
		"budget_max":       maxChanges,
	})
	return true
}

func AppendSessionRecoveryAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	if value, ok := c.Get(ginKeySessionRecoveryLogInfo); ok {
		adminInfo["session_recovery"] = value
	}
}

func SessionRecoveryLogInfo(c *gin.Context) (map[string]interface{}, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(ginKeySessionRecoveryLogInfo)
	info, valid := value.(map[string]interface{})
	return info, ok && valid && info != nil
}
