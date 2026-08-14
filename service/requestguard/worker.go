package requestguard

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const maxObserveQueueCapacity = 65_536

type observeJob struct {
	Snapshot   Snapshot
	Setting    *operation_setting.RequestGuardSetting
	Meta       RequestMeta
	RecordOnly *Result
}

type processContextHolder struct {
	Context context.Context
}

var (
	observeQueueOnce sync.Once
	observeQueue     chan observeJob
	queuedJobs       atomic.Int64
	processCtx       atomic.Pointer[processContextHolder]
	workerState      = struct {
		sync.Mutex
		active int
	}{}
)

func init() {
	processCtx.Store(&processContextHolder{Context: context.Background()})
}

func SetProcessContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	processCtx.Store(&processContextHolder{Context: ctx})
}

func enqueueObserve(job observeJob) bool {
	capacity := job.Setting.Observe.QueueCapacity
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
	queue := getObserveQueue()
	select {
	case queue <- job:
		setQueueDepth(queuedJobs.Load())
		ensureObserveWorkers(queue, job.Setting.Observe.WorkerCount)
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

func ensureObserveWorkers(queue <-chan observeJob, target int) {
	workerState.Lock()
	defer workerState.Unlock()
	for workerState.active < target {
		workerState.active++
		setWorkerCount(int64(workerState.active))
		go runObserveWorker(queue)
	}
}

func runObserveWorker(queue <-chan observeJob) {
	defer func() {
		workerState.Lock()
		workerState.active--
		setWorkerCount(int64(workerState.active))
		workerState.Unlock()
	}()

	holder := processCtx.Load()
	ctx := context.Background()
	if holder != nil && holder.Context != nil {
		ctx = holder.Context
	}
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-queue:
			queuedJobs.Add(-1)
			setQueueDepth(queuedJobs.Load())
			processObserveJob(ctx, job)
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
