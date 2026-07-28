package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// Official model-price sync.
//
// Requirement: keep all model prices aligned with official pricing, refreshed
// daily, and stop syncing prices from arbitrary peer channels. models.dev is an
// open, community-maintained aggregate of official provider pricing
// (OpenAI / Anthropic / Google / etc.), so it is used as the single official
// source here.
//
// The sync is authoritative for official models: all models present in
// models.dev official provider buckets are overwritten with the latest official
// prices. Local-only custom models that do not appear in models.dev are kept.

const (
	officialPriceSourceURL  = "https://models.dev/api.json"
	officialSyncInterval    = 24 * time.Hour
	officialSyncTimeOfDay   = "07:00"
	officialSyncHTTPTimeout = 30 * time.Second
)

var (
	officialSyncOnce    sync.Once
	officialSyncRun     = make(chan struct{}, 1)
	officialSyncStateMu sync.RWMutex
	officialSyncState   = struct {
		LastRunUnix    int64
		LastOK         bool
		LastError      string
		LastModelsNum  int
		LastChangedNum int
	}{}
)

// fetchOfficialPricing downloads and converts models.dev pricing into ratio maps.
func fetchOfficialPricing() (map[string]any, error) {
	return fetchOfficialPricingContext(context.Background())
}

func fetchOfficialPricingContext(ctx context.Context) (map[string]any, error) {
	client := &http.Client{Timeout: officialSyncHTTPTimeout}

	// Simple retry with backoff (mirrors existing ratio_sync behavior).
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialPriceSourceURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "new-api-official-price-sync")
		resp, lastErr = client.Do(req)
		if lastErr == nil {
			break
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(300*(1<<attempt)) * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official price source returned %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, maxRatioConfigBytes)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return convertModelsDevToRatioData(bytes.NewReader(bodyBytes))
}

func officialModelSet(converted map[string]any) map[string]bool {
	result := make(map[string]bool)
	raw, ok := converted[modelsDevOfficialNamesField]
	if !ok {
		return result
	}
	if m, ok := raw.(map[string]any); ok {
		for k, v := range m {
			if enabled, ok := v.(bool); !ok || enabled {
				result[k] = true
			}
		}
	}
	return result
}

// replaceOfficialRatioField rewrites pricing for official models and preserves
// local-only custom models. If a model is official but a specific lane no
// longer exists in models.dev, the stale local lane is removed.
func replaceOfficialRatioField(current map[string]float64, raw any, officialModels map[string]bool) (map[string]float64, int) {
	officialValues := make(map[string]float64)
	if m, ok := raw.(map[string]any); ok {
		for k, v := range m {
			if !officialModels[k] {
				continue
			}
			if f, ok := toFloat(v); ok && isValidNonNegativeCost(f) {
				officialValues[k] = roundRatioValue(f)
			}
		}
	}

	merged := make(map[string]float64, len(current)+len(officialValues))
	changed := 0
	for k, v := range current {
		if officialModels[k] {
			if _, hasOfficialValue := officialValues[k]; !hasOfficialValue {
				changed++
				continue
			}
		}
		merged[k] = v
	}

	for k, officialValue := range officialValues {
		if currentValue, exists := current[k]; !exists || !nearlyEqual(currentValue, officialValue) {
			changed++
		}
		merged[k] = officialValue
	}
	return merged, changed
}

func updateFloatOption(key string, values map[string]float64, updater func(string) error) error {
	jsonStr, err := common.Marshal(values)
	if err != nil {
		return err
	}
	if err := model.UpdateOption(key, string(jsonStr)); err != nil {
		return err
	}
	return updater(string(jsonStr))
}

