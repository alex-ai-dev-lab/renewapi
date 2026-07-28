package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"
)

type ProtocolBridgeCapability struct {
	Supported bool                  `json:"supported"`
	Lossy     bool                  `json:"lossy"`
	Bridge    string                `json:"bridge,omitempty"`
	Reason    string                `json:"reason"`
	Endpoint  constant.EndpointType `json:"endpoint"`
}

func supportedCapability(endpoint constant.EndpointType, bridge, reason string) ProtocolBridgeCapability {
	return ProtocolBridgeCapability{Supported: true, Bridge: bridge, Reason: reason, Endpoint: endpoint}
}

func unsupportedCapability(endpoint constant.EndpointType, bridge, reason string) ProtocolBridgeCapability {
	return ProtocolBridgeCapability{Supported: false, Bridge: bridge, Reason: reason, Endpoint: endpoint}
}

func EvaluateChannelProtocolCapability(channel *model.Channel, modelName string, clientFormat types.RelayFormat, request dto.Request) ProtocolBridgeCapability {
	decision := model.ResolveModelRouteDecision(channel, modelName)
	target := decision.Endpoint
	overrideAllowed := model.ChannelAllowsModelProtocolOverrideTarget(channel, target)

	switch clientFormat {
	case types.RelayFormatOpenAIResponses:
		if decision.Source == model.ModelRouteSourceNative && ChannelSupportsOpenAIResponses(channel) {
			return supportedCapability(constant.EndpointTypeOpenAIResponse, "native", "channel natively supports Responses")
		}
		switch target {
		case constant.EndpointTypeOpenAIResponse:
			if decision.Source != model.ModelRouteSourceNative && overrideAllowed {
				return supportedCapability(target, "native", "allowlisted upstream Responses override")
			}
			if ChannelSupportsOpenAIResponses(channel) {
				return supportedCapability(target, "native", "upstream uses Responses")
			}
			return unsupportedCapability(target, "", "channel does not natively support Responses and no override applies")
		case constant.EndpointTypeOpenAI, constant.EndpointTypeAnthropic:
			if decision.Source == model.ModelRouteSourceNative || !overrideAllowed {
				return unsupportedCapability(target, "", "protocol override is disabled for this channel")
			}
			req, _ := request.(*dto.OpenAIResponsesRequest)
			if req != nil {
				if err := ValidateResponsesTextBridgeRequest(req); err != nil {
					return unsupportedCapability(target, "responses->"+string(target), err.Error())
				}
			}
			return supportedCapability(target, "responses->"+string(target), "safe Responses text bridge available")
		default:
			return unsupportedCapability(target, "", "no Responses bridge is registered for target endpoint")
		}
	case types.RelayFormatOpenAI:
		if target == constant.EndpointTypeOpenAIResponse && decision.Source != model.ModelRouteSourceNative && overrideAllowed {
			req, _ := request.(*dto.GeneralOpenAIRequest)
			if req != nil {
				if err := ValidateChatTextBridgeRequest(req); err != nil {
					return unsupportedCapability(target, "openai->responses", err.Error())
				}
			}
			return supportedCapability(target, "openai->responses", "safe Chat text bridge available")
		}
	case types.RelayFormatClaude:
		if target == constant.EndpointTypeOpenAIResponse && decision.Source != model.ModelRouteSourceNative && overrideAllowed {
			req, _ := request.(*dto.ClaudeRequest)
			if req != nil {
				if err := ValidateClaudeTextBridgeRequest(req); err != nil {
					return unsupportedCapability(target, "claude->responses", err.Error())
				}
			}
			return supportedCapability(target, "claude->responses", "safe Claude text bridge available")
		}
	}

	return supportedCapability(target, "native", "existing native adaptor path")
}

func ValidateResponsesTextBridgeRequest(req *dto.OpenAIResponsesRequest) error {
	if req == nil {
		return fmt.Errorf("responses request is nil")
	}
	unsupported := []struct {
		set  bool
		name string
	}{
		{len(req.Include) > 0, "include"},
		{len(req.Conversation) > 0, "conversation"},
		{len(req.ContextManagement) > 0, "context_management"},
		{req.PreviousResponseID != "", "previous_response_id"},
		{req.Reasoning != nil, "reasoning"},
		{len(req.Metadata) > 0, "metadata"},
		{len(req.ServiceTier) > 0, "service_tier"},
		{len(req.PromptCacheKey) > 0, "prompt_cache_key"},
		{len(req.PromptCacheRetention) > 0, "prompt_cache_retention"},
		{len(req.SafetyIdentifier) > 0, "safety_identifier"},
		{len(req.Text) > 0, "text/output format"},
		{len(req.Truncation) > 0, "truncation"},
		{len(req.User) > 0, "user"},
		{req.MaxToolCalls != nil, "max_tool_calls"},
		{len(req.Prompt) > 0, "prompt"},
		{len(req.EnableThinking) > 0, "enable_thinking"},
		{len(req.Preset) > 0, "preset"},
	}
	for _, item := range unsupported {
		if item.set {
			return fmt.Errorf("Responses field %s cannot be represented by the safe text bridge", item.name)
		}
	}
	if len(req.Store) > 0 && string(req.Store) != "false" && string(req.Store) != "null" {
		return fmt.Errorf("Responses field store cannot be represented by the safe text bridge")
	}
	prepared, _, err := openaicompat.PrepareResponsesRequestForTextBridge(req)
	if err != nil {
		return err
	}
	if err := validateResponsesInput(prepared.Input); err != nil {
		return err
	}
	if err := validateResponsesFunctionTools(prepared.Tools); err != nil {
		return err
	}
	return nil
}

