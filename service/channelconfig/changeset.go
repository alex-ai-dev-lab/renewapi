package channelconfig

import (
	"reflect"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type ChangeSet struct {
	ChangedFields    []string `json:"changed_fields"`
	KeyChanged       bool     `json:"key_changed"`
	AbilityChanged   bool     `json:"ability_changed"`
	TransportChanged bool     `json:"transport_changed"`
	RoutingChanged   bool     `json:"routing_changed"`
	ProtocolChanged  bool     `json:"protocol_changed"`
	EndpointsChanged bool     `json:"endpoints_changed"`
	EndpointCount    int      `json:"endpoint_count"`
}

func (c ChangeSet) Changed() bool {
	return c.KeyChanged || c.EndpointsChanged || len(c.ChangedFields) > 0
}

var abilityFields = map[string]bool{
	"status": true, "models": true, "group": true, "model_mapping": true,
	"priority": true, "weight": true, "tag": true,
}

var transportFields = map[string]bool{
	"type": true, "base_url": true, "other": true, "setting": true, "settings": true,
	"openai_organization": true, "header_override": true,
}

var routingFields = map[string]bool{
	"type": true, "status": true, "models": true, "group": true,
	"model_mapping": true, "priority": true, "weight": true, "setting": true,
}

var protocolFields = map[string]bool{
	"type": true, "setting": true, "settings": true,
}

var semanticJSONFields = map[string]bool{
	"model_mapping": true, "status_code_mapping": true, "other_info": true,
	"param_override": true, "header_override": true, "setting": true, "settings": true,
}

func BuildChangeSet(
	before *model.Channel,
	after *model.Channel,
	presentFields map[string]bool,
	updateKey bool,
	beforeEndpoints []*model.ModelEndpoint,
	requestedEndpoints *[]*model.ModelEndpoint,
) ChangeSet {
	changes := ChangeSet{}
	if before == nil || after == nil {
		return changes
	}

	for field := range presentFields {
		beforeValue, beforeOK := channelFieldValue(before, field)
		afterValue, afterOK := channelFieldValue(after, field)
		if !beforeOK || !afterOK || fieldValuesEqual(field, beforeValue, afterValue) {
			continue
		}
		changes.ChangedFields = append(changes.ChangedFields, field)
	}

	changes.KeyChanged = updateKey && before.Key != after.Key
	if requestedEndpoints != nil {
		changes.EndpointCount = len(*requestedEndpoints)
		changes.EndpointsChanged = !modelEndpointsEqual(beforeEndpoints, *requestedEndpoints)
		if changes.EndpointsChanged {
			changes.ChangedFields = append(changes.ChangedFields, "model_endpoints")
		}
	}

	sort.Strings(changes.ChangedFields)
	changes.ChangedFields = compactSortedStrings(changes.ChangedFields)
	for _, field := range changes.ChangedFields {
		changes.AbilityChanged = changes.AbilityChanged || abilityFields[field]
		changes.TransportChanged = changes.TransportChanged || transportFields[field]
		changes.RoutingChanged = changes.RoutingChanged || routingFields[field]
		changes.ProtocolChanged = changes.ProtocolChanged || protocolFields[field]
	}
	if changes.KeyChanged {
		changes.TransportChanged = true
	}
	if changes.EndpointsChanged {
		changes.TransportChanged = true
		changes.RoutingChanged = true
		changes.ProtocolChanged = true
	}
	return changes
}

func channelFieldValue(channel *model.Channel, field string) (any, bool) {
	switch field {
	case "type":
		return channel.Type, true
	case "openai_organization":
		return channel.OpenAIOrganization, true
	case "test_model":
		return channel.TestModel, true
	case "status":
		return channel.Status, true
	case "name":
		return channel.Name, true
	case "weight":
		return channel.Weight, true
	case "base_url":
		return channel.BaseURL, true
	case "other":
		return channel.Other, true
	case "balance":
		return channel.Balance, true
	case "balance_updated_time":
		return channel.BalanceUpdatedTime, true
	case "models":
		return channel.Models, true
	case "group":
		return channel.Group, true
	case "model_mapping":
		return channel.ModelMapping, true
	case "status_code_mapping":
		return channel.StatusCodeMapping, true
	case "priority":
		return channel.Priority, true
	case "auto_ban":
		return channel.AutoBan, true
	case "other_info":
		return channel.OtherInfo, true
	case "tag":
		return channel.Tag, true
	case "setting":
		return channel.Setting, true
	case "param_override":
		return channel.ParamOverride, true
	case "header_override":
		return channel.HeaderOverride, true
	case "remark":
		return channel.Remark, true
	case "settings":
		return channel.OtherSettings, true
	case "multi_key_mode":
		return channel.ChannelInfo.MultiKeyMode, true
	default:
		return nil, false
	}
}

func fieldValuesEqual(field string, left any, right any) bool {
	if !semanticJSONFields[field] {
		return reflect.DeepEqual(left, right)
	}
	leftText := indirectString(left)
	rightText := indirectString(right)
	var leftJSON any
	var rightJSON any
	if common.Unmarshal([]byte(leftText), &leftJSON) == nil && common.Unmarshal([]byte(rightText), &rightJSON) == nil {
		return reflect.DeepEqual(leftJSON, rightJSON)
	}
	return leftText == rightText
}

func indirectString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case *string:
		if typed == nil {
			return ""
		}
		return strings.TrimSpace(*typed)
	default:
		return ""
	}
}

type endpointSnapshot struct {
	Model       string
	BaseURL     string
	ChannelType int
	HasType     bool
}

func modelEndpointsEqual(left []*model.ModelEndpoint, right []*model.ModelEndpoint) bool {
	return reflect.DeepEqual(endpointSnapshots(left), endpointSnapshots(right))
}

func endpointSnapshots(endpoints []*model.ModelEndpoint) []endpointSnapshot {
	result := make([]endpointSnapshot, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint == nil {
			continue
		}
		snapshot := endpointSnapshot{
			Model:   strings.TrimSpace(endpoint.Model),
			BaseURL: strings.TrimSpace(endpoint.BaseURL),
		}
		if endpoint.ChannelType != nil {
			snapshot.ChannelType = *endpoint.ChannelType
			snapshot.HasType = true
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Model != result[j].Model {
			return result[i].Model < result[j].Model
		}
		if result[i].BaseURL != result[j].BaseURL {
			return result[i].BaseURL < result[j].BaseURL
		}
		if result[i].HasType != result[j].HasType {
			return !result[i].HasType
		}
		return result[i].ChannelType < result[j].ChannelType
	})
	return result
}

func compactSortedStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read] == values[write-1] {
			continue
		}
		values[write] = values[read]
		write++
	}
	return values[:write]
}
