package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func NormalizeResponsesFunctionCallArgumentsFormat(format dto.ResponsesFunctionCallArgumentsFormat) dto.ResponsesFunctionCallArgumentsFormat {
	switch format {
	case dto.ResponsesFunctionCallArgumentsFormatObject,
		dto.ResponsesFunctionCallArgumentsFormatString:
		return format
	default:
		return dto.ResponsesFunctionCallArgumentsFormatAuto
	}
}

func EffectiveResponsesFunctionCallArgumentsFormat(channelType int, setting dto.ChannelSettings, forceObject bool) dto.ResponsesFunctionCallArgumentsFormat {
	format := NormalizeResponsesFunctionCallArgumentsFormat(setting.ResponsesFunctionCallArgumentsFormat)
	switch format {
	case dto.ResponsesFunctionCallArgumentsFormatObject,
		dto.ResponsesFunctionCallArgumentsFormatString:
		return format
	default:
		if forceObject || channelType == constant.ChannelTypeCodex {
			return dto.ResponsesFunctionCallArgumentsFormatObject
		}
		return dto.ResponsesFunctionCallArgumentsFormatString
	}
}

func ShouldEnforceResponsesFunctionCallArgumentsFormat(channelType int, setting dto.ChannelSettings, forceObject bool) bool {
	format := NormalizeResponsesFunctionCallArgumentsFormat(setting.ResponsesFunctionCallArgumentsFormat)
	return forceObject ||
		channelType == constant.ChannelTypeCodex ||
		format == dto.ResponsesFunctionCallArgumentsFormatObject ||
		format == dto.ResponsesFunctionCallArgumentsFormatString
}

func NormalizeResponsesFunctionCallArgumentsPayload(payload any, format dto.ResponsesFunctionCallArgumentsFormat) (any, bool, error) {
	switch req := payload.(type) {
	case dto.OpenAIResponsesRequest:
		changed, err := NormalizeResponsesFunctionCallArguments(&req, format)
		return req, changed, err
	case *dto.OpenAIResponsesRequest:
		changed, err := NormalizeResponsesFunctionCallArguments(req, format)
		return req, changed, err
	default:
		return payload, false, nil
	}
}

func NormalizeResponsesFunctionCallArguments(req *dto.OpenAIResponsesRequest, format dto.ResponsesFunctionCallArgumentsFormat) (bool, error) {
	if req == nil || len(req.Input) == 0 {
		return false, nil
	}
	format = NormalizeResponsesFunctionCallArgumentsFormat(format)
	if format == dto.ResponsesFunctionCallArgumentsFormatAuto {
		format = dto.ResponsesFunctionCallArgumentsFormatString
	}

	input := strings.TrimSpace(string(req.Input))
	if input == "" || !strings.HasPrefix(input, "[") {
		return false, nil
	}

	var items []any
	if err := common.Unmarshal(req.Input, &items); err != nil {
		return false, fmt.Errorf("invalid responses input array: %w", err)
	}

	changed := false
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if itemType, _ := item["type"].(string); itemType != "function_call" {
			continue
		}

		var (
			nextValue   any
			itemChanged bool
			err         error
		)
		switch format {
		case dto.ResponsesFunctionCallArgumentsFormatObject:
			nextValue, itemChanged, err = normalizeResponsesFunctionCallArgumentsObject(item["arguments"])
		case dto.ResponsesFunctionCallArgumentsFormatString:
			nextValue, itemChanged, err = normalizeResponsesFunctionCallArgumentsString(item["arguments"])
		}
		if err != nil {
			return false, err
		}
		if itemChanged {
			item["arguments"] = nextValue
			changed = true
		}
	}

	if !changed {
		return false, nil
	}
	inputRaw, err := common.Marshal(items)
	if err != nil {
		return false, fmt.Errorf("failed to marshal normalized responses input: %w", err)
	}
	req.Input = inputRaw
	return true, nil
}

func normalizeResponsesFunctionCallArgumentsObject(value any) (any, bool, error) {
	switch typed := value.(type) {
	case nil:
		return map[string]any{}, true, nil
	case map[string]any:
		if typed == nil {
			return map[string]any{}, true, nil
		}
		return typed, false, nil
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return map[string]any{}, true, nil
		}
		var parsed map[string]any
		if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil, false, fmt.Errorf("responses function_call.arguments must be a JSON object string for this upstream: %w", err)
		}
		if parsed == nil {
			return nil, false, errors.New("responses function_call.arguments must be a JSON object, got null")
		}
		return parsed, true, nil
	default:
		return nil, false, fmt.Errorf("responses function_call.arguments must be a JSON object for this upstream, got %T", value)
	}
}

func normalizeResponsesFunctionCallArgumentsString(value any) (any, bool, error) {
	switch typed := value.(type) {
	case nil:
		return "{}", true, nil
	case string:
		return typed, false, nil
	default:
		raw, err := common.Marshal(typed)
		if err != nil {
			return nil, false, fmt.Errorf("failed to marshal responses function_call.arguments as string: %w", err)
		}
		return string(raw), true, nil
	}
}

func IsResponsesFunctionCallArgumentsObjectTypeError(openaiErr *types.NewAPIError) bool {
	if openaiErr == nil || openaiErr.StatusCode != http.StatusBadRequest {
		return false
	}
	msg := strings.ToLower(openaiErr.Error())
	return strings.Contains(msg, "invalid type") &&
		strings.Contains(msg, "arguments") &&
		strings.Contains(msg, "expected an object") &&
		strings.Contains(msg, "got a string")
}