func validateResponsesFunctionTools(raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return fmt.Errorf("invalid Responses tools: %w", err)
	}
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType != "function" {
			return fmt.Errorf("Responses tool type %q is not supported by the safe text bridge", toolType)
		}
	}
	return nil
}

func validateResponsesInput(raw []byte) error {
	if len(raw) == 0 || common.GetJsonType(raw) == "string" {
		return nil
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("invalid Responses input: %w", err)
	}
	return validateResponsesInputValue(value)
}

func validateResponsesInputValue(value any) error {
	switch item := value.(type) {
	case string, nil:
		return nil
	case []any:
		for _, child := range item {
			if err := validateResponsesInputValue(child); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "", "message", "input_text", "output_text", "text", "function_call", "function_call_output":
		default:
			return fmt.Errorf("Responses input type %q is not supported by the safe text bridge", itemType)
		}
		if content, ok := item["content"]; ok {
			return validateResponsesInputValue(content)
		}
		return nil
	default:
		return fmt.Errorf("Responses input contains an unsupported value")
	}
}

func ValidateChatTextBridgeRequest(req *dto.GeneralOpenAIRequest) error {
	if req == nil {
		return fmt.Errorf("chat request is nil")
	}
	if req.WebSearchOptions != nil || len(req.Reasoning) > 0 || req.ReasoningEffort != "" || len(req.Modalities) > 0 || len(req.Audio) > 0 || req.ResponseFormat != nil || len(req.Prediction) > 0 || len(req.SearchParameters) > 0 || len(req.EnableSearch) > 0 {
		return fmt.Errorf("Chat request uses a feature outside the safe text/function bridge")
	}
	for _, tool := range req.Tools {
		if tool.Type != "function" {
			return fmt.Errorf("Chat tool type %q is not supported by the safe bridge", tool.Type)
		}
	}
	for _, message := range req.Messages {
		for _, part := range message.ParseContent() {
			if part.Type != dto.ContentTypeText {
				return fmt.Errorf("Chat content type %q is not supported by the safe bridge", part.Type)
			}
		}
	}
	return nil
}

func ValidateClaudeTextBridgeRequest(req *dto.ClaudeRequest) error {
	if req == nil {
		return fmt.Errorf("Claude request is nil")
	}
	if req.Thinking != nil || len(req.ContextManagement) > 0 || len(req.OutputConfig) > 0 || len(req.OutputFormat) > 0 || len(req.Container) > 0 || len(req.McpServers) > 0 || len(req.Speed) > 0 || req.ServiceTier != "" {
		return fmt.Errorf("Claude request uses a feature outside the safe text/function bridge")
	}
	if req.System != nil && !req.IsStringSystem() {
		for _, part := range req.ParseSystem() {
			if part.Type != dto.ContentTypeText {
				return fmt.Errorf("Claude system content type %q is not supported by the safe bridge", part.Type)
			}
		}
	}
	for _, message := range req.Messages {
		if message.IsStringContent() {
			continue
		}
		parts, err := message.ParseContent()
		if err != nil {
			return fmt.Errorf("invalid Claude message content: %w", err)
		}
		for _, part := range parts {
			switch part.Type {
			case dto.ContentTypeText, "tool_use", "tool_result":
			default:
				return fmt.Errorf("Claude content type %q is not supported by the safe bridge", part.Type)
			}
		}
	}
	if req.Tools != nil {
		var tools []map[string]any
		b, err := common.Marshal(req.Tools)
		if err != nil || common.Unmarshal(b, &tools) != nil {
			return fmt.Errorf("invalid Claude tools")
		}
		for _, tool := range tools {
			if toolType := strings.TrimSpace(common.Interface2String(tool["type"])); toolType != "" && toolType != "custom" {
				return fmt.Errorf("Claude tool type %q is not supported by the safe bridge", toolType)
			}
		}
	}
	return nil
}
