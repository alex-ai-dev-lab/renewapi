package model

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

// ModelEndpoint stores an optional per-(channel, model) upstream endpoint and
// protocol override. It lets a single channel expose models that live behind
// different hosts and speak different upstream protocols (for example a channel
// that serves both claude-* via Anthropic and gpt-* via OpenAI) without forcing
// every model onto the channel's single Type.
//
// The routing layer only consults this table when a row exists for the exact
// (ChannelId, Model) pair of a request. Channels/models without a row keep the
// original channel-type based routing byte-for-byte, so the feature is strictly
// opt-in and backward compatible.
type ModelEndpoint struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	ChannelId int    `json:"channel_id" gorm:"uniqueIndex:idx_model_endpoint_channel_model,priority:1;not null"`
	Model     string `json:"model" gorm:"uniqueIndex:idx_model_endpoint_channel_model,priority:2;type:varchar(255);not null"`
	// BaseURL overrides the upstream base URL for this model. Empty means
	// "inherit": use the official base URL of the resolved channel type when a
	// protocol override applies, otherwise the channel's own base URL.
	BaseURL string `json:"base_url" gorm:"column:base_url;type:varchar(512);not null;default:''"`
	// ChannelType optionally overrides the upstream protocol/adaptor for this
	// model. A nil value means "auto": infer from the model name and fall back to
	// the channel's own type when no confident match is found.
	ChannelType *int  `json:"channel_type" gorm:"column:channel_type"`
	CreatedTime int64 `json:"created_time" gorm:"bigint"`
	UpdatedTime int64 `json:"updated_time" gorm:"bigint"`
}

func (ModelEndpoint) TableName() string {
	return "model_endpoints"
}

var (
	modelEndpointCache      map[int]map[string]*ModelEndpoint
	modelEndpointCacheLock  sync.RWMutex
	modelEndpointSyncOnce   sync.Once
	modelEndpointSchemaOnce sync.Once
)

// ensureModelEndpointSchema creates or upgrades the model_endpoints table on
// first use. This keeps the feature self-contained rather than editing the
// shared AutoMigrate lists, and is safe to call from any code path because
// AutoMigrate is idempotent and DB is initialized long before requests run.
func ensureModelEndpointSchema() {
	modelEndpointSchemaOnce.Do(func() {
		if DB == nil {
			return
		}
		if err := DB.AutoMigrate(&ModelEndpoint{}); err != nil {
			common.SysError("failed to auto-migrate model_endpoints table: " + err.Error())
		}
	})
}

func loadModelEndpointsFromDB() (map[int]map[string]*ModelEndpoint, error) {
	ensureModelEndpointSchema()
	var endpoints []*ModelEndpoint
	if err := DB.Find(&endpoints).Error; err != nil {
		return nil, err
	}
	grouped := make(map[int]map[string]*ModelEndpoint)
	for _, ep := range endpoints {
		if ep.ChannelId <= 0 || strings.TrimSpace(ep.Model) == "" {
			continue
		}
		if grouped[ep.ChannelId] == nil {
			grouped[ep.ChannelId] = make(map[string]*ModelEndpoint)
		}
		grouped[ep.ChannelId][ep.Model] = ep
	}
	return grouped, nil
}

// ReloadModelEndpointCache rebuilds the in-memory cache from the database. It is
// safe to call from request handlers after a write and from background sync.
func ReloadModelEndpointCache() {
	grouped, err := loadModelEndpointsFromDB()
	if err != nil {
		common.SysError("failed to reload model endpoint cache: " + err.Error())
		return
	}
	modelEndpointCacheLock.Lock()
	modelEndpointCache = grouped
	modelEndpointCacheLock.Unlock()
}

// ensureModelEndpointCache lazily performs the first load and starts a light
// periodic refresh so multi-node deployments converge without wiring into the
// channel cache bootstrap. Writes on the local node refresh immediately via
// ReloadModelEndpointCache, so this only backstops cross-node staleness.
func ensureModelEndpointCache() {
	modelEndpointSyncOnce.Do(func() {
		ReloadModelEndpointCache()
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				ReloadModelEndpointCache()
			}
		}()
	})
}

// GetModelEndpoint returns the override row for (channelId, modelName) or nil.
// It mirrors the channel cache semantics: when the memory cache is disabled it
// reads straight from the database.
func GetModelEndpoint(channelId int, modelName string) *ModelEndpoint {
	if channelId <= 0 || modelName == "" {
		return nil
	}
	if !common.MemoryCacheEnabled {
		ensureModelEndpointSchema()
		var ep ModelEndpoint
		if err := DB.Where("channel_id = ? AND model = ?", channelId, modelName).First(&ep).Error; err != nil {
			return nil
		}
		return &ep
	}
	ensureModelEndpointCache()
	modelEndpointCacheLock.RLock()
	defer modelEndpointCacheLock.RUnlock()
	if byModel, ok := modelEndpointCache[channelId]; ok {
		if ep, ok := byModel[modelName]; ok {
			return ep
		}
	}
	return nil
}

