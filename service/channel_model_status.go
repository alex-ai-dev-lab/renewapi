package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

const (
	channelModelAutoDisableFailureThreshold = 3
	channelModelAutoDisableCooldownSeconds  = 10 * 60
)

type ChannelModelFailureParams struct {
	ChannelId     int
	Group         string
	ModelName     string
	Endpoint      string
	RequestId     string
	Error         *types.NewAPIError
	AutoBan       bool
	ForceDisabled bool
}

func RecordChannelModelSuccess(channelID int, group, modelName, endpoint, requestID string) {
	modelName = strings.TrimSpace(modelName)
	if channelID <= 0 || modelName == "" {
		return
	}
	if common.MemoryCacheEnabled && !model.HasChannelModelStatusCached(channelID, group, modelName) {
		return
	}
	_ = model.RecordChannelModelSuccess(model.ChannelModelSuccessUpdate{
		ChannelId:     channelID,
		Group:         group,
		ModelName:     modelName,
		LastEndpoint:  endpoint,
		LastRequestId: requestID,
	})
}

func RecordChannelModelFailure(params ChannelModelFailureParams) {
	params.ModelName = strings.TrimSpace(params.ModelName)
	if params.ChannelId <= 0 || params.ModelName == "" || params.Error == nil {
		return
	}
	if !IsRelayFailoverError(params.Error) {
		return
	}
	_ = model.UpsertChannelModelFailure(model.ChannelModelFailureUpdate{
		ChannelId:           params.ChannelId,
		Group:               params.Group,
		ModelName:           params.ModelName,
		LastError:           params.Error.MaskSensitiveErrorWithStatusCode(),
		LastStatusCode:      params.Error.StatusCode,
		LastEndpoint:        params.Endpoint,
		LastRequestId:       params.RequestId,
		AutoDisableEligible: params.AutoBan && common.AutomaticDisableChannelEnabled && (IsModelScopedChannelFailureError(params.Error) || ShouldCountChannelFailureForDisable(params.Error)),
		ForceDisabled:       params.ForceDisabled,
		FailureThreshold:    channelModelAutoDisableFailureThreshold,
		CooldownSeconds:     channelModelAutoDisableCooldownSeconds,
	})
}
