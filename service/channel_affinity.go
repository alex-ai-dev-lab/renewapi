package service

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/tidwall/gjson"
)

const (
	ginKeyChannelAffinityCacheKey   = "channel_affinity_cache_key"
	ginKeyChannelAffinityTTLSeconds = "channel_affinity_ttl_seconds"
	ginKeyChannelAffinityMeta       = "channel_affinity_meta"
	ginKeyChannelAffinityLogInfo    = "channel_affinity_log_info"
	ginKeyChannelAffinitySkipRetry  = "channel_affinity_skip_retry_on_failure"
	ginKeyChannelAffinityMatchedKey = "channel_affinity_matched_key"
	ginKeyChannelAffinityMatchedV2  = "channel_affinity_matched_record_v2"
	ginKeyRelaySemanticSuccess      = "relay_semantic_success"
	ginKeyCompactionResponseHashes  = "responses_compaction_response_hashes"
	ginKeySessionRecoveryLogInfo    = "session_recovery_log_info"

	channelAffinityCacheNamespace           = "new-api:channel_affinity:v3"
	channelAffinityRecordNamespace          = "new-api:channel_affinity:v3_record"
	channelAffinityMultiKeyIndexNamespace   = "new-api:channel_affinity_multi_key_index:v3"
	channelAffinityNegativeNamespace        = "new-api:channel_affinity_negative:v3"
	channelAffinityNegativeV2Namespace      = "new-api:channel_affinity_negative:v3_record"
	channelAffinityKeyNegativeNamespace     = "new-api:channel_affinity_key_negative:v3"
	channelAffinityRecoveryChainNamespace   = "new-api:channel_affinity_recovery_chain:v3"
	channelAffinityUsageCacheStatsNamespace = "new-api:channel_affinity_usage_cache_stats:v3"
)

type ChannelAffinityRecord struct {
	ChannelID  int    `json:"channel_id"`
	Generation string `json:"generation"`
	RecordedAt int64  `json:"recorded_at"`
}

type sessionRecoveryChain struct {
	FirstFailureAt   int64  `json:"first_failure_at"`
	RouteChanges     int    `json:"route_changes"`
	LastChannelID    int    `json:"last_channel_id"`
	LastFailureToken string `json:"last_failure_token"`
}

type channelSessionNegative struct {
	FailedGeneration string `json:"failed_generation"`
	FailedAt         int64  `json:"failed_at"`
}

func affinityRecordEqual(a, b ChannelAffinityRecord) bool {
	return a.ChannelID == b.ChannelID && a.Generation == b.Generation && a.RecordedAt == b.RecordedAt
}

func sessionRecoveryChainEqual(a, b sessionRecoveryChain) bool {
	return a == b
}

func channelSessionNegativeEqual(a, b channelSessionNegative) bool { return a == b }

func compactionAffinitySuffix(c *gin.Context, modelName, usingGroup, contentHash string) string {
	tenant := fmt.Sprintf("token:%d:user:%d", common.GetContextKeyInt(c, constant.ContextKeyTokenId), common.GetContextKeyInt(c, constant.ContextKeyUserId))
	message := strings.Join([]string{tenant, usingGroup, strings.TrimSuffix(modelName, "-openai-compact"), contentHash}, "\n")
	return "compaction:" + common.HmacSha256(message, common.CryptoSecret)
}

func getAffinityRecord(cacheKey string) (ChannelAffinityRecord, bool, error) {
	key := channelAffinityRawCacheKey(cacheKey)
	if key == "" {
		return ChannelAffinityRecord{}, false, nil
	}
	record, found, err := getChannelAffinityRecordCache().Get(key)
	if err != nil || found {
		return record, found, err
	}
	legacyChannelID, legacyFound, legacyErr := getChannelAffinityCache().Get(key)
	if legacyErr != nil || !legacyFound || legacyChannelID <= 0 {
		return ChannelAffinityRecord{}, false, legacyErr
	}
	// Legacy values remain readable until their original TTL expires. They do
	// not carry a trustworthy generation and must not be promoted implicitly.
	return ChannelAffinityRecord{ChannelID: legacyChannelID}, true, nil
}

func rememberMatchedAffinity(c *gin.Context, key string, record ChannelAffinityRecord) {
	if c == nil {
		return
	}
	c.Set(ginKeyChannelAffinityMatchedKey, channelAffinityRawCacheKey(key))
	c.Set(ginKeyChannelAffinityMatchedV2, record)
}

func matchedAffinityRecord(c *gin.Context) (ChannelAffinityRecord, bool) {
	record, ok := observedAffinityRecord(c)
	return record, ok && record.Generation != ""
}

func observedAffinityRecord(c *gin.Context) (ChannelAffinityRecord, bool) {
	if c == nil {
		return ChannelAffinityRecord{}, false
	}
	raw, ok := c.Get(ginKeyChannelAffinityMatchedV2)
	record, valid := raw.(ChannelAffinityRecord)
	return record, ok && valid && record.ChannelID > 0
}

func GetPreferredChannelByCompactionAffinity(c *gin.Context, modelName, usingGroup string) (int, bool) {
	value, ok := c.Get(ContextKeyResponsesCompactedHashes)
	if !ok {
		return 0, false
	}
	hashes, ok := value.([]string)
	if !ok {
		return 0, false
	}
	for _, contentHash := range hashes {
		suffix := compactionAffinitySuffix(c, modelName, usingGroup, contentHash)
		record, found, err := getAffinityRecord(suffix)
		if err != nil {
			common.SysError(fmt.Sprintf("compaction affinity cache get failed: err=%v", err))
			return 0, false
		}
		if found {
			if channelAffinityNegativeForRecord(suffix, record) {
				continue
			}
			channelID := record.ChannelID
			rememberMatchedAffinity(c, suffix, record)
			if multiKeyIndex, indexFound := getChannelAffinityPreferredMultiKeyIndex(suffix); indexFound {
				common.SetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyChannelId, channelID)
				common.SetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyIndex, multiKeyIndex)
			}
			return channelID, true
		}
	}
	return 0, false
}

func CaptureCompactionResponseAffinity(c *gin.Context, body []byte) {
	if c == nil {
		return
	}
	hashes := CompactionResponseContentHashes(body)
	if len(hashes) > 0 {
		c.Set(ginKeyCompactionResponseHashes, hashes)
	}
}

func CaptureCompactionItemAffinity(c *gin.Context, item []byte) {
	if c == nil || !gjson.ValidBytes(item) {
		return
	}
	typ := gjson.GetBytes(item, "type").String()
	if typ != "compaction" && typ != "context_compaction" && typ != "compaction_summary" {
		return
	}
	encrypted := gjson.GetBytes(item, "encrypted_content")
	if encrypted.Type != gjson.String || encrypted.String() == "" {
		return
	}
	sum := common.Sha256Raw([]byte(encrypted.String()))
	c.Set(ginKeyCompactionResponseHashes, []string{fmt.Sprintf("%x", sum)})
}