// applyOfficialPricing aligns local pricing maps with models.dev official
// prices for every official model, while preserving custom local-only models.
func applyOfficialPricing(converted map[string]any) (int, error) {
	officialModels := officialModelSet(converted)
	if len(officialModels) == 0 {
		return 0, fmt.Errorf("official models.dev payload did not include official model names")
	}

	changedTotal := 0
	if raw, ok := converted["model_ratio"]; ok {
		cur, changed := replaceOfficialRatioField(ratio_setting.GetModelRatioCopy(), raw, officialModels)
		if changed > 0 {
			changedTotal += changed
			if err := updateFloatOption("ModelRatio", cur, ratio_setting.UpdateModelRatioByJSONString); err != nil {
				return changedTotal, err
			}
		}
	}

	if raw, ok := converted["completion_ratio"]; ok {
		cur, changed := replaceOfficialRatioField(ratio_setting.GetCompletionRatioCopy(), raw, officialModels)
		if changed > 0 {
			changedTotal += changed
			if err := updateFloatOption("CompletionRatio", cur, ratio_setting.UpdateCompletionRatioByJSONString); err != nil {
				return changedTotal, err
			}
		}
	}

	if raw, ok := converted["cache_ratio"]; ok {
		cur, changed := replaceOfficialRatioField(ratio_setting.GetCacheRatioCopy(), raw, officialModels)
		if changed > 0 {
			changedTotal += changed
			if err := updateFloatOption("CacheRatio", cur, ratio_setting.UpdateCacheRatioByJSONString); err != nil {
				return changedTotal, err
			}
		}
	}

	if raw, ok := converted["create_cache_ratio"]; ok {
		cur, changed := replaceOfficialRatioField(ratio_setting.GetCreateCacheRatioCopy(), raw, officialModels)
		if changed > 0 {
			changedTotal += changed
			if err := updateFloatOption("CreateCacheRatio", cur, ratio_setting.UpdateCreateCacheRatioByJSONString); err != nil {
				return changedTotal, err
			}
		}
	}

	return changedTotal, nil
}

func toFloat(v any) (float64, bool) {
	switch f := v.(type) {
	case float64:
		return f, true
	case float32:
		return float64(f), true
	case int:
		return float64(f), true
	case int64:
		return float64(f), true
	}
	return 0, false
}

func nextOfficialSyncTime(now time.Time) time.Time {
	parsed, err := time.ParseInLocation("15:04", officialSyncTimeOfDay, now.Location())
	if err != nil {
		return now.Add(officialSyncInterval)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// RunOfficialPriceSync performs a single official price sync cycle. Safe to call
// from a scheduled task or a manual admin trigger.
func RunOfficialPriceSync() (int, error) {
	return RunOfficialPriceSyncContext(context.Background())
}

func RunOfficialPriceSyncContext(ctx context.Context) (changed int, err error) {
	select {
	case officialSyncRun <- struct{}{}:
		defer func() { <-officialSyncRun }()
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("official price sync panic: %v", r)
			common.SysError(err.Error())
		}
	}()

	converted, err := fetchOfficialPricingContext(ctx)
	if err != nil {
		officialSyncStateMu.Lock()
		officialSyncState.LastRunUnix = time.Now().Unix()
		officialSyncState.LastOK = false
		officialSyncState.LastError = err.Error()
		officialSyncStateMu.Unlock()
		common.SysError("official price sync fetch failed: " + err.Error())
		return 0, err
	}

	merged, err := applyOfficialPricing(converted)
	officialSyncStateMu.Lock()
	officialSyncState.LastRunUnix = time.Now().Unix()
	if err != nil {
		officialSyncState.LastOK = false
		officialSyncState.LastError = err.Error()
		officialSyncStateMu.Unlock()
		common.SysError("official price sync apply failed: " + err.Error())
		return merged, err
	}

	officialSyncState.LastOK = true
	officialSyncState.LastError = ""
	officialSyncState.LastChangedNum = merged
	if names := officialModelSet(converted); len(names) > 0 {
		officialSyncState.LastModelsNum = len(names)
	}
	modelsNum := officialSyncState.LastModelsNum
	officialSyncStateMu.Unlock()
	common.SysLog(fmt.Sprintf("official price sync ok: %d official models, %d pricing entries changed", modelsNum, merged))
	return merged, nil
}

// StartOfficialPriceSyncTask runs the daily official price sync on the master
// node at 07:00 in the process local timezone.
func StartOfficialPriceSyncTask() {
	officialSyncOnce.Do(func() {
		go RunOfficialPriceSyncTask(context.Background())
	})
}

func RunOfficialPriceSyncTask(ctx context.Context) {
	if !common.IsMasterNode {
		return
	}
	for {
		next := nextOfficialSyncTime(time.Now())
		common.SysLog(fmt.Sprintf("official price sync scheduled at %s", next.Format(time.RFC3339)))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			_, _ = RunOfficialPriceSyncContext(ctx)
		}
	}
}

// OfficialPriceSyncStatus returns a snapshot of the last sync state for the UI.
func OfficialPriceSyncStatus() map[string]any {
	officialSyncStateMu.RLock()
	defer officialSyncStateMu.RUnlock()
	return map[string]any{
		"last_run_unix":    officialSyncState.LastRunUnix,
		"last_ok":          officialSyncState.LastOK,
		"last_error":       officialSyncState.LastError,
		"last_models_num":  officialSyncState.LastModelsNum,
		"last_changed_num": officialSyncState.LastChangedNum,
		"source_url":       officialPriceSourceURL,
	}
}
