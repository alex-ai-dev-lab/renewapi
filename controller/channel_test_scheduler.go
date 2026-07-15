package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

// channelTestTracker tracks per-channel test state for the independent-schedule
// test loop introduced by the per-channel auto-test feature.
type channelTestTracker struct {
	mu sync.Mutex
	// channelLastTest stores the last time each channel was tested.
	channelLastTest map[int]time.Time
	// channelFailCount stores consecutive failures per channel (key: channelId).
	channelFailCount map[int]int
}

var testTracking = &channelTestTracker{
	channelLastTest:  make(map[int]time.Time),
	channelFailCount: make(map[int]int),
}

var responsesCompactionProbeSemaphore = make(chan struct{}, 4)

func responsesCompactionProbeTimeoutSeconds() int {
	timeoutSeconds := common.GetEnvOrDefault("RESPONSES_COMPACTION_PROBE_TIMEOUT_SECONDS", 90)
	if timeoutSeconds < 5 {
		return 5
	}
	return timeoutSeconds
}

func responsesCompactionProbeModels(channel *model.Channel) []string {
	if channel == nil {
		return nil
	}
	settings := channel.GetSetting().ResponsesCompaction
	manualCapabilityIsConcrete := func(modelName string) bool {
		if settings == nil {
			return false
		}
		modelName = strings.TrimSuffix(strings.TrimSpace(modelName), ratio_setting.CompactModelSuffix)
		if record, ok := settings.ModelCapabilities[modelName]; ok {
			capability := strings.TrimSpace(string(record.Capability))
			return capability != "" && !strings.EqualFold(capability, string(dto.CompactionUnknown)) && record.VerifiedAt <= 0
		}
		if settings.DefaultCapability != nil {
			capability := strings.TrimSpace(string(settings.DefaultCapability.Capability))
			return capability != "" && !strings.EqualFold(capability, string(dto.CompactionUnknown)) && settings.DefaultCapability.VerifiedAt <= 0
		}
		return false
	}
	seen := make(map[string]string)
	add := func(modelName string) {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || strings.ContainsAny(modelName, "*?") {
			return
		}
		modelName = strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix)
		if modelName != "" && !manualCapabilityIsConcrete(modelName) {
			seen[strings.ToLower(modelName)] = modelName
		}
	}

	if settings != nil {
		for modelName := range settings.ModelCapabilities {
			add(modelName)
		}
		if settings.DefaultCapability != nil && channel.TestModel != nil {
			add(*channel.TestModel)
		}
	}
	for _, modelName := range channel.GetModels() {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(modelName)), ratio_setting.CompactModelSuffix) {
			add(modelName)
		}
	}
	if records, err := model.ListChannelModelCapabilities(channel.Id, model.ChannelCapabilityResponsesCompaction); err == nil {
		for _, record := range records {
			add(record.ModelName)
		}
	}
	targetModel := strings.TrimSpace(common.GetEnvOrDefaultString("RESPONSES_COMPACTION_MODEL", ""))
	if targetModel != "" {
		for _, routingModel := range channel.GetRoutingModels() {
			if strings.EqualFold(routingModel, targetModel) {
				add(targetModel)
				break
			}
		}
	}

	result := make([]string, 0, len(seen))
	for _, modelName := range seen {
		result = append(result, modelName)
	}
	sort.Strings(result)
	maxModels := common.GetEnvOrDefault("RESPONSES_COMPACTION_PROBE_MAX_MODELS", 4)
	if maxModels > 0 && len(result) > maxModels {
		result = result[:maxModels]
	}
	return result
}