func RecordCompactionResponseAffinity(c *gin.Context, channelID int, modelName, usingGroup string) {
	if c == nil || channelID <= 0 {
		return
	}
	value, ok := c.Get(ginKeyCompactionResponseHashes)
	if !ok {
		return
	}
	hashes, ok := value.([]string)
	if !ok {
		return
	}
	ttl := 3600
	if setting := operation_setting.GetChannelAffinitySetting(); setting != nil && setting.DefaultTTLSeconds > 0 {
		ttl = setting.DefaultTTLSeconds
	}
	for _, contentHash := range hashes {
		suffix := compactionAffinitySuffix(c, modelName, usingGroup, contentHash)
		record := ChannelAffinityRecord{ChannelID: channelID, Generation: common.GetUUID(), RecordedAt: time.Now().UnixNano()}
		if err := getChannelAffinityRecordCache().SetWithTTL(suffix, record, time.Duration(ttl)*time.Second); err != nil {
			common.SysError(fmt.Sprintf("compaction affinity v2 cache set failed: err=%v", err))
		}
		if err := getChannelAffinityCache().SetWithTTL(suffix, channelID, time.Duration(ttl)*time.Second); err != nil {
			common.SysError(fmt.Sprintf("compaction affinity cache set failed: err=%v", err))
		}
		if common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
			index := common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
			if index >= 0 {
				if err := getChannelAffinityMultiKeyIndexCache().SetWithTTL(suffix, index, time.Duration(ttl)*time.Second); err != nil {
					common.SysError(fmt.Sprintf("compaction affinity multi-key index set failed: err=%v", err))
				}
			}
		}
	}
}

var (
	channelAffinityCacheOnce       sync.Once
	channelAffinityCache           *cachex.HybridCache[int]
	channelAffinityRecordCacheOnce sync.Once
	channelAffinityRecordCache     *cachex.HybridCache[ChannelAffinityRecord]

	channelAffinityMultiKeyIndexCacheOnce sync.Once
	channelAffinityMultiKeyIndexCache     *cachex.HybridCache[int]
	channelAffinityNegativeCacheOnce      sync.Once
	channelAffinityNegativeCache          *cachex.HybridCache[int]
	channelAffinityNegativeV2CacheOnce    sync.Once
	channelAffinityNegativeV2Cache        *cachex.HybridCache[channelSessionNegative]
	channelAffinityKeyNegativeCacheOnce   sync.Once
	channelAffinityKeyNegativeCache       *cachex.HybridCache[int]
	channelAffinityRecoveryChainCacheOnce sync.Once
	channelAffinityRecoveryChainCache     *cachex.HybridCache[sessionRecoveryChain]

	channelAffinityUsageCacheStatsOnce  sync.Once
	channelAffinityUsageCacheStatsCache *cachex.HybridCache[ChannelAffinityUsageCacheCounters]

	channelAffinityRegexCache sync.Map // map[string]*regexp.Regexp
)

func affinityCacheCapacityAndTTL() (int, int) {
	capacity, ttl := 100_000, 3600
	if setting := operation_setting.GetChannelAffinitySetting(); setting != nil {
		if setting.MaxEntries > 0 {
			capacity = setting.MaxEntries
		}
		if setting.DefaultTTLSeconds > 0 {
			ttl = setting.DefaultTTLSeconds
		}
	}
	return capacity, ttl
}

func getChannelAffinityRecordCache() *cachex.HybridCache[ChannelAffinityRecord] {
	channelAffinityRecordCacheOnce.Do(func() {
		capacity, ttl := affinityCacheCapacityAndTTL()
		channelAffinityRecordCache = cachex.NewHybridCache[ChannelAffinityRecord](cachex.HybridCacheConfig[ChannelAffinityRecord]{
			Namespace: cachex.Namespace(channelAffinityRecordNamespace), Redis: common.RDB,
			RedisEnabled: func() bool { return common.RedisEnabled && common.RDB != nil },
			RedisCodec:   cachex.JSONCodec[ChannelAffinityRecord]{},
			Memory: func() *hot.HotCache[string, ChannelAffinityRecord] {
				return hot.NewHotCache[string, ChannelAffinityRecord](hot.LRU, capacity).WithTTL(time.Duration(ttl) * time.Second).WithJanitor().Build()
			},
		})
	})
	return channelAffinityRecordCache
}

type channelAffinityMeta struct {
	CacheKey       string
	TTLSeconds     int
	RuleName       string
	SkipRetry      bool
	ParamTemplate  map[string]interface{}
	KeySourceType  string
	KeySourceKey   string
	KeySourcePath  string
	KeyHint        string
	KeyFingerprint string
	UsingGroup     string
	ModelName      string
	RequestPath    string
}

type ChannelAffinityStatsContext struct {
	RuleName       string
	UsingGroup     string
	KeyFingerprint string
	TTLSeconds     int64
}

const (
	cacheTokenRateModeCachedOverPrompt           = "cached_over_prompt"
	cacheTokenRateModeCachedOverPromptPlusCached = "cached_over_prompt_plus_cached"
	cacheTokenRateModeMixed                      = "mixed"
)

type ChannelAffinityCacheStats struct {
	Enabled       bool           `json:"enabled"`
	Total         int            `json:"total"`
	Unknown       int            `json:"unknown"`
	ByRuleName    map[string]int `json:"by_rule_name"`
	CacheCapacity int            `json:"cache_capacity"`
	CacheAlgo     string         `json:"cache_algo"`
}