// InferChannelTypeFromModel maps a model name to a likely upstream channel type
// using cheap prefix matching. The bool result reports whether a confident
// match was found; callers fall back to the channel's own type when it is false.
func InferChannelTypeFromModel(modelName string) (int, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return 0, false
	}
	switch {
	case strings.HasPrefix(name, "claude"):
		return constant.ChannelTypeAnthropic, true
	case strings.HasPrefix(name, "gpt"),
		strings.HasPrefix(name, "chatgpt"),
		strings.HasPrefix(name, "o1"),
		strings.HasPrefix(name, "o3"),
		strings.HasPrefix(name, "o4"),
		strings.HasPrefix(name, "text-embedding"),
		strings.HasPrefix(name, "dall-e"),
		strings.HasPrefix(name, "whisper"),
		strings.HasPrefix(name, "tts"):
		return constant.ChannelTypeOpenAI, true
	case strings.HasPrefix(name, "grok"):
		return constant.ChannelTypeXai, true
	case strings.HasPrefix(name, "gemini"), strings.HasPrefix(name, "gemma"):
		return constant.ChannelTypeGemini, true
	default:
		return 0, false
	}
}

// ResolveModelRoute returns the effective upstream channel type and base URL for
// the given channel + model. When no per-model override applies it returns the
// channel's own type and base URL unchanged with overridden == false, letting
// callers preserve the exact original behavior.
func ResolveModelRoute(channel *Channel, modelName string) (channelType int, baseURL string, overridden bool) {
	if channel == nil {
		return 0, "", false
	}
	channelType = channel.Type
	baseURL = channel.GetBaseURL()
	ep := GetModelEndpoint(channel.Id, modelName)
	if ep == nil {
		return channelType, baseURL, false
	}
	// Resolve protocol: explicit override wins, else infer, else keep channel type.
	if ep.ChannelType != nil {
		channelType = *ep.ChannelType
	} else if inferred, ok := InferChannelTypeFromModel(modelName); ok {
		channelType = inferred
	}
	// Resolve base URL: explicit override wins, else the official base URL of the
	// resolved type when the protocol changed, else the channel's own base URL.
	if strings.TrimSpace(ep.BaseURL) != "" {
		baseURL = strings.TrimSpace(ep.BaseURL)
	} else if channelType != channel.Type {
		if channelType >= 0 && channelType < len(constant.ChannelBaseURLs) {
			if official := constant.ChannelBaseURLs[channelType]; official != "" {
				baseURL = official
			}
		}
	}
	return channelType, baseURL, true
}

// GetChannelModelEndpoints returns all override rows for a channel, ordered by
// model name for stable rendering in the admin UI.
func GetChannelModelEndpoints(channelId int) ([]*ModelEndpoint, error) {
	var endpoints []*ModelEndpoint
	if channelId <= 0 {
		return endpoints, nil
	}
	ensureModelEndpointSchema()
	err := DB.Where("channel_id = ?", channelId).Order("model asc").Find(&endpoints).Error
	return endpoints, err
}

// ReplaceChannelModelEndpoints atomically replaces the override set for a
// channel. Rows with an empty model name or duplicate model are dropped so the
// unique index is never violated.
func ReplaceChannelModelEndpoints(channelId int, endpoints []*ModelEndpoint) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	ensureModelEndpointSchema()
	now := time.Now().Unix()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", channelId).Delete(&ModelEndpoint{}).Error; err != nil {
			return err
		}
		cleaned := make([]*ModelEndpoint, 0, len(endpoints))
		seen := make(map[string]bool)
		for _, ep := range endpoints {
			if ep == nil {
				continue
			}
			modelName := strings.TrimSpace(ep.Model)
			if modelName == "" || seen[modelName] {
				continue
			}
			seen[modelName] = true
			cleaned = append(cleaned, &ModelEndpoint{
				ChannelId:   channelId,
				Model:       modelName,
				BaseURL:     strings.TrimSpace(ep.BaseURL),
				ChannelType: ep.ChannelType,
				CreatedTime: now,
				UpdatedTime: now,
			})
		}
		if len(cleaned) > 0 {
			if err := tx.Create(&cleaned).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	ReloadModelEndpointCache()
	return nil
}

// DeleteChannelModelEndpoints removes all override rows for a channel. It is
// invoked when a channel is deleted so no orphan rows survive.
func DeleteChannelModelEndpoints(channelId int) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	ensureModelEndpointSchema()
	if err := DB.Where("channel_id = ?", channelId).Delete(&ModelEndpoint{}).Error; err != nil {
		return err
	}
	ReloadModelEndpointCache()
	return nil
}
