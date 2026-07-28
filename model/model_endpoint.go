package model

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// ModelEndpoint stores an optional per-(channel, model) upstream endpoint and
// protocol override. It lets a single channel expose models that live behind
// different hosts and speak different upstream protocols (for example a channel
// that serves both claude-* via Anthropic and gpt-* via OpenAI) without forcing
// every model onto the channel's single Type.
//
// The routing layer always lets an exact per-(ChannelId, Model) row override the
// upstream protocol. Global model defaults are applied only when the channel
// explicitly opts into model-based protocol routing, so native-protocol channels
// keep their own authentication and request format by default.
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
	modelEndpointCache     map[int]map[string]*ModelEndpoint
	modelEndpointCacheLock sync.RWMutex
	modelEndpointSyncOnce  sync.Once
)

func loadModelEndpointsFromDB() (map[int]map[string]*ModelEndpoint, error) {
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
func ReloadModelEndpointCacheWithError() error {
	grouped, err := loadModelEndpointsFromDB()
	if err != nil {
		return err
	}
	modelEndpointCacheLock.Lock()
	modelEndpointCache = grouped
	modelEndpointCacheLock.Unlock()
	return nil
}

func ReloadModelEndpointCache() {
	if err := ReloadModelEndpointCacheWithError(); err != nil {
		common.SysError("failed to reload model endpoint cache: " + err.Error())
	}
}

// ensureModelEndpointCache lazily performs the first load and starts a light
// periodic refresh so multi-node deployments converge without wiring into the
// channel cache bootstrap. Writes on the local node refresh immediately via
// ReloadModelEndpointCache, so this only backstops cross-node staleness.
func ensureModelEndpointCache() {
	modelEndpointSyncOnce.Do(func() {
		ReloadModelEndpointCache()
	})
}

func RunModelEndpointCacheSync(ctx context.Context) {
	ensureModelEndpointCache()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ReloadModelEndpointCache()
		}
	}
}