func getChannelAffinityCache() *cachex.HybridCache[int] {
	channelAffinityCacheOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := setting.MaxEntries
		if capacity <= 0 {
			capacity = 100_000
		}
		defaultTTLSeconds := setting.DefaultTTLSeconds
		if defaultTTLSeconds <= 0 {
			defaultTTLSeconds = 3600
		}

		channelAffinityCache = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(channelAffinityCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityCache
}

func getChannelAffinityMultiKeyIndexCache() *cachex.HybridCache[int] {
	channelAffinityMultiKeyIndexCacheOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}

		channelAffinityMultiKeyIndexCache = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(channelAffinityMultiKeyIndexNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityMultiKeyIndexCache
}

func getChannelAffinityNegativeCache() *cachex.HybridCache[int] {
	channelAffinityNegativeCacheOnce.Do(func() {
		capacity := 100_000
		if setting := operation_setting.GetChannelAffinitySetting(); setting != nil && setting.MaxEntries > 0 {
			capacity = setting.MaxEntries
		}
		channelAffinityNegativeCache = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(channelAffinityNegativeNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).
					WithTTL(30 * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityNegativeCache
}

func getChannelAffinityNegativeV2Cache() *cachex.HybridCache[channelSessionNegative] {
	channelAffinityNegativeV2CacheOnce.Do(func() {
		capacity, ttl := affinityCacheCapacityAndTTL()
		channelAffinityNegativeV2Cache = cachex.NewHybridCache[channelSessionNegative](cachex.HybridCacheConfig[channelSessionNegative]{
			Namespace: cachex.Namespace(channelAffinityNegativeV2Namespace), Redis: common.RDB,
			RedisEnabled: func() bool { return common.RedisEnabled && common.RDB != nil },
			RedisCodec:   cachex.JSONCodec[channelSessionNegative]{},
			Memory: func() *hot.HotCache[string, channelSessionNegative] {
				return hot.NewHotCache[string, channelSessionNegative](hot.LRU, capacity).WithTTL(time.Duration(ttl) * time.Second).WithJanitor().Build()
			},
		})
	})
	return channelAffinityNegativeV2Cache
}

func getChannelAffinityKeyNegativeCache() *cachex.HybridCache[int] {
	channelAffinityKeyNegativeCacheOnce.Do(func() {
		capacity, ttl := affinityCacheCapacityAndTTL()
		channelAffinityKeyNegativeCache = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(channelAffinityKeyNegativeNamespace), Redis: common.RDB,
			RedisEnabled: func() bool { return common.RedisEnabled && common.RDB != nil }, RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).WithTTL(time.Duration(ttl) * time.Second).WithJanitor().Build()
			},
		})
	})
	return channelAffinityKeyNegativeCache
}

func getChannelAffinityRecoveryChainCache() *cachex.HybridCache[sessionRecoveryChain] {
	channelAffinityRecoveryChainCacheOnce.Do(func() {
		capacity, ttl := affinityCacheCapacityAndTTL()
		channelAffinityRecoveryChainCache = cachex.NewHybridCache[sessionRecoveryChain](cachex.HybridCacheConfig[sessionRecoveryChain]{
			Namespace: cachex.Namespace(channelAffinityRecoveryChainNamespace), Redis: common.RDB,
			RedisEnabled: func() bool { return common.RedisEnabled && common.RDB != nil },
			RedisCodec:   cachex.JSONCodec[sessionRecoveryChain]{},
			Memory: func() *hot.HotCache[string, sessionRecoveryChain] {
				return hot.NewHotCache[string, sessionRecoveryChain](hot.LRU, capacity).WithTTL(time.Duration(ttl) * time.Second).WithJanitor().Build()
			},
		})
	})
	return channelAffinityRecoveryChainCache
}

func GetChannelAffinityCacheStats() ChannelAffinityCacheStats {
	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil {
		return ChannelAffinityCacheStats{
			Enabled:    false,
			Total:      0,
			Unknown:    0,
			ByRuleName: map[string]int{},
		}
	}

	cache := getChannelAffinityCache()
	mainCap, _ := cache.Capacity()
	mainAlgo, _ := cache.Algorithm()

	rules := setting.Rules
	ruleByName := make(map[string]operation_setting.ChannelAffinityRule, len(rules))
	for _, r := range rules {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			continue
		}
		if !r.IncludeRuleName {
			continue
		}
		ruleByName[name] = r
	}

	byRuleName := make(map[string]int, len(ruleByName))
	for name := range ruleByName {
		byRuleName[name] = 0
	}

	keys, err := cache.Keys()
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity cache list keys failed: err=%v", err))
		keys = nil
	}
	total := len(keys)
	unknown := 0
	for _, k := range keys {
		prefix := channelAffinityCacheNamespace + ":"
		if !strings.HasPrefix(k, prefix) {
			unknown++
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		parts := strings.Split(rest, ":")
		if len(parts) < 2 {
			unknown++
			continue
		}
		ruleName := parts[0]
		rule, ok := ruleByName[ruleName]
		if !ok {
			unknown++
			continue
		}
		if rule.IncludeModelName {
			if len(parts) < 3 {
				unknown++
				continue
			}
		}
		if rule.IncludeUsingGroup {
			minParts := 3
			if rule.IncludeModelName {
				minParts = 4
			}
			if len(parts) < minParts {
				unknown++
				continue
			}
		}
		byRuleName[ruleName]++
	}

	return ChannelAffinityCacheStats{
		Enabled:       setting.Enabled,
		Total:         total,
		Unknown:       unknown,
		ByRuleName:    byRuleName,
		CacheCapacity: mainCap,
		CacheAlgo:     mainAlgo,
	}
}

func ClearChannelAffinityCacheAll() int {
	cache := getChannelAffinityCache()
	keys, err := cache.Keys()
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity cache list keys failed: err=%v", err))
		keys = nil
	}
	if len(keys) > 0 {
		if _, err := cache.DeleteMany(keys); err != nil {
			common.SysError(fmt.Sprintf("channel affinity cache delete many failed: err=%v", err))
		}
	}
	if err := getChannelAffinityMultiKeyIndexCache().Purge(); err != nil {
		common.SysError(fmt.Sprintf("channel affinity multi-key index cache purge failed: err=%v", err))
	}
	_ = getChannelAffinityRecordCache().Purge()
	_ = getChannelAffinityNegativeCache().Purge()
	_ = getChannelAffinityNegativeV2Cache().Purge()
	_ = getChannelAffinityKeyNegativeCache().Purge()
	_ = getChannelAffinityRecoveryChainCache().Purge()
	return len(keys)
}

func ClearChannelAffinityCacheByRuleName(ruleName string) (int, error) {
	ruleName = strings.TrimSpace(ruleName)
	if ruleName == "" {
		return 0, fmt.Errorf("rule_name 不能为空")
	}

	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil {
		return 0, fmt.Errorf("channel_affinity_setting 未初始化")
	}

	var matchedRule *operation_setting.ChannelAffinityRule
	for i := range setting.Rules {
		r := &setting.Rules[i]
		if strings.TrimSpace(r.Name) != ruleName {
			continue
		}
		matchedRule = r
		break
	}
	if matchedRule == nil {
		return 0, fmt.Errorf("未知规则名称")
	}
	if !matchedRule.IncludeRuleName {
		return 0, fmt.Errorf("该规则未启用 include_rule_name，无法按规则清空缓存")
	}

	cache := getChannelAffinityCache()
	deleted, err := cache.DeleteByPrefix(ruleName)
	if err != nil {
		return 0, err
	}
	if _, idxErr := getChannelAffinityMultiKeyIndexCache().DeleteByPrefix(ruleName); idxErr != nil {
		common.SysError(fmt.Sprintf("channel affinity multi-key index cache delete failed: rule=%s, err=%v", ruleName, idxErr))
	}
	_, _ = getChannelAffinityRecordCache().DeleteByPrefix(ruleName)
	_, _ = getChannelAffinityNegativeCache().DeleteByPrefix(ruleName)
	_, _ = getChannelAffinityNegativeV2Cache().DeleteByPrefix(ruleName)
	_, _ = getChannelAffinityKeyNegativeCache().DeleteByPrefix(ruleName)
	_, _ = getChannelAffinityRecoveryChainCache().DeleteByPrefix(ruleName)
	return deleted, nil
}