func buildResponsesCompactionProbeRequest(modelName string, stream bool, compactedItem json.RawMessage) (*dto.OpenAIResponsesRequest, error) {
	message := json.RawMessage(`{"role":"user","content":[{"type":"input_text","text":"Compress this channel capability probe state."}]}`)
	items := []json.RawMessage{message}
	if len(compactedItem) > 0 {
		items = []json.RawMessage{
			append(json.RawMessage(nil), compactedItem...),
			json.RawMessage(`{"role":"user","content":[{"type":"input_text","text":"Reply with CONTINUE_OK."}]}`),
		}
	} else {
		items = append(items, json.RawMessage(`{"type":"compaction_trigger"}`))
	}
	input, err := common.Marshal(items)
	if err != nil {
		return nil, err
	}
	return &dto.OpenAIResponsesRequest{
		Model:  modelName,
		Input:  input,
		Store:  json.RawMessage(`false`),
		Stream: common.GetPointer(stream),
	}, nil
}

func responsesCompactionObservationComplete(record model.ChannelModelCapability) bool {
	if record.LegacyStatus == model.ChannelCapabilityStatusUnknown || record.NativeStatus == model.ChannelCapabilityStatusUnknown {
		return false
	}
	if record.NativeStatus != model.ChannelCapabilityStatusSupported {
		return true
	}
	return record.NativeStreamStatus != model.ChannelCapabilityStatusUnknown &&
		record.ContinuationStatus != model.ChannelCapabilityStatusUnknown
}

