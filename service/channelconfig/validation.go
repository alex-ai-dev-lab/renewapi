package channelconfig

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type ModelEndpointInput struct {
	Model       string `json:"model"`
	BaseURL     string `json:"base_url"`
	ChannelType *int   `json:"channel_type"`
}

func NormalizeModelEndpoints(channelID int, payload []ModelEndpointInput) ([]*model.ModelEndpoint, error) {
	if channelID <= 0 {
		return nil, fmt.Errorf("invalid channel id")
	}
	endpoints := make([]*model.ModelEndpoint, 0, len(payload))
	seen := make(map[string]struct{}, len(payload))
	for index, item := range payload {
		modelName := strings.TrimSpace(item.Model)
		if modelName == "" {
			return nil, fmt.Errorf("model endpoint %d: model is required", index+1)
		}
		if len([]rune(modelName)) > 255 {
			return nil, fmt.Errorf("model endpoint %d: model exceeds 255 characters", index+1)
		}
		if _, exists := seen[modelName]; exists {
			return nil, fmt.Errorf("duplicate model endpoint: %s", modelName)
		}
		seen[modelName] = struct{}{}

		baseURL := strings.TrimSpace(item.BaseURL)
		if len([]rune(baseURL)) > 512 {
			return nil, fmt.Errorf("model endpoint %d: base_url exceeds 512 characters", index+1)
		}
		if baseURL != "" {
			parsed, err := url.Parse(baseURL)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return nil, fmt.Errorf("model endpoint %d: base_url must be an absolute HTTP(S) URL", index+1)
			}
			if parsed.User != nil {
				return nil, fmt.Errorf("model endpoint %d: base_url must not contain user credentials", index+1)
			}
		}
		if item.ChannelType != nil {
			if *item.ChannelType <= constant.ChannelTypeUnknown {
				return nil, fmt.Errorf("model endpoint %d: unknown channel_type %d", index+1, *item.ChannelType)
			}
			if _, known := constant.ChannelTypeNames[*item.ChannelType]; !known {
				return nil, fmt.Errorf("model endpoint %d: unknown channel_type %d", index+1, *item.ChannelType)
			}
		}
		endpoints = append(endpoints, &model.ModelEndpoint{
			ChannelId: channelID, Model: modelName, BaseURL: baseURL, ChannelType: item.ChannelType,
		})
	}
	return endpoints, nil
}

func ValidateChannel(channel *model.Channel, isAdd bool) error {
	if channel == nil {
		return fmt.Errorf("channel cannot be empty")
	}
	if err := channel.ValidateSettings(); err != nil {
		return fmt.Errorf("渠道额外设置[channel setting] 格式错误：%s", err.Error())
	}
	if isAdd && strings.TrimSpace(channel.Key) == "" {
		return fmt.Errorf("channel cannot be empty")
	}
	if strings.TrimSpace(channel.Name) == "" {
		return fmt.Errorf("渠道名称不能为空")
	}
	if len(channel.GetModels()) == 0 {
		return fmt.Errorf("模型不能为空")
	}
	if len(channel.GetGroups()) == 0 {
		return fmt.Errorf("分组不能为空")
	}
	for _, modelName := range channel.GetModels() {
		if len([]rune(modelName)) > 255 {
			return fmt.Errorf("模型名称过长: %s", modelName)
		}
	}
	for _, group := range channel.GetGroups() {
		if len([]rune(group)) > 64 {
			return fmt.Errorf("分组名称过长: %s", group)
		}
	}
	if channel.Remark != nil && len([]rune(*channel.Remark)) > 255 {
		return fmt.Errorf("渠道备注不能超过 255 个字符")
	}
	if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(*channel.BaseURL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("渠道地址必须是绝对 HTTP(S) URL")
		}
		if parsed.User != nil {
			return fmt.Errorf("渠道地址不能包含用户凭据")
		}
	}
	if channel.ModelMapping != nil {
		var mapping map[string]any
		if err := common.Unmarshal([]byte(*channel.ModelMapping), &mapping); err != nil {
			return fmt.Errorf("模型映射格式错误: %w", err)
		}
		if mapping == nil {
			return fmt.Errorf("模型映射必须是 JSON 对象")
		}
	}
	if channel.Type == constant.ChannelTypeVertexAi {
		if channel.Other == "" {
			return fmt.Errorf("部署地区不能为空")
		}
		regionMap, err := common.StrToMap(channel.Other)
		if err != nil || regionMap["default"] == nil {
			return fmt.Errorf("部署地区必须是包含 default 字段的标准 JSON")
		}
	}
	if channel.Type == constant.ChannelTypeCodex {
		trimmedKey := strings.TrimSpace(channel.Key)
		if isAdd || trimmedKey != "" {
			if !strings.HasPrefix(trimmedKey, "{") {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			var keyMap map[string]any
			if err := common.Unmarshal([]byte(trimmedKey), &keyMap); err != nil {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			for _, required := range []string{"access_token", "account_id"} {
				if value, ok := keyMap[required]; !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
					return fmt.Errorf("Codex key JSON must include %s", required)
				}
			}
		}
	}
	return nil
}