func matchAnyRegexCached(patterns []string, s string) bool {
	if len(patterns) == 0 || s == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		re, ok := channelAffinityRegexCache.Load(pattern)
		if !ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			re = compiled
			channelAffinityRegexCache.Store(pattern, re)
		}
		if re.(*regexp.Regexp).MatchString(s) {
			return true
		}
	}
	return false
}

func matchAnyIncludeFold(patterns []string, s string) bool {
	if len(patterns) == 0 || s == "" {
		return false
	}
	sLower := strings.ToLower(s)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(sLower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func extractChannelAffinityValue(c *gin.Context, src operation_setting.ChannelAffinityKeySource) string {
	switch src.Type {
	case "context_int":
		if src.Key == "" {
			return ""
		}
		v := c.GetInt(src.Key)
		if v <= 0 {
			return ""
		}
		return strconv.Itoa(v)
	case "context_string":
		if src.Key == "" {
			return ""
		}
		return strings.TrimSpace(c.GetString(src.Key))
	case "request_header":
		if c == nil || c.Request == nil || src.Key == "" {
			return ""
		}
		return strings.TrimSpace(c.Request.Header.Get(src.Key))
	case "gjson":
		if src.Path == "" {
			return ""
		}
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return ""
		}
		body, err := storage.Bytes()
		if err != nil || len(body) == 0 {
			return ""
		}
		res := gjson.GetBytes(body, src.Path)
		if !res.Exists() {
			return ""
		}
		switch res.Type {
		case gjson.String, gjson.Number, gjson.True, gjson.False:
			return strings.TrimSpace(res.String())
		default:
			return strings.TrimSpace(res.Raw)
		}
	default:
		return ""
	}
}

func buildChannelAffinityCacheKeySuffix(rule operation_setting.ChannelAffinityRule, modelName string, usingGroup string, affinityValue string) string {
	parts := make([]string, 0, 4)
	if rule.IncludeRuleName && rule.Name != "" {
		parts = append(parts, rule.Name)
	}
	if rule.IncludeModelName && modelName != "" {
		parts = append(parts, modelName)
	}
	if rule.IncludeUsingGroup && usingGroup != "" {
		parts = append(parts, usingGroup)
	}
	parts = append(parts, affinityFingerprint(affinityValue))
	return strings.Join(parts, ":")
}

func setChannelAffinityContext(c *gin.Context, meta channelAffinityMeta) {
	c.Set(ginKeyChannelAffinityCacheKey, meta.CacheKey)
	c.Set(ginKeyChannelAffinityTTLSeconds, meta.TTLSeconds)
	c.Set(ginKeyChannelAffinityMeta, meta)
}

func getChannelAffinityContext(c *gin.Context) (string, int, bool) {
	keyAny, ok := c.Get(ginKeyChannelAffinityCacheKey)
	if !ok {
		return "", 0, false
	}
	key, ok := keyAny.(string)
	if !ok || key == "" {
		return "", 0, false
	}
	ttlAny, ok := c.Get(ginKeyChannelAffinityTTLSeconds)
	if !ok {
		return key, 0, true
	}
	ttlSeconds, _ := ttlAny.(int)
	return key, ttlSeconds, true
}

func getChannelAffinityMeta(c *gin.Context) (channelAffinityMeta, bool) {
	anyMeta, ok := c.Get(ginKeyChannelAffinityMeta)
	if !ok {
		return channelAffinityMeta{}, false
	}
	meta, ok := anyMeta.(channelAffinityMeta)
	if !ok {
		return channelAffinityMeta{}, false
	}
	return meta, true
}

func GetChannelAffinityStatsContext(c *gin.Context) (ChannelAffinityStatsContext, bool) {
	if c == nil {
		return ChannelAffinityStatsContext{}, false
	}
	meta, ok := getChannelAffinityMeta(c)
	if !ok {
		return ChannelAffinityStatsContext{}, false
	}
	ruleName := strings.TrimSpace(meta.RuleName)
	keyFp := strings.TrimSpace(meta.KeyFingerprint)
	usingGroup := strings.TrimSpace(meta.UsingGroup)
	if ruleName == "" || keyFp == "" {
		return ChannelAffinityStatsContext{}, false
	}
	ttlSeconds := int64(meta.TTLSeconds)
	if ttlSeconds <= 0 {
		return ChannelAffinityStatsContext{}, false
	}
	return ChannelAffinityStatsContext{
		RuleName:       ruleName,
		UsingGroup:     usingGroup,
		KeyFingerprint: keyFp,
		TTLSeconds:     ttlSeconds,
	}, true
}

func affinityFingerprint(s string) string {
	if s == "" {
		return ""
	}
	return common.HmacSha256(s, common.CryptoSecret)
}

func buildChannelAffinityKeyHint(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	fingerprint := affinityFingerprint(s)
	if len(fingerprint) > 12 {
		fingerprint = fingerprint[:12]
	}
	return "hmac:" + fingerprint
}

func cloneStringAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeChannelOverride(base map[string]interface{}, tpl map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(tpl) == 0 {
		return map[string]interface{}{}
	}
	if len(tpl) == 0 {
		return base
	}
	out := cloneStringAnyMap(base)
	for k, v := range tpl {
		if strings.EqualFold(strings.TrimSpace(k), "operations") {
			baseOps, hasBaseOps := extractParamOperations(out[k])
			tplOps, hasTplOps := extractParamOperations(v)
			if hasTplOps {
				if hasBaseOps {
					out[k] = append(tplOps, baseOps...)
				} else {
					out[k] = tplOps
				}
				continue
			}
		}
		if _, exists := out[k]; exists {
			continue
		}
		out[k] = v
	}
	return out
}

func extractParamOperations(value interface{}) ([]interface{}, bool) {
	switch ops := value.(type) {
	case []interface{}:
		if len(ops) == 0 {
			return []interface{}{}, true
		}
		cloned := make([]interface{}, 0, len(ops))
		cloned = append(cloned, ops...)
		return cloned, true
	case []map[string]interface{}:
		cloned := make([]interface{}, 0, len(ops))
		for _, op := range ops {
			cloned = append(cloned, op)
		}
		return cloned, true
	default:
		return nil, false
	}
}

func appendChannelAffinityTemplateAdminInfo(c *gin.Context, meta channelAffinityMeta) {
	if c == nil {
		return
	}
	if len(meta.ParamTemplate) == 0 {
		return
	}

	templateInfo := map[string]interface{}{
		"applied":             true,
		"rule_name":           meta.RuleName,
		"param_override_keys": len(meta.ParamTemplate),
	}
	if anyInfo, ok := c.Get(ginKeyChannelAffinityLogInfo); ok {
		if info, ok := anyInfo.(map[string]interface{}); ok {
			info["override_template"] = templateInfo
			c.Set(ginKeyChannelAffinityLogInfo, info)
			return
		}
	}
	c.Set(ginKeyChannelAffinityLogInfo, map[string]interface{}{
		"reason":            meta.RuleName,
		"rule_name":         meta.RuleName,
		"using_group":       meta.UsingGroup,
		"model":             meta.ModelName,
		"request_path":      meta.RequestPath,
		"key_source":        meta.KeySourceType,
		"key_key":           meta.KeySourceKey,
		"key_path":          meta.KeySourcePath,
		"key_hint":          meta.KeyHint,
		"key_fp":            meta.KeyFingerprint,
		"override_template": templateInfo,
	})
}

// ApplyChannelAffinityOverrideTemplate merges per-rule channel override templates onto the selected channel override config.
func ApplyChannelAffinityOverrideTemplate(c *gin.Context, paramOverride map[string]interface{}) (map[string]interface{}, bool) {
	if c == nil {
		return paramOverride, false
	}
	meta, ok := getChannelAffinityMeta(c)
	if !ok {
		return paramOverride, false
	}
	if len(meta.ParamTemplate) == 0 {
		return paramOverride, false
	}

	mergedParam := mergeChannelOverride(paramOverride, meta.ParamTemplate)
	appendChannelAffinityTemplateAdminInfo(c, meta)
	return mergedParam, true
}

func GetPreferredChannelByAffinity(c *gin.Context, modelName string, usingGroup string) (int, bool) {
	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil || !setting.Enabled {
		return 0, false
	}
	path := ""
	if c != nil && c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	userAgent := ""
	if c != nil && c.Request != nil {
		userAgent = c.Request.UserAgent()
	}

	for _, rule := range setting.Rules {
		if !matchAnyRegexCached(rule.ModelRegex, modelName) {
			continue
		}
		if len(rule.PathRegex) > 0 && !matchAnyRegexCached(rule.PathRegex, path) {
			continue
		}
		if len(rule.UserAgentInclude) > 0 && !matchAnyIncludeFold(rule.UserAgentInclude, userAgent) {
			continue
		}
		var affinityValue string
		var usedSource operation_setting.ChannelAffinityKeySource
		for _, src := range rule.KeySources {
			affinityValue = extractChannelAffinityValue(c, src)
			if affinityValue != "" {
				usedSource = src
				break
			}
		}
		if affinityValue == "" {
			continue
		}
		if rule.ValueRegex != "" && !matchAnyRegexCached([]string{rule.ValueRegex}, affinityValue) {
			continue
		}

		ttlSeconds := rule.TTLSeconds
		if ttlSeconds <= 0 {
			ttlSeconds = setting.DefaultTTLSeconds
		}
		cacheKeySuffix := buildChannelAffinityCacheKeySuffix(rule, modelName, usingGroup, affinityValue)
		cacheKeyFull := channelAffinityCacheNamespace + ":" + cacheKeySuffix
		setChannelAffinityContext(c, channelAffinityMeta{
			CacheKey:       cacheKeyFull,
			TTLSeconds:     ttlSeconds,
			RuleName:       rule.Name,
			SkipRetry:      rule.SkipRetryOnFailure,
			ParamTemplate:  cloneStringAnyMap(rule.ParamOverrideTemplate),
			KeySourceType:  strings.TrimSpace(usedSource.Type),
			KeySourceKey:   strings.TrimSpace(usedSource.Key),
			KeySourcePath:  strings.TrimSpace(usedSource.Path),
			KeyHint:        buildChannelAffinityKeyHint(affinityValue),
			KeyFingerprint: affinityFingerprint(affinityValue),
			UsingGroup:     usingGroup,
			ModelName:      modelName,
			RequestPath:    path,
		})

		record, found, err := getAffinityRecord(cacheKeySuffix)
		if err != nil {
			common.SysError(fmt.Sprintf("channel affinity cache get failed: key=%s, err=%v", cacheKeyFull, err))
			return 0, false
		}
		if found {
			if channelAffinityNegativeForRecord(cacheKeySuffix, record) {
				return 0, false
			}
			channelID := record.ChannelID
			rememberMatchedAffinity(c, cacheKeySuffix, record)
			if multiKeyIndex, indexFound := getChannelAffinityPreferredMultiKeyIndex(cacheKeySuffix); indexFound {
				common.SetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyChannelId, channelID)
				common.SetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyIndex, multiKeyIndex)
			}
			return channelID, true
		}
		return 0, false
	}
	return 0, false
}

