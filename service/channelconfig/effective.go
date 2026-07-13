package channelconfig

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	runtimeservice "github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/adminconfig"
	"github.com/QuantumNous/new-api/types"
)

type EffectiveConfigSnapshot struct {
	GeneratedAt   int64                                    `json:"generated_at"`
	ChannelID     int                                      `json:"channel_id"`
	ConfigVersion int64                                    `json:"config_version"`
	PersistedOnly bool                                     `json:"persisted_only"`
	Model         string                                   `json:"model,omitempty"`
	Route         *model.ModelRouteDecision                `json:"route,omitempty"`
	Capability    *runtimeservice.ProtocolBridgeCapability `json:"capability,omitempty"`
	Items         []adminconfig.EffectiveValue             `json:"items"`
}

func effectiveLayer(source adminconfig.Source, sourceID string, value any, applicable bool, present bool) adminconfig.Layer {
	return adminconfig.Layer{
		Source: source, SourceID: sourceID, Applicable: applicable, Present: present, Value: value,
	}
}

func channelOnlyValue(channel *model.Channel, key string, value any, present bool, masked bool) adminconfig.EffectiveValue {
	return adminconfig.Resolve(key, masked,
		effectiveLayer(adminconfig.SourceGlobal, "", nil, false, false),
		effectiveLayer(adminconfig.SourceGroup, channel.Group, nil, false, false),
		effectiveLayer(adminconfig.SourceChannel, strconv.Itoa(channel.Id), value, true, present),
		effectiveLayer(adminconfig.SourceRequest, "", nil, false, false),
	)
}

func effectiveBaseURL(channel *model.Channel, modelName string, route *model.ModelRouteDecision) adminconfig.EffectiveValue {
	providerURL := ""
	if channel.Type >= 0 && channel.Type < len(constant.ChannelBaseURLs) {
		providerURL = strings.TrimSpace(constant.ChannelBaseURLs[channel.Type])
	}
	channelURL := ""
	if channel.BaseURL != nil {
		channelURL = strings.TrimSpace(*channel.BaseURL)
	}
	layers := []adminconfig.Layer{
		effectiveLayer(adminconfig.SourceProvider, constant.GetChannelTypeName(channel.Type), providerURL, true, providerURL != ""),
		effectiveLayer(adminconfig.SourceChannel, strconv.Itoa(channel.Id), channelURL, true, channelURL != ""),
	}
	if modelName != "" {
		value := ""
		present := route != nil && route.Source == model.ModelRouteSourceExplicit
		if present {
			value = route.BaseURL
		}
		layers = append(layers, effectiveLayer(adminconfig.SourceModelEndpoint, modelName, value, true, present))
	}
	return adminconfig.Resolve("base_url", false, layers...)
}

// BuildEffectiveConfigSnapshot explains persisted configuration by reusing
// runtime route and protocol functions. It never probes an upstream.
func BuildEffectiveConfigSnapshot(channel *model.Channel, modelName string, clientFormat types.RelayFormat) EffectiveConfigSnapshot {
	modelName = strings.TrimSpace(modelName)
	var route *model.ModelRouteDecision
	var capability *runtimeservice.ProtocolBridgeCapability
	if modelName != "" {
		decision := model.ResolveModelRouteDecision(channel, modelName)
		route = &decision
		resolvedCapability := runtimeservice.EvaluateChannelProtocolCapability(channel, modelName, clientFormat, nil)
		capability = &resolvedCapability
	}

	return EffectiveConfigSnapshot{
		GeneratedAt:   time.Now().Unix(),
		ChannelID:     channel.Id,
		ConfigVersion: channel.ConfigVersion,
		PersistedOnly: true,
		Model:         modelName,
		Route:         route,
		Capability:    capability,
		Items: []adminconfig.EffectiveValue{
			channelOnlyValue(channel, "status", channel.Status, true, false),
			effectiveBaseURL(channel, modelName, route),
			channelOnlyValue(channel, "group", channel.Group, strings.TrimSpace(channel.Group) != "", false),
			channelOnlyValue(channel, "priority", channel.Priority, channel.Priority != nil, false),
			channelOnlyValue(channel, "weight", channel.Weight, channel.Weight != nil, false),
			channelOnlyValue(channel, "models", channel.Models, strings.TrimSpace(channel.Models) != "", false),
			channelOnlyValue(channel, "model_mapping", nil, channel.ModelMapping != nil && strings.TrimSpace(*channel.ModelMapping) != "", true),
			channelOnlyValue(channel, "key", nil, strings.TrimSpace(channel.Key) != "", true),
		},
	}
}
