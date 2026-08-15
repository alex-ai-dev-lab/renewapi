package requestguard

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	maxObserveQueueCapacity  = 65_536
	maxObserveWorkerCount    = 32
	observeWorkerSyncPeriod  = 100 * time.Millisecond
	observeShutdownDrainTime = 2 * time.Second
)

type observeJob struct {
	Snapshot   Snapshot
	Setting    *operation_setting.RequestGuardSetting
	Meta       RequestMeta
	RecordOnly *Result
}

type observeWorkerHandle struct {
	stop     chan struct{}
	done     chan struct{}
	cancel   context.CancelFunc
	stopOnce sync.Once
}

func (h *observeWorkerHandle) stopGracefully() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() { close(h.stop) })
}

type observeWorkerManager struct {
	queue   <-chan observeJob
	workers []*observeWorkerHandle
	all     []*observeWorkerHandle
}

var (
	observeQueueOnce      sync.Once
	observeQueue          chan observeJob
	queuedJobs            atomic.Int64
	inFlightObserveJobs   atomic.Int64
	activeObserveWorkers  atomic.Int64
	observeAccepting      atomic.Bool
	observeManagerRunning atomic.Bool
)

func enqueueObserve(job observeJob) bool {
	if job.Setting == nil {
		recordObserveDrop()
		return false
	}
	capacity := job.Setting.Observe.QueueCapacity
	if capacity < 1 {
		capacity = 1
	} else if capacity > maxObserveQueueCapacity {
		capacity = maxObserveQueueCapacity
	}
	if !observeAccepting.Load() {
		recordObserveDrop()
		return false
	}
	for {
		current := queuedJobs.Load()
		if current >= int64(capacity) {
			recordObserveDrop()
			return false
		}
		if queuedJobs.CompareAndSwap(current, current+1) {
			break
		}
	}
	select {
	case getObserveQueue() <- job:
		setQueueDepth(queuedJobs.Load())
		return true
	default:
		queuedJobs.Add(-1)
		setQueueDepth(queuedJobs.Load())
		recordObserveDrop()
		return false
	}
}

func getObserveQueue() chan observeJob {
	observeQueueOnce.Do(func() {
		observeQueue = make(chan observeJob, maxObserveQueueCapacity)
	})
	return observeQueue
}

// RunObserveWorkers owns the observe pool for the process lifetime. It starts
// workers before the first enqueue, follows worker-count configuration changes,
// and performs a bounded drain before returning on shutdown.
func RunObserveWorkers(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !observeManagerRunning.CompareAndSwap(false, true) {
		<-ctx.Done()
		return
	}
	defer observeManagerRunning.Store(false)

	manager := &observeWorkerManager{queue: getObserveQueue()}
	observeAccepting.Store(true)
	defer observeAccepting.Store(false)
	manager.resize(configuredObserveWorkerCount())

	ticker := time.NewTicker(observeWorkerSyncPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			observeAccepting.Store(false)
			manager.shutdown(observeShutdownDrainTime)
			return
		case <-ticker.C:
			manager.resize(configuredObserveWorkerCount())
		}
	}
}

func configuredObserveWorkerCount() int {
	target := 1
	if setting := operation_setting.GetRequestGuardSnapshot(); setting != nil {
		target = setting.Observe.WorkerCount
	}
	if target < 1 {
		return 1
	}
	if target > maxObserveWorkerCount {
		return maxObserveWorkerCount
	}
	return target
}

func (m *observeWorkerManager) resize(target int) {
	if m == nil {
		return
	}
	m.pruneFinished()
	for len(m.workers) < target {
		handle := startObserveWorker(m.queue)
		m.workers = append(m.workers, handle)
		m.all = append(m.all, handle)
	}
	for len(m.workers) > target {
		last := len(m.workers) - 1
		handle := m.workers[last]
		m.workers = m.workers[:last]
		handle.stopGracefully()
	}
}

func (m *observeWorkerManager) pruneFinished() {
	if m == nil || len(m.all) == 0 {
		return
	}
	kept := m.all[:0]
	for _, handle := range m.all {
		select {
		case <-handle.done:
		default:
			kept = append(kept, handle)
		}
	}
	m.all = kept
}

func startObserveWorker(queue <-chan observeJob) *observeWorkerHandle {
	workerCtx, cancel := context.WithCancel(context.Background())
	handle := &observeWorkerHandle{
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		active := activeObserveWorkers.Add(1)
		setWorkerCount(active)
		defer func() {
			cancel()
			active = activeObserveWorkers.Add(-1)
			setWorkerCount(active)
			close(handle.done)
		}()
		runObserveWorker(workerCtx, handle.stop, queue)
	}()
	return handle
}

func runObserveWorker(ctx context.Context, stop <-chan struct{}, queue <-chan observeJob) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case job := <-queue:
			queuedJobs.Add(-1)
			setQueueDepth(queuedJobs.Load())
			inFlightObserveJobs.Add(1)
			processObserveJob(ctx, job)
			inFlightObserveJobs.Add(-1)
		}
	}
}

func (m *observeWorkerManager) shutdown(timeout time.Duration) {
	if m == nil {
		return
	}
	if timeout <= 0 {
		timeout = observeShutdownDrainTime
	}
	deadline := time.Now().Add(timeout)
	for (queuedJobs.Load() > 0 || inFlightObserveJobs.Load() > 0) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	for _, handle := range m.workers {
		handle.stopGracefully()
	}
	for _, handle := range m.all {
		if time.Now().After(deadline) {
			handle.cancel()
			continue
		}
		select {
		case <-handle.done:
		case <-time.After(time.Until(deadline)):
			handle.cancel()
		}
	}
}

func processObserveJob(ctx context.Context, job observeJob) {
	if job.RecordOnly != nil {
		recordEvent(job.Meta, job.Snapshot, *job.RecordOnly, job.Setting)
		return
	}
	result := globalEvaluator.Evaluate(ctx, job.Snapshot, job.Setting, job.Meta)
	recordDecision(job.Meta.Mode, result.Kind)
	recordEvent(job.Meta, job.Snapshot, result, job.Setting)
}