func channelAffinityRawCacheKey(cacheKey string) string {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return ""
	}
	prefix := channelAffinityCacheNamespace + ":"
	return strings.TrimPrefix(cacheKey, prefix)
}

func channelAffinityNegativeKey(cacheKey string, channelID int) string {
	cacheKey = channelAffinityRawCacheKey(cacheKey)
	if cacheKey == "" || channelID <= 0 {
		return ""
	}
	return cacheKey + "\nchannel:" + strconv.Itoa(channelID)
}

func channelAffinityNegative(cacheKey string, channelID int) bool {
	key := channelAffinityNegativeKey(cacheKey, channelID)
	if key == "" {
		return false
	}
	value, found, err := getChannelAffinityNegativeCache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity negative cache get failed: err=%v", err))
		return false
	}
	return found && value == 1
}

func channelAffinityNegativeForRecord(cacheKey string, record ChannelAffinityRecord) bool {
	if record.ChannelID <= 0 {
		return false
	}
	key := channelAffinityNegativeKey(cacheKey, record.ChannelID)
	if key == "" {
		return false
	}
	negative, found, err := getChannelAffinityNegativeV2Cache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity negative v2 cache get failed: err=%v", err))
		return false
	}
	if found {
		return negative.FailedGeneration == record.Generation && negative.FailedAt >= record.RecordedAt
	}
	return channelAffinityNegative(cacheKey, record.ChannelID)
}

func intEqual(a, b int) bool { return a == b }

func currentChannelAffinityKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(ginKeyChannelAffinityMatchedKey); ok {
		if key, ok := value.(string); ok && strings.TrimSpace(key) != "" {
			return channelAffinityRawCacheKey(key)
		}
	}
	if meta, ok := getChannelAffinityMeta(c); ok {
		return channelAffinityRawCacheKey(meta.CacheKey)
	}
	return ""
}

func ShouldAvoidChannelForSession(c *gin.Context, channelID int) bool {
	key := currentChannelAffinityKey(c)
	record, found, err := getAffinityRecord(key)
	if err == nil && found && record.ChannelID == channelID {
		return channelAffinityNegativeForRecord(key, record)
	}
	return channelAffinityNegative(key, channelID)
}

