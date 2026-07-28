package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/compat/ssepool"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10
	DefaultMaxScannerBufferSize = 64 << 20
	DefaultPingInterval         = 10 * time.Second
	streamWriteTimeout          = 30 * time.Second
)

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func NewStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	return scanner
}

// ExtendWriteDeadline prevents one slow client write from blocking stream
// cleanup forever. Writers without deadline support are ignored.
func ExtendWriteDeadline(c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(streamWriteTimeout))
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {
	if c == nil || resp == nil || resp.Body == nil || info == nil || dataHandler == nil {
		return
	}

	info.StreamStatus = relaycommon.NewStreamStatusFromExisting(info.StreamStatus)
	c.Set(streamStatusContextKey, info.StreamStatus)
	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	if streamingTimeout <= 0 {
		streamingTimeout = 30 * time.Second
	}
	firstByteTimeout := time.Duration(common.RelayFirstByteTimeout) * time.Second
	if firstByteTimeout <= 0 {
		firstByteTimeout = 15 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	abortCh := make(chan struct{})
	handlerDone := make(chan struct{})
	dataChan := make(chan string, 10)
	streamTimer := time.NewTimer(streamingTimeout)
	firstByteTimer := time.NewTimer(firstByteTimeout)
	var pingTicker *time.Ticker
	var writeMutex sync.Mutex
	var wg sync.WaitGroup
	var cleanupOnce sync.Once
	var abortOnce sync.Once
	var firstResponseObserved atomic.Bool
	pooledBuf := ssepool.Get()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(pooledBuf[:], getScannerBufferSize())
	scanner.Split(bufio.ScanLines)

	abort := func() {
		abortOnce.Do(func() { close(abortCh) })
	}
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			abort()
			_ = resp.Body.Close()
			if !streamTimer.Stop() {
				select {
				case <-streamTimer.C:
				default:
				}
			}
			if !firstByteTimer.Stop() {
				select {
				case <-firstByteTimer.C:
				default:
				}
			}
			if pingTicker != nil {
				pingTicker.Stop()
			}
			wg.Wait()
			ssepool.Put(pooledBuf)
		})
	}
	defer cleanup()

	generalSettings := operation_setting.GetGeneralSetting()
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}
	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "first byte timeout seconds: %d", int64(firstByteTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))
	SetEventStreamHeaders(c)
	ctx = context.WithValue(ctx, "stop_chan", abortCh)

	if pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					err := fmt.Errorf("ping panic: %v", recovered)
					logger.LogError(c, err.Error())
					info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonPanic, err)
					abort()
				}
				logger.LogDebug(c, "ping goroutine exited")
				wg.Done()
			}()

			maxDuration := time.NewTimer(30 * time.Minute)
			defer maxDuration.Stop()
			for {
				select {
				case <-pingTicker.C:
					if !info.StreamStatus.IsClientCommitted() {
						continue
					}
					writeMutex.Lock()
					ExtendWriteDeadline(c)
					err := PingData(c)
					writeMutex.Unlock()
					if err != nil {
						info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonPingFail, err)
						abort()
						return
					}
				case <-ctx.Done():
					return
				case <-abortCh:
					return
				case <-c.Request.Context().Done():
					return
				case <-maxDuration.C:
					err := fmt.Errorf("ping goroutine max duration reached")
					info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonPingFail, err)
					abort()
					return
				}
			}
		})
	}

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("handler panic: %v", recovered)
				logger.LogError(c, err.Error())
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonPanic, err)
				abort()
			}
			close(handlerDone)
			wg.Done()
		}()

		sr := newStreamResult(info.StreamStatus)
		for {
			select {
			case <-abortCh:
				return
			case data, ok := <-dataChan:
				if !ok {
					return
				}
				sr.reset()
				writeMutex.Lock()
				ExtendWriteDeadline(c)
				dataHandler(data, sr)
				writeMutex.Unlock()
				if sr.IsStopped() {
					abort()
					return
				}
			}
		}
	})

	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("scanner panic: %v", recovered)
				logger.LogError(c, err.Error())
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonPanic, err)
				abort()
			}
			close(dataChan)
			logger.LogDebug(c, "scanner goroutine exited")
			wg.Done()
		}()

		for scanner.Scan() {
			select {
			case <-abortCh:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				abort()
				return
			default:
			}

			if !streamTimer.Stop() {
				select {
				case <-streamTimer.C:
				default:
				}
			}
			streamTimer.Reset(streamingTimeout)
			data := scanner.Text()
			logger.LogDebug(c, "stream scanner data: %s", data)
			if strings.HasPrefix(data, "[DONE]") {
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonDone, nil)
				return
			}
			if !strings.HasPrefix(data, "data:") {
				continue
			}
			data = strings.TrimSpace(data[5:])
			if data == "" {
				continue
			}
			if strings.HasPrefix(data, "[DONE]") {
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonDone, nil)
				return
			}
			info.StreamStatus.ObserveRawFrame()
			if firstResponseObserved.CompareAndSwap(false, true) {
				if !firstByteTimer.Stop() {
					select {
					case <-firstByteTimer.C:
					default:
					}
				}
			}
			info.SetFirstResponseTime()
			info.ReceivedResponseCount++
			select {
			case dataChan <- data:
			case <-abortCh:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				abort()
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-abortCh:
				return
			default:
			}
			if c.Request.Context().Err() == nil {
				logger.LogError(c, "scanner error: "+err.Error())
				info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonScannerErr, err)
			}
			return
		}
		info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonEOF, nil)
	})

	select {
	case <-handlerDone:
	case <-abortCh:
	case <-streamTimer.C:
		info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonTimeout, nil)
	case <-firstByteTimer.C:
		if firstResponseObserved.Load() {
			info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonTimeout, nil)
		} else {
			info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonFirstByteTimeout, nil)
		}
	case <-c.Request.Context().Done():
		info.StreamStatus.SetTransportEnd(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	}

	cleanup()
	info.StreamStatus.Finalize()
	if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
	} else {
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
	}
}
