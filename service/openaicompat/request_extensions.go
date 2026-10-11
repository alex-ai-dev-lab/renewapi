package openaicompat

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
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
		return fmt.Errorf("%s.%s has no supported representation in the target protocol", path, names[0])
	}
	return nil
}

func ValidateRequestExtensions(extra dto.RequestExtensions, path string, allowed ...string) error {
	return rejectExtensions(extra, path, allowed...)
}

func copyChatResponsesFields(req *dto.GeneralOpenAIRequest, out *dto.OpenAIResponsesRequest) error {
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