func EvictCurrentChannelAffinityIfMatches(c *gin.Context, failedChannelID int) bool {
	if c == nil || failedChannelID <= 0 {
		return false
	}
	key := currentChannelAffinityKey(c)
	if key == "" {
		return false
	}
	expected, ok := matchedAffinityRecord(c)
	if !ok {
		if _, found, err := getChannelAffinityRecordCache().Get(key); err != nil || found {
			return false
		}
		deleted, err := getChannelAffinityCache().CompareAndDelete(key, failedChannelID, intEqual)
		if err != nil {
			common.SysError(fmt.Sprintf("legacy channel affinity compare-delete failed: err=%v", err))
		}
		return deleted
	}
	if expected.ChannelID != failedChannelID {
		return false
	}
	deleted, err := getChannelAffinityRecordCache().CompareAndDelete(key, expected, affinityRecordEqual)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity compare-delete failed: err=%v", err))
		return false
	}
	if deleted && common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		index := common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		if index >= 0 {
			if _, err := getChannelAffinityMultiKeyIndexCache().CompareAndDelete(key, index, intEqual); err != nil {
				common.SysError(fmt.Sprintf("channel affinity multi-key compare-delete failed: err=%v", err))
			}
		}
	}
	if deleted {
		_, _ = getChannelAffinityCache().CompareAndDelete(key, failedChannelID, intEqual)
	}
	return deleted
}

func MarkChannelSessionNegative(c *gin.Context, channelID int) {
	MarkChannelSessionNegativeWithTTL(c, channelID, 0)
}

func MarkChannelSessionNegativeWithTTL(c *gin.Context, channelID int, ttl time.Duration) bool {
	key := channelAffinityNegativeKey(currentChannelAffinityKey(c), channelID)
	if key == "" {
		return false
	}
	ttlSeconds := 30
	if setting := operation_setting.GetStreamRecoverySetting(); setting != nil {
		if setting.SessionNegativeTTLSeconds <= 0 {
			return false
		}
		ttlSeconds = setting.SessionNegativeTTLSeconds
	}
	if ttl > 0 {
		ttlSeconds = int(ttl.Seconds())
	}
	if record, ok := matchedAffinityRecord(c); ok && record.ChannelID == channelID {
		negative := channelSessionNegative{FailedGeneration: record.Generation, FailedAt: time.Now().UnixNano()}
		if err := getChannelAffinityNegativeV2Cache().SetWithTTL(key, negative, time.Duration(ttlSeconds)*time.Second); err != nil {
			common.SysError(fmt.Sprintf("channel affinity negative v2 cache set failed: err=%v", err))
			return false
		}
		if err := getChannelAffinityNegativeCache().SetWithTTL(key, 1, time.Duration(ttlSeconds)*time.Second); err != nil {
			common.SysError(fmt.Sprintf("channel affinity negative cache set failed: err=%v", err))
			return false
		}
		return true
	}
	if err := getChannelAffinityNegativeCache().SetWithTTL(key, 1, time.Duration(ttlSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("channel affinity negative cache set failed: err=%v", err))
		return false
	}
	return true
}

func channelAffinityKeyNegativeKey(c *gin.Context, channelID, keyIndex int) string {
	base := currentChannelAffinityKey(c)
	if base == "" || channelID <= 0 || keyIndex < 0 {
		return ""
	}
	return fmt.Sprintf("%s\nchannel:%d\nkey:%d", base, channelID, keyIndex)
}

func ShouldAvoidChannelKeyForSession(c *gin.Context, channelID, keyIndex int) bool {
	key := channelAffinityKeyNegativeKey(c, channelID, keyIndex)
	if key == "" {
		return false
	}
	value, found, err := getChannelAffinityKeyNegativeCache().Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity key negative get failed: err=%v", err))
		return false
	}
	return found && value == 1
}

func MarkChannelSessionKeyNegative(c *gin.Context, channelID, keyIndex int) bool {
	key := channelAffinityKeyNegativeKey(c, channelID, keyIndex)
	if key == "" {
		return false
	}
	ttlSeconds := 90
	if setting := operation_setting.GetStreamRecoverySetting(); setting != nil && setting.KeyNegativeTTLSeconds > 0 {
		ttlSeconds = setting.KeyNegativeTTLSeconds
	}
	if err := getChannelAffinityKeyNegativeCache().SetWithTTL(key, 1, time.Duration(ttlSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("channel affinity key negative set failed: err=%v", err))
		return false
	}
	return true
}

func clearChannelSessionNegative(c *gin.Context, channelID int) {
	key := channelAffinityNegativeKey(currentChannelAffinityKey(c), channelID)
	if key == "" {
		return
	}
	_, _ = getChannelAffinityNegativeCache().CompareAndDelete(key, 1, intEqual)
	if negative, found, _ := getChannelAffinityNegativeV2Cache().Get(key); found {
		_, _ = getChannelAffinityNegativeV2Cache().CompareAndDelete(key, negative, channelSessionNegativeEqual)
	}
}

func SetRelaySemanticSuccess(c *gin.Context, success bool) {
	if c != nil {
		c.Set(ginKeyRelaySemanticSuccess, success)
	}
}

func RelaySemanticSuccess(c *gin.Context) bool {
	return c != nil && c.GetBool(ginKeyRelaySemanticSuccess)
}

func getChannelAffinityPreferredMultiKeyIndex(cacheKey string) (int, bool) {
	cacheKey = channelAffinityRawCacheKey(cacheKey)
	if cacheKey == "" {
		return 0, false
	}
	cache := getChannelAffinityMultiKeyIndexCache()
	index, found, err := cache.Get(cacheKey)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity multi-key index cache get failed: key=%s, err=%v", cacheKey, err))
		return 0, false
	}
	if !found || index < 0 {
		return 0, false
	}
	return index, true
}

func ResolveChannelAffinityMultiKey(c *gin.Context, channel *model.Channel) (string, int, bool, *types.NewAPIError) {
	if c == nil || channel == nil || !channel.ChannelInfo.IsMultiKey {
		return "", 0, false, nil
	}
	if preferredChannelID := common.GetContextKeyInt(c, constant.ContextKeyChannelPreferredMultiKeyChannelId); preferredChannelID == channel.Id {
		if rawIndex, ok := common.GetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyIndex); ok {
			if preferredIndex, ok := rawIndex.(int); ok && preferredIndex >= 0 {
				key, index, used, err := channel.GetEnabledKeyByIndex(preferredIndex)
				if err != nil || (used && !ShouldAvoidChannelKeyForSession(c, channel.Id, index)) {
					return key, index, used, err
				}
			}
		}
	}

	meta, ok := getChannelAffinityMeta(c)
	if !ok {
		return "", 0, false, nil
	}
	seed := strings.TrimSpace(meta.CacheKey)
	if seed == "" {
		seed = strings.TrimSpace(meta.KeyFingerprint)
	}
	if seed == "" {
		return "", 0, false, nil
	}
	key, index, used, err := channel.GetStableEnabledKey(seed)
	if err != nil || !used || !ShouldAvoidChannelKeyForSession(c, channel.Id, index) {
		return key, index, used, err
	}
	keys := channel.GetKeys()
	for offset := 1; offset < len(keys); offset++ {
		candidate := (index + offset) % len(keys)
		key, selected, enabled, keyErr := channel.GetEnabledKeyByIndex(candidate)
		if keyErr != nil {
			return "", 0, false, keyErr
		}
		if enabled && !ShouldAvoidChannelKeyForSession(c, channel.Id, selected) {
			return key, selected, true, nil
		}
	}
	return "", 0, false, types.NewError(fmt.Errorf("all enabled keys are temporarily unavailable for this session"), types.ErrorCodeChannelNoAvailableKey)
}

