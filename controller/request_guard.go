package controller

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/requestguard"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type requestGuardEndpointView struct {
	operation_setting.RequestGuardEndpoint
	HasSecret    bool   `json:"has_secret"`
	SecretStatus string `json:"secret_status"`
}

type requestGuardConfigView struct {
	Enabled              bool                                   `json:"enabled"`
	Mode                 string                                 `json:"mode"`
	FailurePolicy        string                                 `json:"failure_policy"`
	InputMode            string                                 `json:"input_mode"`
	MaxInputRunes        int                                    `json:"max_input_runes"`
	EvaluationTimeoutMs  int                                    `json:"evaluation_timeout_ms"`
	Scope                operation_setting.RequestGuardScope    `json:"scope"`
	Bulkhead             operation_setting.RequestGuardBulkhead `json:"bulkhead"`
	Observe              operation_setting.RequestGuardObserve  `json:"observe"`
	StorePassEvents      bool                                   `json:"store_pass_events"`
	StoreRedactedPreview bool                                   `json:"store_redacted_preview"`
	Endpoints            []requestGuardEndpointView             `json:"endpoints"`
}

type requestGuardEndpointUpdate struct {
	operation_setting.RequestGuardEndpoint
	Secret      *string `json:"secret"`
	ClearSecret bool    `json:"clear_secret"`
}

type requestGuardConfigUpdate struct {
	Enabled              bool                                   `json:"enabled"`
	Mode                 string                                 `json:"mode"`
	FailurePolicy        string                                 `json:"failure_policy"`
	InputMode            string                                 `json:"input_mode"`
	MaxInputRunes        int                                    `json:"max_input_runes"`
	EvaluationTimeoutMs  int                                    `json:"evaluation_timeout_ms"`
	Scope                operation_setting.RequestGuardScope    `json:"scope"`
	Bulkhead             operation_setting.RequestGuardBulkhead `json:"bulkhead"`
	Observe              operation_setting.RequestGuardObserve  `json:"observe"`
	StorePassEvents      bool                                   `json:"store_pass_events"`
	StoreRedactedPreview bool                                   `json:"store_redacted_preview"`
	Endpoints            []requestGuardEndpointUpdate           `json:"endpoints"`
}

func GetRequestGuardConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": buildRequestGuardConfigView(operation_setting.GetRequestGuardSetting())})
}

