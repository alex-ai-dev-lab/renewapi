package channelconfig

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	runtimeservice "github.com/QuantumNous/new-api/service"

	"gorm.io/gorm"
)

type AuditMetadata struct {
	OperatorID int
	RequestID  string
	Reason     string
}

type UpdateCommand struct {
	Channel               *model.Channel
	ExpectedConfigVersion *int64
	PresentFields         map[string]bool
	UpdateKey             bool
	ModelEndpoints        *[]*model.ModelEndpoint
	Audit                  AuditMetadata
}

type UpdateResult struct {
	Channel           *model.Channel `json:"channel"`
	ChangeSet         ChangeSet      `json:"change_set"`
	NoOp              bool           `json:"no_op"`
	CacheSynchronized bool           `json:"cache_synchronized"`
	Warnings          []string       `json:"warnings,omitempty"`
}

type auditDocument struct {
	ChangedFields      []string           `json:"changed_fields"`
	Changes            []AuditFieldChange `json:"changes"`
	KeyChanged         bool               `json:"key_changed"`
	ModelEndpointCount *int               `json:"model_endpoint_count,omitempty"`
}

func Update(command UpdateCommand) (*UpdateResult, error) {
	if command.Channel == nil || command.Channel.Id <= 0 {
		return nil, fmt.Errorf("invalid channel")
	}
	reason := strings.TrimSpace(command.Audit.Reason)
	if reason == "" { reason = "channel configuration update" }
	if len([]rune(reason)) > 255 { return nil, fmt.Errorf("change_reason must not exceed 255 characters") }
	if err := ValidateChannel(command.Channel, false); err != nil { return nil, err }

	result := &UpdateResult{CacheSynchronized: true}
	var beforeSettings, afterSettings = command.Channel.GetSetting(), command.Channel.GetSetting()
	previousStatus := 0
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		before, err := model.GetChannelByIDTx(tx, command.Channel.Id)
		if err != nil { return err }
		beforeSettings = before.GetSetting()
		if command.ExpectedConfigVersion != nil && before.ConfigVersion != *command.ExpectedConfigVersion { return model.ErrChannelConfigConflict }

		beforeEndpoints := make([]*model.ModelEndpoint, 0)
		if command.ModelEndpoints != nil {
			beforeEndpoints, err = model.GetChannelModelEndpointsTx(tx, command.Channel.Id)
			if err != nil { return err }
		}
		proposed := *command.Channel
		proposed.ConfigVersion = before.ConfigVersion
		if !command.UpdateKey { proposed.Key = before.Key }
		changes := BuildChangeSet(before, &proposed, command.PresentFields, command.UpdateKey, beforeEndpoints, command.ModelEndpoints)
		result.ChangeSet = changes
		if !changes.Changed() {
			result.Channel, result.NoOp = before, true
			return nil
		}
		previousStatus, err = proposed.UpdateConfigTx(tx, command.ExpectedConfigVersion, command.UpdateKey, changes.AbilityChanged)
		if err != nil { return err }
		if changes.EndpointsChanged {
			if err := model.ReplaceChannelModelEndpointsTx(tx, proposed.Id, *command.ModelEndpoints); err != nil { return err }
		}
		document := auditDocument{ChangedFields: changes.ChangedFields, Changes: changes.FieldChanges, KeyChanged: changes.KeyChanged}
		if changes.EndpointsChanged { count := changes.EndpointCount; document.ModelEndpointCount = &count }
		diff, err := common.Marshal(document)
		if err != nil { return err }
		audit := &model.ConfigAudit{ResourceType: "channel", ResourceId: proposed.Id, Action: "update", OperatorId: command.Audit.OperatorID,
			Reason: reason, RequestId: strings.TrimSpace(command.Audit.RequestID), Diff: string(diff), ConfigVersion: proposed.ConfigVersion, CreatedAt: common.GetTimestamp()}
		if err := model.CreateConfigAuditTx(tx, audit); err != nil { return err }
		result.Channel = &proposed
		afterSettings = proposed.GetSetting()
		return nil
	})
	if err != nil { return nil, err }
	if result.NoOp { return result, nil }

	if result.ChangeSet.TransportChanged {
		runtimeservice.InvalidateHTTPClientSettings(beforeSettings)
		runtimeservice.InvalidateHTTPClientSettings(afterSettings)
	}
	if result.ChangeSet.RoutingChanged || result.ChangeSet.AbilityChanged {
		if err := retryCacheRefresh("channel", model.ReloadChannelCache); err != nil {
			result.CacheSynchronized = false
			result.Warnings = append(result.Warnings, err.Error())
		}
	} else {
		model.CacheUpdateChannel(result.Channel)
	}
	if result.ChangeSet.EndpointsChanged {
		if err := retryCacheRefresh("model endpoint", model.ReloadModelEndpointCacheWithError); err != nil {
			result.CacheSynchronized = false
			result.Warnings = append(result.Warnings, err.Error())
		}
	}
	if previousStatus != common.ChannelStatusEnabled && result.Channel.Status == common.ChannelStatusEnabled { model.OnChannelEnabled(result.Channel.Id) }
	return result, nil
}

func retryCacheRefresh(name string, refresh func() error) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = refresh(); err == nil { return nil }
		if attempt < 3 { time.Sleep(time.Duration(attempt) * 10 * time.Millisecond) }
	}
	message := fmt.Sprintf("%s cache refresh failed after commit: %v", name, err)
	common.SysError(message)
	return fmt.Errorf("%s", message)
}