func ShouldSkipRetryAfterChannelAffinityFailure(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(ginKeyChannelAffinitySkipRetry)
	if ok {
		b, ok := v.(bool)
		if ok {
			return b
		}
	}
	meta, ok := getChannelAffinityMeta(c)
	if !ok {
		return false
	}
	return meta.SkipRetry
}

func MarkChannelAffinityUsed(c *gin.Context, selectedGroup string, channelID int) {
	if c == nil || channelID <= 0 {
		return
	}
	meta, ok := getChannelAffinityMeta(c)
	if !ok {
		return
	}
	c.Set(ginKeyChannelAffinitySkipRetry, meta.SkipRetry)
	info := map[string]interface{}{
		"reason":         meta.RuleName,
		"rule_name":      meta.RuleName,
		"using_group":    meta.UsingGroup,
		"selected_group": selectedGroup,
		"model":          meta.ModelName,
		"request_path":   meta.RequestPath,
		"channel_id":     channelID,
		"key_source":     meta.KeySourceType,
		"key_key":        meta.KeySourceKey,
		"key_path":       meta.KeySourcePath,
		"key_hint":       meta.KeyHint,
		"key_fp":         meta.KeyFingerprint,
	}
	if preferredChannelID := common.GetContextKeyInt(c, constant.ContextKeyChannelPreferredMultiKeyChannelId); preferredChannelID == channelID {
		if rawIndex, ok := common.GetContextKey(c, constant.ContextKeyChannelPreferredMultiKeyIndex); ok {
			if preferredIndex, ok := rawIndex.(int); ok && preferredIndex >= 0 {
				info["preferred_multi_key_index"] = preferredIndex
			}
		}
	}
	c.Set(ginKeyChannelAffinityLogInfo, info)
}

func AppendChannelAffinityAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	anyInfo, ok := c.Get(ginKeyChannelAffinityLogInfo)
	if !ok || anyInfo == nil {
		return
	}
	adminInfo["channel_affinity"] = anyInfo
}

func RecordChannelAffinity(c *gin.Context, channelID int) {
	if channelID <= 0 {
		return
	}
	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil || !setting.Enabled {
		return
	}
	if setting.SwitchOnSuccess && c != nil {
		if successChannelID := c.GetInt("channel_id"); successChannelID > 0 {
			channelID = successChannelID
		}
	}
	cacheKey, ttlSeconds, ok := getChannelAffinityContext(c)
	if !ok {
		return
	}
	if ttlSeconds <= 0 {
		ttlSeconds = setting.DefaultTTLSeconds
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	cache := getChannelAffinityCache()
	record := ChannelAffinityRecord{ChannelID: channelID, Generation: common.GetUUID(), RecordedAt: time.Now().UnixNano()}
	if err := getChannelAffinityRecordCache().SetWithTTL(cacheKey, record, time.Duration(ttlSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("channel affinity v2 cache set failed: key=%s, err=%v", cacheKey, err))
	}
	if err := cache.SetWithTTL(cacheKey, channelID, time.Duration(ttlSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("channel affinity cache set failed: key=%s, err=%v", cacheKey, err))
	}
	clearChannelSessionNegative(c, channelID)
	if chain, found, _ := getChannelAffinityRecoveryChainCache().Get(cacheKey); found {
		_, _ = getChannelAffinityRecoveryChainCache().CompareAndDelete(cacheKey, chain, sessionRecoveryChainEqual)
	}
	if common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		index := common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		if index >= 0 {
			if key := channelAffinityKeyNegativeKey(c, channelID, index); key != "" {
				_, _ = getChannelAffinityKeyNegativeCache().CompareAndDelete(key, 1, intEqual)
			}
			multiKeyCacheKey := channelAffinityRawCacheKey(cacheKey)
			if err := getChannelAffinityMultiKeyIndexCache().SetWithTTL(multiKeyCacheKey, index, time.Duration(ttlSeconds)*time.Second); err != nil {
				common.SysError(fmt.Sprintf("channel affinity multi-key index cache set failed: key=%s, err=%v", multiKeyCacheKey, err))
			}
		}
	}
}

type ChannelAffinityUsageCacheStats struct {
	RuleName            string `json:"rule_name"`
	UsingGroup          string `json:"using_group"`
	KeyFingerprint      string `json:"key_fp"`
	CachedTokenRateMode string `json:"cached_token_rate_mode"`

	Hit           int64 `json:"hit"`
	Total         int64 `json:"total"`
	WindowSeconds int64 `json:"window_seconds"`

	PromptTokens         int64 `json:"prompt_tokens"`
	CompletionTokens     int64 `json:"completion_tokens"`
	TotalTokens          int64 `json:"total_tokens"`
	CachedTokens         int64 `json:"cached_tokens"`
	PromptCacheHitTokens int64 `json:"prompt_cache_hit_tokens"`
	LastSeenAt           int64 `json:"last_seen_at"`
}

type ChannelAffinityUsageCacheCounters struct {
	CachedTokenRateMode string `json:"cached_token_rate_mode"`

	Hit           int64 `json:"hit"`
	Total         int64 `json:"total"`
	WindowSeconds int64 `json:"window_seconds"`

	PromptTokens         int64 `json:"prompt_tokens"`
	CompletionTokens     int64 `json:"completion_tokens"`
	TotalTokens          int64 `json:"total_tokens"`
	CachedTokens         int64 `json:"cached_tokens"`
	PromptCacheHitTokens int64 `json:"prompt_cache_hit_tokens"`
	LastSeenAt           int64 `json:"last_seen_at"`
}

var channelAffinityUsageCacheStatsLocks [64]sync.Mutex

// ObserveChannelAffinityUsageCacheByRelayFormat records usage cache stats with a stable rate mode derived from relay format.
func ObserveChannelAffinityUsageCacheByRelayFormat(c *gin.Context, usage *dto.Usage, relayFormat types.RelayFormat) {
	ObserveChannelAffinityUsageCacheFromContext(c, usage, cachedTokenRateModeByRelayFormat(relayFormat))
}