func UpdateRequestGuardConfig(c *gin.Context) {
	var payload requestGuardConfigUpdate
	if err := common.DecodeJson(c.Request.Body, &payload); err != nil {
		writeRequestGuardError(c, http.StatusBadRequest, "invalid RequestGuard configuration")
		return
	}

	next := operation_setting.RequestGuardSetting{
		Enabled: payload.Enabled, Mode: payload.Mode, FailurePolicy: payload.FailurePolicy,
		InputMode: payload.InputMode, MaxInputRunes: payload.MaxInputRunes,
		EvaluationTimeoutMs: payload.EvaluationTimeoutMs, Scope: payload.Scope,
		Bulkhead: payload.Bulkhead, Observe: payload.Observe,
		StorePassEvents: payload.StorePassEvents, StoreRedactedPreview: payload.StoreRedactedPreview,
		Endpoints: make([]operation_setting.RequestGuardEndpoint, 0, len(payload.Endpoints)),
	}
	secretUpdates := make(map[string]string)
	for _, endpoint := range payload.Endpoints {
		next.Endpoints = append(next.Endpoints, endpoint.RequestGuardEndpoint)
		secretKey := requestguard.EndpointSecretOptionKey(endpoint.ID)
		switch {
		case endpoint.ClearSecret:
			secretUpdates[secretKey] = ""
		case endpoint.Secret != nil && strings.TrimSpace(*endpoint.Secret) != "":
			secretUpdates[secretKey] = strings.TrimSpace(*endpoint.Secret)
		}
	}
	if err := operation_setting.ValidateRequestGuardSetting(next); err != nil {
		writeRequestGuardError(c, http.StatusBadRequest, err.Error())
		return
	}

	current := operation_setting.GetRequestGuardSetting()
	nextIDs := make(map[string]struct{}, len(next.Endpoints))
	for _, endpoint := range next.Endpoints {
		nextIDs[endpoint.ID] = struct{}{}
	}
	for _, endpoint := range current.Endpoints {
		if _, ok := nextIDs[endpoint.ID]; !ok {
			secretUpdates[requestguard.EndpointSecretOptionKey(endpoint.ID)] = ""
		}
	}
	encoded, err := common.Marshal(next)
	if err != nil {
		writeRequestGuardError(c, http.StatusInternalServerError, "failed to encode RequestGuard configuration")
		return
	}
	updates := make(map[string]string, len(secretUpdates)+1)
	updates["request_guard_setting"] = string(encoded)
	for key, value := range secretUpdates {
		updates[key] = value
	}
	if err := model.UpdateOptionsBulk(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": buildRequestGuardConfigView(operation_setting.GetRequestGuardSetting())})
}

func ProbeRequestGuardEndpoint(c *gin.Context) {
	var payload struct {
		EndpointID string `json:"endpoint_id"`
	}
	if err := common.DecodeJson(c.Request.Body, &payload); err != nil || strings.TrimSpace(payload.EndpointID) == "" {
		writeRequestGuardError(c, http.StatusBadRequest, "endpoint_id is required")
		return
	}
	started := time.Now()
	result, err := requestguard.Probe(c.Request.Context(), strings.TrimSpace(payload.EndpointID), c.Request.Host)
	latencyMs := time.Since(started).Milliseconds()
	if result.Latency > 0 {
		latencyMs = result.Latency.Milliseconds()
	}
	if err != nil {
		writeRequestGuardError(c, http.StatusBadRequest, err.Error())
		return
	}
	reachable := result.Kind == requestguard.DecisionAllow || result.Kind == requestguard.DecisionFlag || result.Kind == requestguard.DecisionBlock
	codecValid := result.Kind != requestguard.DecisionInvalid
	c.JSON(http.StatusOK, gin.H{
		"success": true, "message": "", "data": gin.H{
			"reachable": reachable, "http_status": result.HTTPStatus, "codec_valid": codecValid,
			"latency_ms": latencyMs, "model": result.Model, "error_class": result.ErrorClass,
			"decision": result.Kind, "reason_code": result.ReasonCode,
		},
	})
}

func GetRequestGuardStatus(c *gin.Context) {
	setting := operation_setting.GetRequestGuardSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true, "message": "", "data": gin.H{
			"enabled": setting.Enabled, "mode": setting.Mode, "failure_policy": setting.FailurePolicy,
			"metrics": requestguard.CurrentMetrics(), "endpoints": requestguard.CurrentEndpointStatuses(),
		},
	})
}

func GetRequestGuardEvents(c *gin.Context) {
	beforeID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("before_id")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	events, err := model.ListRequestGuardEvents(beforeID, limit, strings.TrimSpace(c.Query("decision")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": events})
}

func buildRequestGuardConfigView(setting operation_setting.RequestGuardSetting) requestGuardConfigView {
	endpoints := make([]requestGuardEndpointView, 0, len(setting.Endpoints))
	for _, endpoint := range setting.Endpoints {
		hasSecret := requestguard.HasEndpointSecret(endpoint.ID)
		status := "not_configured"
		if hasSecret {
			status = "configured"
		}
		endpoints = append(endpoints, requestGuardEndpointView{RequestGuardEndpoint: endpoint, HasSecret: hasSecret, SecretStatus: status})
	}
	sort.SliceStable(endpoints, func(i, j int) bool {
		if endpoints[i].Priority == endpoints[j].Priority {
			return endpoints[i].ID < endpoints[j].ID
		}
		return endpoints[i].Priority > endpoints[j].Priority
	})
	return requestGuardConfigView{
		Enabled: setting.Enabled, Mode: setting.Mode, FailurePolicy: setting.FailurePolicy,
		InputMode: setting.InputMode, MaxInputRunes: setting.MaxInputRunes,
		EvaluationTimeoutMs: setting.EvaluationTimeoutMs, Scope: setting.Scope,
		Bulkhead: setting.Bulkhead, Observe: setting.Observe,
		StorePassEvents: setting.StorePassEvents, StoreRedactedPreview: setting.StoreRedactedPreview,
		Endpoints: endpoints,
	}
}

func writeRequestGuardError(c *gin.Context, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = errors.New("RequestGuard request failed").Error()
	}
	c.JSON(status, gin.H{"success": false, "message": message})
}
