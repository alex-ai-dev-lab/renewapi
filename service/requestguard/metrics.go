package requestguard

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestGuardDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "newapi_request_guard_requests_total", Help: "RequestGuard requests by mode and decision."},
		[]string{"mode", "decision"},
	)
	requestGuardEndpointAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "newapi_request_guard_endpoint_requests_total", Help: "RequestGuard endpoint request outcomes."},
		[]string{"endpoint_id", "outcome"},
	)
	requestGuardDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "newapi_request_guard_duration_seconds", Help: "End-to-end RequestGuard evaluation latency.", Buckets: prometheus.DefBuckets},
	)
	requestGuardEndpointDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "newapi_request_guard_endpoint_duration_seconds", Help: "RequestGuard endpoint evaluation latency.", Buckets: prometheus.DefBuckets},
		[]string{"endpoint_id"},
	)
	requestGuardFailover = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "newapi_request_guard_failover_total", Help: "RequestGuard endpoint failovers."},
	)
	requestGuardBulkheadRejected = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "newapi_request_guard_bulkhead_rejected_total", Help: "RequestGuard evaluations rejected by a bulkhead."},
	)
	requestGuardInputTruncated = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "newapi_request_guard_input_truncated_total", Help: "RequestGuard inputs truncated at the configured rune limit."},
	)
	requestGuardObserveDropped = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "newapi_request_guard_observe_queue_dropped_total", Help: "Observe jobs dropped because the bounded queue was full."},
	)
	requestGuardFailOpen = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "newapi_request_guard_fail_open_total", Help: "Requests allowed by the explicit fail-open policy."},
		[]string{"reason"},
	)
	requestGuardAuditErrors = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "newapi_request_guard_audit_write_errors_total", Help: "RequestGuard audit event persistence failures."},
	)
	requestGuardQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "newapi_request_guard_observe_queue_depth", Help: "Current RequestGuard observe queue depth."},
	)
	requestGuardWorkers = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "newapi_request_guard_observe_workers", Help: "Current RequestGuard observe worker count."},
	)
	requestGuardBulkhead = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "newapi_request_guard_bulkhead_active", Help: "Current RequestGuard evaluations holding a bulkhead slot."},
	)
)

type metricState struct {
	Decisions      atomic.Int64
	ObserveDropped atomic.Int64
	FailOpen       atomic.Int64
	AuditErrors    atomic.Int64
	QueueDepth     atomic.Int64
	Workers        atomic.Int64
	BulkheadActive atomic.Int64
	Failovers      atomic.Int64
	BulkheadReject atomic.Int64
	InputTruncated atomic.Int64
}

var metricsState metricState

type MetricsSnapshot struct {
	Decisions      int64 `json:"decisions"`
	ObserveDropped int64 `json:"observe_dropped"`
	FailOpen       int64 `json:"fail_open"`
	AuditErrors    int64 `json:"audit_errors"`
	QueueDepth     int64 `json:"queue_depth"`
	Workers        int64 `json:"workers"`
	BulkheadActive int64 `json:"bulkhead_active"`
	Failovers      int64 `json:"failovers"`
	BulkheadReject int64 `json:"bulkhead_rejected"`
	InputTruncated int64 `json:"input_truncated"`
}

type EndpointStatus struct {
	EndpointID    string `json:"endpoint_id"`
	Healthy       bool   `json:"healthy"`
	LastOutcome   string `json:"last_outcome"`
	LastError     string `json:"last_error,omitempty"`
	LastLatencyMs int64  `json:"last_latency_ms"`
	LastCheckedAt int64  `json:"last_checked_at"`
}

var endpointStatuses = struct {
	sync.RWMutex
	values map[string]EndpointStatus
}{values: make(map[string]EndpointStatus)}

func Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		requestGuardDecisions,
		requestGuardEndpointAttempts,
		requestGuardDuration,
		requestGuardEndpointDuration,
		requestGuardFailover,
		requestGuardBulkheadRejected,
		requestGuardInputTruncated,
		requestGuardObserveDropped,
		requestGuardFailOpen,
		requestGuardAuditErrors,
		requestGuardQueueDepth,
		requestGuardWorkers,
		requestGuardBulkhead,
	}
}

func CurrentMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		Decisions: metricsState.Decisions.Load(), ObserveDropped: metricsState.ObserveDropped.Load(),
		FailOpen: metricsState.FailOpen.Load(), AuditErrors: metricsState.AuditErrors.Load(),
		QueueDepth: metricsState.QueueDepth.Load(), Workers: metricsState.Workers.Load(),
		BulkheadActive: metricsState.BulkheadActive.Load(),
		Failovers:      metricsState.Failovers.Load(), BulkheadReject: metricsState.BulkheadReject.Load(),
		InputTruncated: metricsState.InputTruncated.Load(),
	}
}

func CurrentEndpointStatuses() []EndpointStatus {
	endpointStatuses.RLock()
	defer endpointStatuses.RUnlock()
	result := make([]EndpointStatus, 0, len(endpointStatuses.values))
	for _, status := range endpointStatuses.values {
		result = append(result, status)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].EndpointID < result[j].EndpointID })
	return result
}

func recordDecision(mode string, decision DecisionKind) {
	metricsState.Decisions.Add(1)
	requestGuardDecisions.WithLabelValues(mode, string(decision)).Inc()
}

func recordObserveDrop() {
	metricsState.ObserveDropped.Add(1)
	requestGuardObserveDropped.Inc()
}

func recordFailOpen(reason string) {
	metricsState.FailOpen.Add(1)
	requestGuardFailOpen.WithLabelValues(reason).Inc()
}

func recordAuditError() {
	metricsState.AuditErrors.Add(1)
	requestGuardAuditErrors.Inc()
}

func setQueueDepth(value int64) {
	metricsState.QueueDepth.Store(value)
	requestGuardQueueDepth.Set(float64(value))
}

func setWorkerCount(value int64) {
	metricsState.Workers.Store(value)
	requestGuardWorkers.Set(float64(value))
}

func setBulkheadActive(value int) {
	metricsState.BulkheadActive.Store(int64(value))
	requestGuardBulkhead.Set(float64(value))
}

func recordRequestDuration(duration time.Duration) {
	requestGuardDuration.Observe(duration.Seconds())
}

func recordFailover() {
	metricsState.Failovers.Add(1)
	requestGuardFailover.Inc()
}

func recordBulkheadRejected() {
	metricsState.BulkheadReject.Add(1)
	requestGuardBulkheadRejected.Inc()
}

func recordInputTruncated() {
	metricsState.InputTruncated.Add(1)
	requestGuardInputTruncated.Inc()
}

func recordEndpointAttempt(endpointID, outcome string, latency time.Duration, errorText string) {
	requestGuardEndpointAttempts.WithLabelValues(endpointID, outcome).Inc()
	requestGuardEndpointDuration.WithLabelValues(endpointID).Observe(latency.Seconds())
	endpointStatuses.Lock()
	endpointStatuses.values[endpointID] = EndpointStatus{
		EndpointID: endpointID, Healthy: outcome == "allow" || outcome == "flag" || outcome == "block",
		LastOutcome: outcome, LastError: errorText, LastLatencyMs: latency.Milliseconds(), LastCheckedAt: time.Now().Unix(),
	}
	endpointStatuses.Unlock()
}