func ObserveChannelAffinityUsageCacheFromContext(c *gin.Context, usage *dto.Usage, cachedTokenRateMode string) {
	statsCtx, ok := GetChannelAffinityStatsContext(c)
	if !ok {
		return
	}
	observeChannelAffinityUsageCache(statsCtx, usage, cachedTokenRateMode)
}

func GetChannelAffinityUsageCacheStats(ruleName, usingGroup, keyFp string) ChannelAffinityUsageCacheStats {
	ruleName = strings.TrimSpace(ruleName)
	usingGroup = strings.TrimSpace(usingGroup)
	keyFp = strings.TrimSpace(keyFp)

	entryKey := channelAffinityUsageCacheEntryKey(ruleName, usingGroup, keyFp)
	if entryKey == "" {
		return ChannelAffinityUsageCacheStats{
			RuleName:       ruleName,
			UsingGroup:     usingGroup,
			KeyFingerprint: keyFp,
		}
	}

	cache := getChannelAffinityUsageCacheStatsCache()
	v, found, err := cache.Get(entryKey)
	if err != nil || !found {
		return ChannelAffinityUsageCacheStats{
			RuleName:       ruleName,
			UsingGroup:     usingGroup,
			KeyFingerprint: keyFp,
		}
	}
	return ChannelAffinityUsageCacheStats{
		CachedTokenRateMode:  v.CachedTokenRateMode,
		RuleName:             ruleName,
		UsingGroup:           usingGroup,
		KeyFingerprint:       keyFp,
		Hit:                  v.Hit,
		Total:                v.Total,
		WindowSeconds:        v.WindowSeconds,
		PromptTokens:         v.PromptTokens,
		CompletionTokens:     v.CompletionTokens,
		TotalTokens:          v.TotalTokens,
		CachedTokens:         v.CachedTokens,
		PromptCacheHitTokens: v.PromptCacheHitTokens,
		LastSeenAt:           v.LastSeenAt,
	}
}

func observeChannelAffinityUsageCache(statsCtx ChannelAffinityStatsContext, usage *dto.Usage, cachedTokenRateMode string) {
	entryKey := channelAffinityUsageCacheEntryKey(statsCtx.RuleName, statsCtx.UsingGroup, statsCtx.KeyFingerprint)
	if entryKey == "" {
		return
	}

	windowSeconds := statsCtx.TTLSeconds
	if windowSeconds <= 0 {
		return
	}

	cache := getChannelAffinityUsageCacheStatsCache()
	ttl := time.Duration(windowSeconds) * time.Second

	lock := channelAffinityUsageCacheStatsLock(entryKey)
	lock.Lock()
	defer lock.Unlock()

	prev, found, err := cache.Get(entryKey)
	if err != nil {
		return
	}
	next := prev
	if !found {
		next = ChannelAffinityUsageCacheCounters{}
	}
	currentMode := normalizeCachedTokenRateMode(cachedTokenRateMode)
	if currentMode != "" {
		if next.CachedTokenRateMode == "" {
			next.CachedTokenRateMode = currentMode
		} else if next.CachedTokenRateMode != currentMode && next.CachedTokenRateMode != cacheTokenRateModeMixed {
			next.CachedTokenRateMode = cacheTokenRateModeMixed
		}
	}
	next.Total++
	hit, cachedTokens, promptCacheHitTokens := usageCacheSignals(usage)
	if hit {
		next.Hit++
	}
	next.WindowSeconds = windowSeconds
	next.LastSeenAt = time.Now().Unix()
	next.CachedTokens += cachedTokens
	next.PromptCacheHitTokens += promptCacheHitTokens
	next.PromptTokens += int64(usagePromptTokens(usage))
	next.CompletionTokens += int64(usageCompletionTokens(usage))
	next.TotalTokens += int64(usageTotalTokens(usage))
	_ = cache.SetWithTTL(entryKey, next, ttl)
}

func normalizeCachedTokenRateMode(mode string) string {
	switch mode {
	case cacheTokenRateModeCachedOverPrompt:
		return cacheTokenRateModeCachedOverPrompt
	case cacheTokenRateModeCachedOverPromptPlusCached:
		return cacheTokenRateModeCachedOverPromptPlusCached
	case cacheTokenRateModeMixed:
		return cacheTokenRateModeMixed
	default:
		return ""
	}
}

func cachedTokenRateModeByRelayFormat(relayFormat types.RelayFormat) string {
	switch relayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		return cacheTokenRateModeCachedOverPrompt
	case types.RelayFormatClaude:
		return cacheTokenRateModeCachedOverPromptPlusCached
	default:
		return ""
	}
}

func channelAffinityUsageCacheEntryKey(ruleName, usingGroup, keyFp string) string {
	ruleName = strings.TrimSpace(ruleName)
	usingGroup = strings.TrimSpace(usingGroup)
	keyFp = strings.TrimSpace(keyFp)
	if ruleName == "" || keyFp == "" {
		return ""
	}
	return ruleName + "\n" + usingGroup + "\n" + keyFp
}

func usageCacheSignals(usage *dto.Usage) (hit bool, cachedTokens int64, promptCacheHitTokens int64) {
	if usage == nil {
		return false, 0, 0
	}

	cached := int64(0)
	if usage.PromptTokensDetails.CachedTokens > 0 {
		cached = int64(usage.PromptTokensDetails.CachedTokens)
	} else if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
		cached = int64(usage.InputTokensDetails.CachedTokens)
	}
	pcht := int64(0)
	if usage.PromptCacheHitTokens > 0 {
		pcht = int64(usage.PromptCacheHitTokens)
	}
	return cached > 0 || pcht > 0, cached, pcht
}

func usagePromptTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.PromptTokens > 0 {
		return usage.PromptTokens
	}
	return usage.InputTokens
}

func usageCompletionTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.CompletionTokens > 0 {
		return usage.CompletionTokens
	}
	return usage.OutputTokens
}

func usageTotalTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	pt := usagePromptTokens(usage)
	ct := usageCompletionTokens(usage)
	if pt > 0 || ct > 0 {
		return pt + ct
	}
	return 0
}

func getChannelAffinityUsageCacheStatsCache() *cachex.HybridCache[ChannelAffinityUsageCacheCounters] {
	channelAffinityUsageCacheStatsOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}

		channelAffinityUsageCacheStatsCache = cachex.NewHybridCache[ChannelAffinityUsageCacheCounters](cachex.HybridCacheConfig[ChannelAffinityUsageCacheCounters]{
			Namespace: cachex.Namespace(channelAffinityUsageCacheStatsNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ChannelAffinityUsageCacheCounters]{},
			Memory: func() *hot.HotCache[string, ChannelAffinityUsageCacheCounters] {
				return hot.NewHotCache[string, ChannelAffinityUsageCacheCounters](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityUsageCacheStatsCache
}

func channelAffinityUsageCacheStatsLock(key string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := h.Sum32() % uint32(len(channelAffinityUsageCacheStatsLocks))
	return &channelAffinityUsageCacheStatsLocks[idx]
}
