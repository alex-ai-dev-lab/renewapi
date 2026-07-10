package model

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

type consumeLogExport struct {
	UserId    int
	Username  string
	ModelName string
	Quota     int
	Timestamp int64
	Tokens    int
}

type consumeLogBatchItem struct {
	Log    *Log
	Export consumeLogExport
}

type consumeLogBatcher struct {
	queue     chan consumeLogBatchItem
	batchSize int
	interval  time.Duration
}

var (
	consumeLogBatchOnce sync.Once
	consumeLogBatch     *consumeLogBatcher
)

func enqueueConsumeLog(c *gin.Context, log *Log, export consumeLogExport) bool {
	if !common.GetEnvOrDefaultBool("SPEND_LOG_BATCH_ENABLED", false) {
		return false
	}
	b := getConsumeLogBatcher()
	if b == nil {
		return false
	}
	select {
	case b.queue <- consumeLogBatchItem{Log: log, Export: export}:
		return true
	default:
		logger.LogError(c, "consume log batch queue full, fallback to sync write")
		return false
	}
}

func recordConsumeLogSync(c *gin.Context, log *Log, export consumeLogExport) {
	if err := LOG_DB.Create(log).Error; err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	exportConsumeLog(export)
}

func getConsumeLogBatcher() *consumeLogBatcher {
	consumeLogBatchOnce.Do(func() {
		size := common.GetEnvOrDefault("SPEND_LOG_BATCH_SIZE", 100)
		if size <= 0 {
			size = 100
		}
		queueSize := common.GetEnvOrDefault("SPEND_LOG_BATCH_QUEUE_SIZE", size*10)
		if queueSize < size {
			queueSize = size
		}
		intervalMs := common.GetEnvOrDefault("SPEND_LOG_BATCH_INTERVAL_MS", 1000)
		if intervalMs <= 0 {
			intervalMs = 1000
		}
		consumeLogBatch = &consumeLogBatcher{
			queue:     make(chan consumeLogBatchItem, queueSize),
			batchSize: size,
			interval:  time.Duration(intervalMs) * time.Millisecond,
		}
		go consumeLogBatch.run(context.Background())
	})
	return consumeLogBatch
}

func (b *consumeLogBatcher) run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	batch := make([]consumeLogBatchItem, 0, b.batchSize)
	for {
		select {
		case <-ctx.Done():
			b.flush(batch)
			return
		case item := <-b.queue:
			batch = append(batch, item)
			if len(batch) >= b.batchSize {
				b.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				b.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (b *consumeLogBatcher) flush(items []consumeLogBatchItem) {
	if len(items) == 0 {
		return
	}
	logs := make([]*Log, 0, len(items))
	for _, item := range items {
		logs = append(logs, item.Log)
	}
	if err := LOG_DB.CreateInBatches(logs, len(logs)).Error; err != nil {
		common.SysError("failed to batch record consume logs: " + err.Error())
		for _, item := range items {
			if item.Log == nil {
				continue
			}
			if syncErr := LOG_DB.Create(item.Log).Error; syncErr != nil {
				common.SysError("failed to fallback record consume log: " + syncErr.Error())
				continue
			}
			exportConsumeLog(item.Export)
		}
		return
	}
	for _, item := range items {
		exportConsumeLog(item.Export)
	}
}

func exportConsumeLog(export consumeLogExport) {
	if !common.IsDataExportEnabled() {
		return
	}
	gopool.Go(func() {
		LogQuotaData(export.UserId, export.Username, export.ModelName, export.Quota, export.Timestamp, export.Tokens)
	})
}

func FlushSpendLogBatchForTest(timeout time.Duration) {
	if consumeLogBatch == nil {
		return
	}
	deadline := time.Now().Add(timeout)
	for {
		if len(consumeLogBatch.queue) == 0 || time.Now().After(deadline) {
			time.Sleep(consumeLogBatch.interval + 10*time.Millisecond)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ResetSpendLogBatchForTest() {
	consumeLogBatchOnce = sync.Once{}
	consumeLogBatch = nil
}
