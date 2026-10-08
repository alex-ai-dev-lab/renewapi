package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"
)

func ValidateNormalizedBridgeRequest(request dto.Request, target constant.EndpointType) error {
	if request == nil {
		return nil
	}
	switch req := request.(type) {
	case *dto.ClaudeRequest:
		if target == constant.EndpointTypeAnthropic {
			return nil
		}
		if target == constant.EndpointTypeOpenAIResponse {
			_, err := openaicompat.ClaudeRequestToResponsesRequest(req)
			return err
		}
		copy := *req
		copy.Thinking = nil
		copy.OutputConfig = nil
		copy.Messages = nil
		for _, message := range req.Messages {
			if message.IsStringContent() {
				continue
			}
			parts, err := message.ParseContent()
			if err != nil {
				return err
			}
			for _, part := range parts {
				switch part.Type {
				case "text", "input_text", "image", "tool_use", "tool_result":
				default:
					return fmt.Errorf("unsupported Messages content %q for Chat", part.Type)
				}
			}
		}
		return ValidateClaudeTextBridgeRequest(&copy)
	case *dto.GeneralOpenAIRequest:
		if target == constant.EndpointTypeOpenAI {
			return nil
		}
		copy := *req
		copy.Reasoning = nil
		copy.ReasoningEffort = ""
		copy.ResponseFormat = nil
		copy.Messages = nil
		for _, message := range req.Messages {
			for _, part := range message.ParseContent() {
				if part.Type != dto.ContentTypeText && part.Type != dto.ContentTypeImageURL {
					return fmt.Errorf("unsupported Chat content %q", part.Type)
				}
			}
		}
		return ValidateChatTextBridgeRequest(&copy)
	case *dto.OpenAIResponsesRequest:
		if target == constant.EndpointTypeOpenAIResponse {
			return nil
		}
		copy := *req
		copy.Reasoning = nil
		copy.Metadata = nil
		copy.ServiceTier = ""
		copy.Text = nil
		copy.Input = nil
		if err := validateNormalizedResponsesInput(req.Input); err != nil {
			return err
		}
		return ValidateResponsesTextBridgeRequest(&copy)
	default:
		return fmt.Errorf("request type %T cannot use the three text endpoint bridge", request)
	}
}

func validateNormalizedResponsesInput(raw []byte) error {
	if len(raw) == 0 || common.GetJsonType(raw) == "string" {
		return nil
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return fmt.Errorf("invalid Responses input: %w", err)
	}
	var visit func(any) error
	visit = func(value any) error {
		switch item := value.(type) {
		case nil, string:
			return nil
		case []any:
			for _, child := range item {
				if err := visit(child); err != nil {
					return err
				}
			}
		case map[string]any:
			kind, _ := item["type"].(string)
			switch kind {
			case "", "message", "input_text", "output_text", "text", "input_image", "function_call", "function_call_output":
			default:
				return fmt.Errorf("unsupported Responses input type %q", kind)
			}
			if err := visit(item["content"]); err != nil {
				return err
			}
			if kind == "function_call_output" {
				return visit(item["output"])
			}
		default:
			return fmt.Errorf("unsupported Responses content value")
		}
		return nil
	}
	for _, item := range items {
		if err := visit(item); err != nil {
			return err
		}
	}
	return nil
}

func TextEndpointForClient(format types.RelayFormat) constant.EndpointType {
	switch format {
	case types.RelayFormatOpenAI:
		return constant.EndpointTypeOpenAI
	case types.RelayFormatOpenAIResponses:
		return constant.EndpointTypeOpenAIResponse
	case types.RelayFormatClaude:
		return constant.EndpointTypeAnthropic
	default:
		return ""
	}
}
