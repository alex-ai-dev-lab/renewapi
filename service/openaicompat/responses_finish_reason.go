package openaicompat

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// ResponsesFinishReason maps a Responses terminal result to Chat Completions semantics.
// Only map reasons with an unambiguous Chat Completions equivalent; unknown reasons
// preserve the historical stop/tool_calls behavior.
func ResponsesFinishReason(resp *dto.OpenAIResponsesResponse, hasToolCalls bool) string {
	if resp != nil &&
		strings.EqualFold(strings.TrimSpace(rawText(resp.Status)), "incomplete") &&
		resp.IncompleteDetails != nil &&
		strings.EqualFold(strings.TrimSpace(resp.IncompleteDetails.Reasoning), "max_output_tokens") {
		return "length"
	}
	if hasToolCalls {
		return "tool_calls"
	}
	return "stop"
}
