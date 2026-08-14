package requestguard

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type evaluator interface {
	Evaluate(ctx context.Context, snapshot Snapshot, setting *operation_setting.RequestGuardSetting, meta RequestMeta) Result
	EvaluateEndpoint(ctx context.Context, snapshot Snapshot, setting *operation_setting.RequestGuardSetting, endpointID, requestHost string) Result
}

type defaultEvaluator struct {
	scanner scanner
	limits  *bulkhead
}

type bulkhead struct {
	mu          sync.Mutex
	global      int
	perEndpoint map[string]int
}

func newDefaultEvaluator() *defaultEvaluator {
	return &defaultEvaluator{
		scanner: &openAICompatibleScanner{},
		limits:  &bulkhead{perEndpoint: make(map[string]int)},
	}
}

func (e *defaultEvaluator) Evaluate(ctx context.Context, snapshot Snapshot, setting *operation_setting.RequestGuardSetting, meta RequestMeta) Result {
	return e.evaluate(ctx, snapshot, setting, "", meta.RequestHost)
}

func (e *defaultEvaluator) EvaluateEndpoint(ctx context.Context, snapshot Snapshot, setting *operation_setting.RequestGuardSetting, endpointID, requestHost string) Result {
	return e.evaluate(ctx, snapshot, setting, endpointID, requestHost)
}

func (e *defaultEvaluator) evaluate(ctx context.Context, snapshot Snapshot, setting *operation_setting.RequestGuardSetting, onlyEndpointID, requestHost string) Result {
	startedAll := time.Now()
	defer func() {
		recordRequestDuration(time.Since(startedAll))
	}()
	if setting == nil {
		return Result{Kind: DecisionUnavailable, ReasonCode: "config_unavailable"}
	}
	deadlineContext, cancel := context.WithTimeout(ctx, time.Duration(setting.EvaluationTimeoutMs)*time.Millisecond)
	defer cancel()

	sawInvalid := false
	attempted := false
	attemptCount := 0
	lastErrorClass := ""
	lastHTTPStatus := 0
	for _, endpoint := range setting.Endpoints {
		if !endpoint.Enabled || (onlyEndpointID != "" && endpoint.ID != onlyEndpointID) {
			continue
		}
		attempted = true
		if attemptCount > 0 {
			recordFailover()
		}
		attemptCount++
		if !e.limits.tryAcquire(setting, endpoint.ID) {
			recordEndpointAttempt(endpoint.ID, "bulkhead_full", 0, "bulkhead saturated")
			recordBulkheadRejected()
			lastErrorClass = "bulkhead_full"
			continue
		}
		started := time.Now()
		remaining := time.Until(deadlineFromContext(deadlineContext))
		if remaining <= 0 {
			e.limits.release(endpoint.ID)
			break
		}
		endpointTimeout := time.Duration(endpoint.TimeoutMs) * time.Millisecond
		endpointContext := deadlineContext
		endpointCancel := func() {}
		if endpointTimeout < remaining {
			endpointContext, endpointCancel = context.WithTimeout(deadlineContext, endpointTimeout)
		}
		endpointSnapshot := limitSnapshot(snapshot, endpoint.InputLimitRunes)
		result, err := e.scanner.Evaluate(endpointContext, endpointSnapshot, endpoint, EndpointSecret(endpoint.ID), requestHost)
		endpointCancel()
		e.limits.release(endpoint.ID)
		latency := time.Since(started)
		if err == nil {
			result.EndpointID = endpoint.ID
			result.Model = endpoint.Model
			result.Latency = latency
			recordEndpointAttempt(endpoint.ID, string(result.Kind), latency, "")
			return result
		}
		lastErrorClass, lastHTTPStatus = scanErrorDetails(err)
		if errors.Is(err, ErrInvalidResponse) {
			sawInvalid = true
			recordEndpointAttempt(endpoint.ID, "invalid", latency, lastErrorClass)
		} else {
			recordEndpointAttempt(endpoint.ID, "unavailable", latency, lastErrorClass)
		}
		if deadlineContext.Err() != nil {
			break
		}
	}
	if onlyEndpointID != "" && !attempted {
		return Result{Kind: DecisionUnavailable, ReasonCode: "endpoint_not_found", ErrorClass: "endpoint_not_found"}
	}
	if sawInvalid {
		return Result{Kind: DecisionInvalid, ReasonCode: "invalid_guard_response", HTTPStatus: lastHTTPStatus, ErrorClass: lastErrorClass}
	}
	reason := "guard_unavailable"
	if deadlineContext.Err() != nil {
		reason = "evaluation_timeout"
	}
	return Result{Kind: DecisionUnavailable, ReasonCode: reason, HTTPStatus: lastHTTPStatus, ErrorClass: lastErrorClass}
}

func (b *bulkhead) tryAcquire(setting *operation_setting.RequestGuardSetting, endpointID string) bool {
	if b == nil || setting == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.global >= setting.Bulkhead.MaxConcurrent || b.perEndpoint[endpointID] >= setting.Bulkhead.MaxPerEndpoint {
		return false
	}
	b.global++
	b.perEndpoint[endpointID]++
	setBulkheadActive(b.global)
	return true
}

func (b *bulkhead) release(endpointID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.global > 0 {
		b.global--
	}
	if b.perEndpoint[endpointID] <= 1 {
		delete(b.perEndpoint, endpointID)
	} else {
		b.perEndpoint[endpointID]--
	}
	setBulkheadActive(b.global)
}

func deadlineFromContext(ctx context.Context) time.Time {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Now()
	}
	return deadline
}

func EndpointSecret(endpointID string) string {
	return endpointSecretProvider(endpointID)
}

var endpointSecretProvider = defaultEndpointSecretProvider

func defaultEndpointSecretProvider(endpointID string) string {
	return loadEndpointSecret(endpointID)
}

func validateEndpointID(setting *operation_setting.RequestGuardSetting, endpointID string) error {
	for _, endpoint := range setting.Endpoints {
		if endpoint.ID == endpointID && endpoint.Enabled {
			return nil
		}
	}
	return fmt.Errorf("enabled endpoint %q not found", endpointID)
}
