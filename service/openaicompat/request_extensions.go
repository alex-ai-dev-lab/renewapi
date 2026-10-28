package openaicompat

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func rejectExtensions(extra dto.RequestExtensions, path string, allowed ...string) error {
	names := make([]string, 0, len(extra.ExtraFields))
	for name := range extra.ExtraFields {
		found := false
		for _, key := range allowed {
			if name == key {
				found = true
				break
			}
		}
		if !found {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return unsupportedProtocolConversion("%s.%s has no supported representation in the target protocol", path, names[0])
	}
	return nil
}

func ValidateRequestExtensions(extra dto.RequestExtensions, path string, allowed ...string) error {
	return rejectExtensions(extra, path, allowed...)
}

// ValidateResponsesBridgeParameters rejects parameters whose response or request
// semantics the destination bridge cannot retain. Native forwarding is unaffected.
func ValidateResponsesBridgeParameters(req *dto.OpenAIResponsesRequest, target constant.EndpointType) error {
	if req == nil || target == constant.EndpointTypeOpenAIResponse {
		return nil
	}
	if req.TopLogProbs != nil {
		return fmt.Errorf("Responses field top_logprobs is not supported by the %s response bridge", target)
	}
	if target == constant.EndpointTypeAnthropic {
		for _, field := range []struct {
			name string
			raw  common.RawMessage
		}{
			{"frequency_penalty", req.FrequencyPenalty},
			{"presence_penalty", req.PresencePenalty},
			{"prompt_cache_options", req.PromptCacheOptions},
		} {
			if len(field.raw) > 0 && string(field.raw) != "null" {
				return fmt.Errorf("Responses field %s cannot be represented by Messages", field.name)
			}
		}
	}
	return nil
}

func copyChatResponsesFields(req *dto.GeneralOpenAIRequest, out *dto.OpenAIResponsesRequest) error {
	if req.TopLogProbs != nil || req.LogProbs != nil && *req.LogProbs {
		return fmt.Errorf("Chat logprobs are not supported by the Responses response bridge")
	}
	if err := rejectExtensions(req.RequestExtensions, "request", "include", "client_metadata", "prompt_cache_options"); err != nil {
		return err
	}
	for i, message := range req.Messages {
		if err := rejectExtensions(message.RequestExtensions, fmt.Sprintf("messages[%d]", i)); err != nil {
			return err
		}
	}
	out.Include = req.ExtraFields["include"]
	out.PromptCacheOptions = req.ExtraFields["prompt_cache_options"]
	if raw, ok := req.ExtraFields["client_metadata"]; ok {
		out.ExtraFields = map[string]common.RawMessage{"client_metadata": raw}
	}
	if req.PromptCacheKey != "" {
		out.PromptCacheKey, _ = common.Marshal(req.PromptCacheKey)
	}
	out.PromptCacheRetention = req.PromptCacheRetention
	out.SafetyIdentifier = req.SafetyIdentifier
	if len(req.ServiceTier) > 0 {
		if err := common.Unmarshal(req.ServiceTier, &out.ServiceTier); err != nil {
			return fmt.Errorf("invalid service_tier: %w", err)
		}
	}
	if len(req.Reasoning) > 0 {
		if err := common.Unmarshal(req.Reasoning, &out.Reasoning); err != nil {
			return fmt.Errorf("invalid reasoning: %w", err)
		}
	}
	if len(req.Verbosity) > 0 {
		text := map[string]any{}
		if len(out.Text) > 0 {
			if err := common.Unmarshal(out.Text, &text); err != nil {
				return err
			}
		}
		text["verbosity"] = req.Verbosity
		out.Text, _ = common.Marshal(text)
	}
	return nil
}