func probeResponsesCompactionCapabilities(channel *model.Channel, testUserID int, force bool) {
	if !force && !common.GetEnvOrDefaultBool("RESPONSES_COMPACTION_PROBE_ENABLED", false) {
		return
	}
	for _, modelName := range responsesCompactionProbeModels(channel) {
		if record, found := model.GetChannelModelCapability(channel.Id, modelName, model.ChannelCapabilityResponsesCompaction); found &&
			record.NextProbeAt > common.GetTimestamp() &&
			(strings.EqualFold(record.Source, "probe") || responsesCompactionObservationComplete(record)) {
			continue
		}
		probeTimeoutSeconds := responsesCompactionProbeTimeoutSeconds()
		legacyRequest := &dto.OpenAIResponsesCompactionRequest{
			Model: modelName,
			Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"Compress this channel capability probe state."}]}]`),
		}
		responsesCompactionProbeSemaphore <- struct{}{}
		result := testChannelWithRequest(channel, testUserID, modelName, string(constant.EndpointTypeOpenAIResponseCompact), false, &channelTestRequestOverride{
			Request:        legacyRequest,
			Kind:           dto.ResponsesCompactEndpoint,
			TimeoutSeconds: probeTimeoutSeconds,
		})
		<-responsesCompactionProbeSemaphore
		if result.localErr != nil && result.newAPIError == nil {
			common.SysLog(fmt.Sprintf("responses compaction probe skipped: channel=%d model=%s error=%s", channel.Id, modelName, result.localErr.Error()))
			continue
		}
		service.ObserveResponsesCapabilityAttempt(channel, modelName, service.ResponsesCapabilityAttempt{
			Kind:       dto.ResponsesCompactEndpoint,
			UsedLegacy: true,
			Source:     "probe",
		}, result.newAPIError)

		nativeRequest, err := buildResponsesCompactionProbeRequest(modelName, false, nil)
		if err != nil {
			common.SysLog(fmt.Sprintf("responses native compaction probe skipped: channel=%d model=%s error=%s", channel.Id, modelName, err.Error()))
			continue
		}
		responsesCompactionProbeSemaphore <- struct{}{}
		nativeResult := testChannelWithRequest(channel, testUserID, modelName, string(constant.EndpointTypeOpenAIResponse), false, &channelTestRequestOverride{
			Request:        nativeRequest,
			Kind:           dto.ResponsesCompactionTrigger,
			TimeoutSeconds: probeTimeoutSeconds,
		})
		<-responsesCompactionProbeSemaphore
		service.ObserveResponsesCapabilityAttempt(channel, modelName, service.ResponsesCapabilityAttempt{
			Kind:   dto.ResponsesCompactionTrigger,
			Source: "probe",
		}, nativeResult.newAPIError)

		streamRequest, err := buildResponsesCompactionProbeRequest(modelName, true, nil)
		if err != nil {
			common.SysLog(fmt.Sprintf("responses native stream compaction probe skipped: channel=%d model=%s error=%s", channel.Id, modelName, err.Error()))
			continue
		}
		responsesCompactionProbeSemaphore <- struct{}{}
		streamResult := testChannelWithRequest(channel, testUserID, modelName, string(constant.EndpointTypeOpenAIResponse), true, &channelTestRequestOverride{
			Request:        streamRequest,
			Kind:           dto.ResponsesCompactionTrigger,
			IsStream:       true,
			TimeoutSeconds: probeTimeoutSeconds,
		})
		<-responsesCompactionProbeSemaphore
		service.ObserveResponsesCapabilityAttempt(channel, modelName, service.ResponsesCapabilityAttempt{
			Kind:         dto.ResponsesCompactionTrigger,
			ClientStream: true,
			Source:       "probe",
		}, streamResult.newAPIError)

		compactedItem := nativeResult.compactionItem
		if len(compactedItem) == 0 {
			compactedItem = streamResult.compactionItem
		}
		if len(compactedItem) > 0 {
			continuationRequest, buildErr := buildResponsesCompactionProbeRequest(modelName, false, compactedItem)
			if buildErr == nil {
				responsesCompactionProbeSemaphore <- struct{}{}
				continuationResult := testChannelWithRequest(channel, testUserID, modelName, string(constant.EndpointTypeOpenAIResponse), false, &channelTestRequestOverride{
					Request:        continuationRequest,
					Kind:           dto.ResponsesCompactedContextContinuation,
					TimeoutSeconds: probeTimeoutSeconds,
				})
				<-responsesCompactionProbeSemaphore
				service.ObserveResponsesCapabilityAttempt(channel, modelName, service.ResponsesCapabilityAttempt{
					Kind:   dto.ResponsesCompactedContextContinuation,
					Source: "probe",
				}, continuationResult.newAPIError)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (t *channelTestTracker) recordTest(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channelLastTest[channelID] = time.Now()
}

func (t *channelTestTracker) recordSuccess(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channelFailCount[channelID] = 0
}

func (t *channelTestTracker) recordFailure(channelID int) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channelFailCount[channelID]++
	return t.channelFailCount[channelID]
}

// lastTestSince returns how long ago the channel was last tested; if never, a
// very large duration is returned so the first test always runs.
func (t *channelTestTracker) lastTestSince(channelID int) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	last, ok := t.channelLastTest[channelID]
	if !ok {
		return 365 * 24 * time.Hour
	}
	return time.Since(last)
}

func (t *channelTestTracker) failCount(channelID int) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.channelFailCount[channelID]
}

// cleanup drops tracking entries for channels that no longer exist so the
// in-memory maps don't grow unbounded as channels are deleted over time.
func (t *channelTestTracker) cleanup(activeIDs map[int]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.channelLastTest {
		if !activeIDs[id] {
			delete(t.channelLastTest, id)
		}
	}
	for id := range t.channelFailCount {
		if !activeIDs[id] {
			delete(t.channelFailCount, id)
		}
	}
}

// GetChannelTestConfig resolves the effective test config for a channel.
// Channel-level settings override global defaults.
func getEffectiveTestConfig(channel *model.Channel) (interval time.Duration, retryCount int, retryThreshold int, timeWindowStart string, timeWindowEnd string, timezone string) {
	// Global defaults
	interval = time.Duration(math.Round(operation_setting.GetMonitorSetting().AutoTestChannelMinutes)) * time.Minute
	retryCount = 2
	retryThreshold = 2
	timeWindowStart = "08:00"
	timeWindowEnd = "18:00"
	timezone = "Asia/Taipei"

	// Channel overrides
	cs := channel.GetSetting()
	if cs.AutoTestInterval > 0 {
		interval = time.Duration(cs.AutoTestInterval) * time.Minute
	}
	if cs.AutoTestRetryCount > 0 {
		retryCount = cs.AutoTestRetryCount
	}
	if cs.AutoTestRetryThreshold > 0 {
		retryThreshold = cs.AutoTestRetryThreshold
	}
	if cs.AutoTestTimeWindowStart != "" {
		timeWindowStart = cs.AutoTestTimeWindowStart
	}
	if cs.AutoTestTimeWindowEnd != "" {
		timeWindowEnd = cs.AutoTestTimeWindowEnd
	}
	if cs.AutoTestTimezone != "" {
		timezone = cs.AutoTestTimezone
	}
	return
}

// isInTestWindow checks whether the current time falls within the channel's
// configured test window. An empty window means always allowed.
func isInTestWindow(timeWindowStart, timeWindowEnd string) bool {
	return isInTestWindowWithTimezone(timeWindowStart, timeWindowEnd, "")
}

func isInTestWindowWithTimezone(timeWindowStart, timeWindowEnd, timezone string) bool {
	if timeWindowStart == "" || timeWindowEnd == "" {
		return true
	}
	now := time.Now()
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			now = now.In(loc)
		} else {
			common.SysLog("auto-test: invalid timezone " + timezone)
		}
	}
	nowTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	return isClockInTestWindow(nowTime, timeWindowStart, timeWindowEnd)
}

func isClockInTestWindow(nowTime, timeWindowStart, timeWindowEnd string) bool {
	if timeWindowStart <= timeWindowEnd {
		return nowTime >= timeWindowStart && nowTime <= timeWindowEnd
	}
	// Window spans midnight, e.g. 22:00-06:00
	return nowTime >= timeWindowStart || nowTime <= timeWindowEnd
}

func testSingleChannelWithRetries(channel *model.Channel, testUserID int, retryCount int, retryThreshold int) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("recovered panic testing channel %d: %v", channel.Id, r))
		}
	}()
	if retryCount < 1 {
		retryCount = 1
	}
	if retryThreshold < 1 {
		retryThreshold = 1
	}
	isChannelEnabled := channel.Status == common.ChannelStatusEnabled

	// One test cycle: retry up to retryCount times to confirm a failure before
	// counting this cycle as failed. A single success at any attempt counts the
	// whole cycle as a success.
	var lastResult testResult
	var lastElapsed int64
	var succeeded bool
	for attempt := 0; attempt < retryCount; attempt++ {
		tStart := time.Now()
		lastResult = testChannel(channel, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(channel))
		elapsed := time.Since(tStart).Milliseconds()
		lastElapsed = elapsed

		if lastResult.newAPIError == nil {
			succeeded = true
			service.RecordChannelModelSuccess(channel.Id,
				common.GetContextKeyString(lastResult.context, constant.ContextKeyUsingGroup),
				common.GetContextKeyString(lastResult.context, constant.ContextKeyOriginalModel),
				"",
				common.GetContextKeyString(lastResult.context, common.RequestIdKey))
			channel.UpdateResponseTime(elapsed)
			break
		}
		service.RecordChannelModelFailure(service.ChannelModelFailureParams{
			ChannelId: channel.Id,
			Group:     common.GetContextKeyString(lastResult.context, constant.ContextKeyUsingGroup),
			ModelName: common.GetContextKeyString(lastResult.context, constant.ContextKeyOriginalModel),
			RequestId: common.GetContextKeyString(lastResult.context, common.RequestIdKey),
			Error:     lastResult.newAPIError,
			AutoBan:   channel.GetAutoBan(),
		})
		// Only meaningful disable-worthy errors justify another retry attempt.
		// Deterministic errors (e.g. model price misconfig) won't change between
		// attempts, so stop early instead of wasting retries.
		if !service.IsAntiPoisonValidationError(lastResult.newAPIError) && !service.ShouldDisableChannel(lastResult.newAPIError) {
			break
		}
		if attempt < retryCount-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if succeeded {
		// Reset consecutive failure counter; re-enable if previously auto-disabled.
		testTracking.recordSuccess(channel.Id)
		if !isChannelEnabled && service.ShouldEnableChannel(nil, channel.Status) {
			service.EnableChannel(channel.Id,
				common.GetContextKeyString(lastResult.context, constant.ContextKeyChannelKey),
				channel.Name)
		}
		return
	}

	// Failed cycles still count as a completed test. Persisting test_time makes
	// the channel test UI reflect that the scheduler actually ran.
	channel.UpdateResponseTime(lastElapsed)

	// Cycle failed. Decide whether to disable based on consecutive-failure threshold.
	if isChannelEnabled && (service.IsAntiPoisonValidationError(lastResult.newAPIError) || service.ShouldDisableChannel(lastResult.newAPIError)) {
		failures := testTracking.recordFailure(channel.Id)
		logger.LogInfo(nil, fmt.Sprintf("channel %d (%s) auto-test failed (%d/%d consecutive)",
			channel.Id, channel.Name, failures, retryThreshold))
		antiPoisonRisk := service.IsAntiPoisonValidationError(lastResult.newAPIError)
		if antiPoisonRisk || (failures >= retryThreshold && channel.GetAutoBan()) {
			processChannelError(lastResult.context,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name,
					channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(lastResult.context, constant.ContextKeyChannelKey),
					channel.GetAutoBan()),
				lastResult.newAPIError)
			// Reset counter after disabling so re-enable logic starts fresh.
			testTracking.recordSuccess(channel.Id)
		}
	}
}

// runIndependentChannelTest is called by the global auto-test loop. It tests
// channels that are due according to their individual schedules.
func runIndependentChannelTest() {
	if !operation_setting.GetMonitorSetting().AutoTestChannelEnabled {
		return
	}

	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		common.SysLog("auto-test: failed to resolve test user: " + err.Error())
		return
	}

	// Reuse the in-memory channel cache when available to avoid a full DB load
	// (including keys) on every 30s scan. Fall back to a direct query when the
	// memory cache is disabled or empty.
	var channels []*model.Channel
	if common.MemoryCacheEnabled {
		channels = model.CacheGetAllChannels()
	}
	if len(channels) == 0 {
		channels, err = model.GetAllChannels(0, 0, true, false)
		if err != nil {
			common.SysLog("auto-test: failed to get channels: " + err.Error())
			return
		}
	}

	// Drop tracking state for channels that no longer exist.
	activeIDs := make(map[int]bool, len(channels))
	for _, channel := range channels {
		activeIDs[channel.Id] = true
	}
	testTracking.cleanup(activeIDs)

	for _, channel := range channels {
		// Skip channels that are disabled (not auto-disabled)
		if channel.Status != common.ChannelStatusEnabled &&
			channel.Status != common.ChannelStatusAutoDisabled {
			continue
		}

		// Respect the "allow auto test & recover" flag
		if !channel.AllowAutoTestAndRecover() {
			continue
		}

		// Check per-channel schedule
		interval, retryCount, retryThreshold, twStart, twEnd, timezone := getEffectiveTestConfig(channel)
		since := testTracking.lastTestSince(channel.Id)
		if since < interval {
			continue // Not due yet
		}

		// Check time window
		if !isInTestWindowWithTimezone(twStart, twEnd, timezone) {
			continue
		}

		// Test this channel
		testTracking.recordTest(channel.Id)
		go probeResponsesCompactionCapabilities(channel, testUserID, false)
		go testSingleChannelWithRetries(channel, testUserID, retryCount, retryThreshold)

		// Stagger tests to avoid thundering herd
		time.Sleep(100 * time.Millisecond)
	}
}

// AutomaticallyTestChannelsWithIndependentSchedule replaces the original
// AutomaticallyTestChannels with per-channel scheduling support.
func startIndependentAutoTest() {
	if !common.IsMasterNode {
		return
	}

	// Scan frequency: every 30 seconds, check if any channels are due
	scanInterval := 30 * time.Second

	go func() {
		for {
			if !operation_setting.GetMonitorSetting().AutoTestChannelEnabled {
				time.Sleep(1 * time.Minute)
				continue
			}
			for {
				runIndependentChannelTest()
				time.Sleep(scanInterval)
				if !operation_setting.GetMonitorSetting().AutoTestChannelEnabled {
					break
				}
			}
		}
	}()
}
