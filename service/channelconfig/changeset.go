package channelconfig

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type AuditValue struct {
	Redacted bool   `json:"redacted,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	Value    any    `json:"value,omitempty"`
}

type AuditFieldChange struct {
	Field  string     `json:"field"`
	Before AuditValue `json:"before"`
	After  AuditValue `json:"after"`
}

type ChangeSet struct {
	ChangedFields    []string           `json:"changed_fields"`
	FieldChanges     []AuditFieldChange `json:"field_changes,omitempty"`
	KeyChanged       bool               `json:"key_changed"`
	AbilityChanged   bool               `json:"ability_changed"`
	TransportChanged bool               `json:"transport_changed"`
	RoutingChanged   bool               `json:"routing_changed"`
	ProtocolChanged  bool               `json:"protocol_changed"`
	EndpointsChanged bool               `json:"endpoints_changed"`
	EndpointCount    int                `json:"endpoint_count"`
}

func (c ChangeSet) Changed() bool { return c.KeyChanged || c.EndpointsChanged || len(c.ChangedFields) > 0 }

var abilityFields = map[string]bool{"status": true, "models": true, "group": true, "model_mapping": true, "priority": true, "weight": true, "tag": true}
var transportFields = map[string]bool{"type": true, "base_url": true, "other": true, "setting": true, "settings": true, "openai_organization": true, "header_override": true}
var routingFields = map[string]bool{"type": true, "status": true, "models": true, "group": true, "model_mapping": true, "priority": true, "weight": true, "setting": true}
var protocolFields = map[string]bool{"type": true, "setting": true, "settings": true}
var semanticJSONFields = map[string]bool{"model_mapping": true, "status_code_mapping": true, "other_info": true, "param_override": true, "header_override": true, "setting": true, "settings": true}
var sensitiveAuditFields = map[string]bool{"key": true, "other": true, "other_info": true, "setting": true, "settings": true, "param_override": true, "header_override": true, "openai_organization": true}

func BuildChangeSet(before, after *model.Channel, presentFields map[string]bool, updateKey bool, beforeEndpoints []*model.ModelEndpoint, requestedEndpoints *[]*model.ModelEndpoint) ChangeSet {
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
		changes.FieldChanges = append(changes.FieldChanges, AuditFieldChange{Field: field, Before: auditValue(field, beforeValue), After: auditValue(field, afterValue)})
	}
	changes.KeyChanged = updateKey && before.Key != after.Key
	if changes.KeyChanged {
		changes.ChangedFields = append(changes.ChangedFields, "key")
		changes.FieldChanges = append(changes.FieldChanges, AuditFieldChange{Field: "key", Before: auditValue("key", before.Key), After: auditValue("key", after.Key)})
	}
	if requestedEndpoints != nil {
		changes.EndpointCount = len(*requestedEndpoints)
		changes.EndpointsChanged = !modelEndpointsEqual(beforeEndpoints, *requestedEndpoints)
		if changes.EndpointsChanged {
			changes.ChangedFields = append(changes.ChangedFields, "model_endpoints")
			changes.FieldChanges = append(changes.FieldChanges, AuditFieldChange{Field: "model_endpoints", Before: auditValue("model_endpoints", endpointSnapshots(beforeEndpoints)), After: auditValue("model_endpoints", endpointSnapshots(*requestedEndpoints))})
		}
	}
	sort.Strings(changes.ChangedFields)
	changes.ChangedFields = compactSortedStrings(changes.ChangedFields)
	sort.Slice(changes.FieldChanges, func(i, j int) bool { return changes.FieldChanges[i].Field < changes.FieldChanges[j].Field })
	for _, field := range changes.ChangedFields {
		changes.AbilityChanged = changes.AbilityChanged || abilityFields[field]
		changes.TransportChanged = changes.TransportChanged || transportFields[field]
		changes.RoutingChanged = changes.RoutingChanged || routingFields[field]
		changes.ProtocolChanged = changes.ProtocolChanged || protocolFields[field]
	}
	if changes.KeyChanged { changes.TransportChanged = true }
	if changes.EndpointsChanged { changes.TransportChanged, changes.RoutingChanged, changes.ProtocolChanged = true, true, true }
	return changes
}

func auditValue(field string, value any) AuditValue {
	if sensitiveAuditFields[field] {
		encoded, _ := common.Marshal(value)
		hash := sha256.Sum256(encoded)
		return AuditValue{Redacted: true, SHA256: fmt.Sprintf("%x", hash[:])}
	}
	return AuditValue{Value: value}
}

func channelFieldValue(channel *model.Channel, field string) (any, bool) {
	switch field {
	case "type": return channel.Type, true
	case "openai_organization": return channel.OpenAIOrganization, true
	case "test_model": return channel.TestModel, true
	case "status": return channel.Status, true
	case "name": return channel.Name, true
	case "weight": return channel.Weight, true
	case "base_url": return channel.BaseURL, true
	case "other": return channel.Other, true
	case "balance": return channel.Balance, true
	case "balance_updated_time": return channel.BalanceUpdatedTime, true
	case "models": return channel.Models, true
	case "group": return channel.Group, true
	case "model_mapping": return channel.ModelMapping, true
	case "status_code_mapping": return channel.StatusCodeMapping, true
	case "priority": return channel.Priority, true
	case "auto_ban": return channel.AutoBan, true
	case "other_info": return channel.OtherInfo, true
	case "tag": return channel.Tag, true
	case "setting": return channel.Setting, true
	case "param_override": return channel.ParamOverride, true
	case "header_override": return channel.HeaderOverride, true
	case "remark": return channel.Remark, true
	case "settings": return channel.OtherSettings, true
	case "multi_key_mode": return channel.ChannelInfo.MultiKeyMode, true
	default: return nil, false
	}
}

func fieldValuesEqual(field string, left, right any) bool {
	if !semanticJSONFields[field] { return reflect.DeepEqual(left, right) }
	leftText, rightText := indirectString(left), indirectString(right)
	var leftJSON, rightJSON any
	if common.Unmarshal([]byte(leftText), &leftJSON) == nil && common.Unmarshal([]byte(rightText), &rightJSON) == nil { return reflect.DeepEqual(leftJSON, rightJSON) }
	return leftText == rightText
}

func indirectString(value any) string {
	switch typed := value.(type) { case string: return strings.TrimSpace(typed); case *string: if typed != nil { return strings.TrimSpace(*typed) } }
	return ""
}

type endpointSnapshot struct { Model string `json:"model"`; BaseURL string `json:"base_url"`; ChannelType *int `json:"channel_type,omitempty"` }
func endpointSnapshots(endpoints []*model.ModelEndpoint) []endpointSnapshot {
	result := make([]endpointSnapshot, 0, len(endpoints))
	for _, endpoint := range endpoints { if endpoint != nil { result = append(result, endpointSnapshot{Model: strings.TrimSpace(endpoint.Model), BaseURL: strings.TrimSpace(endpoint.BaseURL), ChannelType: endpoint.ChannelType}) } }
	sort.Slice(result, func(i, j int) bool { return result[i].Model < result[j].Model })
	return result
}
func modelEndpointsEqual(left, right []*model.ModelEndpoint) bool { return reflect.DeepEqual(endpointSnapshots(left), endpointSnapshots(right)) }
func compactSortedStrings(values []string) []string { if len(values)<2{return values}; write:=1; for read:=1;read<len(values);read++{if values[read]!=values[write-1]{values[write]=values[read];write++}}; return values[:write] }