// GetModelEndpoint returns the override row for (channelId, modelName) or nil.
// It mirrors the channel cache semantics: when the memory cache is disabled it
// reads straight from the database.
func GetModelEndpoint(channelId int, modelName string) *ModelEndpoint {
	if channelId <= 0 || modelName == "" {
		return nil
	}
	if !common.MemoryCacheEnabled {
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

// ChannelAllowsModelProtocolOverride reports whether the channel opted into
// global model-name based protocol/adaptor overrides. Exact per-model endpoint
// rows remain explicit admin overrides and do not require this flag.
func ChannelAllowsModelProtocolOverride(channel *Channel) bool {
	if channel == nil {
		return false
	}
	if channel.Type == constant.ChannelTypeCodex {
		return false
	}
	return channel.GetSetting().AllowModelProtocolOverride
}

type ModelRouteSource string

const (
	ModelRouteSourceNative   ModelRouteSource = "native"
	ModelRouteSourceExplicit ModelRouteSource = "explicit"
	ModelRouteSourceGlobal   ModelRouteSource = "global"
)

// ModelRouteDecision is the single source of truth for the effective upstream
// adaptor and endpoint selected for a channel/model pair.
type ModelRouteDecision struct {
	ChannelType    int                   `json:"channel_type"`
	BaseURL        string                `json:"base_url"`
	Endpoint       constant.EndpointType `json:"endpoint"`
	Source         ModelRouteSource      `json:"source"`
	MatchedType    string                `json:"matched_type,omitempty"`
	MatchedPattern string                `json:"matched_pattern,omitempty"`
	Overridden     bool                  `json:"overridden"`
	Reason         string                `json:"reason"`
}

func EndpointTypeForChannelType(channelType int) constant.EndpointType {
	switch channelType {
	case constant.ChannelTypeAnthropic:
		return constant.EndpointTypeAnthropic
	case constant.ChannelTypeGemini:
		return constant.EndpointTypeGemini
	case constant.ChannelTypeCodex:
		return constant.EndpointTypeOpenAIResponse
	default:
		return constant.EndpointTypeOpenAI
	}
}

func ChannelAllowsModelProtocolOverrideTarget(channel *Channel, target constant.EndpointType) bool {
	if !ChannelAllowsModelProtocolOverride(channel) || target == "" {
		return false
	}
	for _, configured := range channel.GetSetting().ModelProtocolOverrideTargets {
		if constant.EndpointType(strings.TrimSpace(configured)) == target {
			return true
		}
	}
	return false
}

func globalRouteProfile(channel *Channel, modelName string) (operation_setting.ModelEndpointRouteProfile, bool) {
	profile, ok := operation_setting.ResolveModelDefaultProfile(modelName)
	if !ok {
		return operation_setting.ModelEndpointRouteProfile{}, false
	}
	target := constant.EndpointType(strings.TrimSpace(profile.DefaultEndpoint))
	if !ChannelAllowsModelProtocolOverrideTarget(channel, target) {
		return operation_setting.ModelEndpointRouteProfile{}, false
	}
	return profile, true
}

// ResolveModelRouteDecision applies explicit per-model rows first, then an
// allowlisted global model rule, and finally the channel's native protocol.
func ResolveModelRouteDecision(channel *Channel, modelName string) ModelRouteDecision {
	if channel == nil {
		return ModelRouteDecision{Source: ModelRouteSourceNative, Reason: "channel is nil"}
	}
	decision := ModelRouteDecision{
		ChannelType: channel.Type,
		BaseURL:     channel.GetBaseURL(),
		Endpoint:    EndpointTypeForChannelType(channel.Type),
		Source:      ModelRouteSourceNative,
		Reason:      "channel native protocol",
	}

	ep := GetModelEndpoint(channel.Id, modelName)
	if ep == nil {
		if profile, ok := globalRouteProfile(channel, modelName); ok {
			decision.ChannelType = profile.ChannelType
			decision.Endpoint = constant.EndpointType(profile.DefaultEndpoint)
			decision.Source = ModelRouteSourceGlobal
			decision.MatchedType = profile.MatchType
			decision.MatchedPattern = profile.Pattern
			decision.Overridden = true
			decision.Reason = "allowlisted global model rule"
		}
		return decision
	}

	decision.Source = ModelRouteSourceExplicit
	decision.Overridden = true
	decision.Reason = "explicit channel model endpoint"
	protocolFromGlobal := false
	if ep.ChannelType != nil {
		decision.ChannelType = *ep.ChannelType
		decision.Endpoint = EndpointTypeForChannelType(decision.ChannelType)
	} else if profile, ok := globalRouteProfile(channel, modelName); ok {
		decision.ChannelType = profile.ChannelType
		decision.Endpoint = constant.EndpointType(profile.DefaultEndpoint)
		decision.MatchedType = profile.MatchType
		decision.MatchedPattern = profile.Pattern
		protocolFromGlobal = true
	} else if inferred, ok := InferChannelTypeFromModel(modelName); ok {
		decision.ChannelType = inferred
		decision.Endpoint = EndpointTypeForChannelType(inferred)
	}

	if strings.TrimSpace(ep.BaseURL) != "" {
		decision.BaseURL = strings.TrimSpace(ep.BaseURL)
	} else if decision.ChannelType != channel.Type && !protocolFromGlobal {
		if decision.ChannelType >= 0 && decision.ChannelType < len(constant.ChannelBaseURLs) {
			if official := constant.ChannelBaseURLs[decision.ChannelType]; official != "" {
				decision.BaseURL = official
			}
		}
	}
	return decision
}

// ResolveModelRoute returns the effective upstream channel type and base URL for
// the given channel + model. When no per-model override applies and the channel
// has not opted into global protocol routing, it returns the channel's own type
// and base URL unchanged with overridden == false.
//
// Resolution order:
//  1. Per-channel per-model row (model_endpoints): an explicit ChannelType wins;
//     an "auto" row consults the global default, then name inference.
//  2. When no row exists and the channel opted into model protocol routing, the
//     global model-endpoint default registry decides the protocol. This layer is
//     protocol-only and never rewrites the base URL, so aggregator/relay
//     channels keep their own host.
//  3. Otherwise the channel's own type and base URL are used unchanged.
func ResolveModelRoute(channel *Channel, modelName string) (channelType int, baseURL string, overridden bool) {
	decision := ResolveModelRouteDecision(channel, modelName)
	return decision.ChannelType, decision.BaseURL, decision.Overridden
}

// GetChannelModelEndpoints returns all override rows for a channel, ordered by
// model name for stable rendering in the admin UI.
func GetChannelModelEndpoints(channelId int) ([]*ModelEndpoint, error) {
	return GetChannelModelEndpointsTx(DB, channelId)
}

func GetChannelModelEndpointsTx(tx *gorm.DB, channelId int) ([]*ModelEndpoint, error) {
	endpoints := make([]*ModelEndpoint, 0)
	if tx == nil {
		return endpoints, errors.New("transaction is required")
	}
	if channelId <= 0 {
		return endpoints, nil
	}
	err := tx.Where("channel_id = ?", channelId).Order("model asc").Find(&endpoints).Error
	return endpoints, err
}

// ReplaceChannelModelEndpoints atomically replaces the override set for a
// channel. Rows with an empty model name or duplicate model are dropped so the
// unique index is never violated.
func ReplaceChannelModelEndpoints(channelId int, endpoints []*ModelEndpoint) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ReplaceChannelModelEndpointsTx(tx, channelId, endpoints)
	})
	if err != nil {
		return err
	}
	ReloadModelEndpointCache()
	return nil
}

func ReplaceChannelModelEndpointsTx(tx *gorm.DB, channelId int, endpoints []*ModelEndpoint) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	if err := tx.Where("channel_id = ?", channelId).Delete(&ModelEndpoint{}).Error; err != nil {
		return err
	}
	now := time.Now().Unix()
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
		cleaned = append(cleaned, &ModelEndpoint{ChannelId: channelId, Model: modelName,
			BaseURL: strings.TrimSpace(ep.BaseURL), ChannelType: ep.ChannelType,
			CreatedTime: now, UpdatedTime: now})
	}
	if len(cleaned) == 0 {
		return nil
	}
	return tx.Create(&cleaned).Error
}

// DeleteChannelModelEndpoints removes all override rows for a channel. It is
// invoked when a channel is deleted so no orphan rows survive.
func DeleteChannelModelEndpoints(channelId int) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	if err := DB.Where("channel_id = ?", channelId).Delete(&ModelEndpoint{}).Error; err != nil {
		return err
	}
	ReloadModelEndpointCache()
	return nil
}
