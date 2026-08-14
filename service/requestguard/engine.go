package requestguard

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var globalEvaluator evaluator = newDefaultEvaluator()

func Preflight(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) *types.NewAPIError {
	setting := operation_setting.GetRequestGuardSnapshot()
	if setting == nil || !setting.Enabled || setting.Mode == operation_setting.RequestGuardModeOff {
		return nil
	}
	if c != nil && c.GetHeader(internalRequestHeader) == "1" {
		return types.NewErrorWithStatusCode(
			errors.New("recursive RequestGuard invocation blocked"),
			types.ErrorCodeRequestGuardUnavailable,
			http.StatusLoopDetected,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	protocol, matched := matchScope(setting, info, request)
	if !matched {
		return nil
	}
	snapshot, err := Extract(request, info, setting.MaxInputRunes, setting.InputMode)
	if err != nil {
		result := Result{Kind: DecisionInvalid, ReasonCode: "snapshot_invalid"}
		return handleEnforceResult(buildRequestMeta(c, info, protocol, setting.Mode), snapshot, result, setting)
	}
	if snapshot.Truncated {
		recordInputTruncated()
	}
	meta := buildRequestMeta(c, info, protocol, setting.Mode)
	if snapshot.RuneCount == 0 {
		result := Result{Kind: DecisionAllow, ReasonCode: "no_text"}
		recordDecision(setting.Mode, result.Kind)
		if setting.StorePassEvents {
			enqueueObserve(observeJob{Snapshot: snapshot, Setting: setting, Meta: meta, RecordOnly: &result})
		}
		return nil
	}

	if setting.Mode == operation_setting.RequestGuardModeObserve {
		enqueueObserve(observeJob{Snapshot: snapshot, Setting: setting, Meta: meta})
		return nil
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	result := globalEvaluator.Evaluate(ctx, snapshot, setting, meta)
	return handleEnforceResult(meta, snapshot, result, setting)
}

func Probe(ctx context.Context, endpointID, requestHost string) (Result, error) {
	setting := operation_setting.GetRequestGuardSnapshot()
	if setting == nil {
		return Result{}, errors.New("RequestGuard configuration unavailable")
	}
	if err := validateEndpointID(setting, endpointID); err != nil {
		return Result{}, err
	}
	builder := newSnapshotBuilder(512)
	builder.append("user", "RequestGuard connectivity probe. Classify this benign text according to policy.")
	result := globalEvaluator.EvaluateEndpoint(ctx, builder.snapshot(), setting, endpointID, requestHost)
	return result, nil
}

func handleEnforceResult(meta RequestMeta, snapshot Snapshot, result Result, setting *operation_setting.RequestGuardSetting) *types.NewAPIError {
	recordDecision(meta.Mode, result.Kind)
	if result.Kind == DecisionAllow && setting.StorePassEvents {
		enqueueObserve(observeJob{Snapshot: snapshot, Setting: setting, Meta: meta, RecordOnly: &result})
	} else if result.Kind != DecisionAllow {
		recordEvent(meta, snapshot, result, setting)
	}

	switch result.Kind {
	case DecisionAllow, DecisionFlag:
		return nil
	case DecisionBlock:
		return types.NewErrorWithStatusCode(
			errors.New("request blocked by RequestGuard policy"),
			types.ErrorCodeRequestGuardBlocked,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	case DecisionUnavailable, DecisionInvalid:
		if setting.FailurePolicy == operation_setting.RequestGuardFailureOpen {
			recordFailOpen(result.ReasonCode)
			return nil
		}
		code := types.ErrorCodeRequestGuardUnavailable
		message := "RequestGuard is temporarily unavailable"
		if result.Kind == DecisionInvalid {
			code = types.ErrorCodeRequestGuardInvalidResponse
			message = "RequestGuard returned an invalid response"
		}
		return types.NewErrorWithStatusCode(
			errors.New(message), code, http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
		)
	default:
		return types.NewErrorWithStatusCode(
			errors.New("RequestGuard returned an unknown decision"),
			types.ErrorCodeRequestGuardInvalidResponse,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
		)
	}
}

func buildRequestMeta(c *gin.Context, info *relaycommon.RelayInfo, protocol, mode string) RequestMeta {
	meta := RequestMeta{Protocol: protocol, Mode: mode}
	if info != nil {
		meta.RequestID = info.RequestId
		meta.UserID = info.UserId
		meta.TokenID = info.TokenId
		meta.Group = info.UsingGroup
		if strings.TrimSpace(meta.Group) == "" {
			meta.Group = info.TokenGroup
		}
		meta.Model = info.OriginModelName
	}
	if c != nil && c.Request != nil {
		meta.RequestHost = c.Request.Host
	}
	return meta
}
