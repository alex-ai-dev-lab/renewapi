package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
)

// ResponsesUsageAccumulator 只收集本次尝试的用量事实；是否结算仍由提交门和协议结果决定。
type ResponsesUsageAccumulator struct {
	info                *relaycommon.RelayInfo
	usage               dto.Usage
	output              strings.Builder
	hasInput, hasOutput bool
	semantic, finished  bool
	arguments           map[string]string
}

func NewResponsesUsageAccumulator(info *relaycommon.RelayInfo) *ResponsesUsageAccumulator {
	return &ResponsesUsageAccumulator{info: info, arguments: make(map[string]string)}
}

func (a *ResponsesUsageAccumulator) Observe(data string) {
	if a == nil || a.finished || !gjson.Valid(data) {
		return
	}
	event := gjson.Parse(data)
	a.semantic = a.semantic || relaycommon.ClassifyStreamPayload(data).Semantic
	switch event.Get("type").String() {
	case "response.output_text.delta",
		"response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.refusal.delta":
		a.output.WriteString(event.Get("delta").String())
	case "response.function_call_arguments.delta":
		delta := event.Get("delta").String()
		a.arguments[event.Get("item_id").String()] += delta
		a.output.WriteString(delta)
	case "response.function_call_arguments.done":
		key := event.Get("item_id").String()
		arguments := event.Get("arguments").String()
		previous := a.arguments[key]
		if strings.HasPrefix(arguments, previous) {
			a.output.WriteString(arguments[len(previous):])
			a.arguments[key] = arguments
		}
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		if a.output.Len() == 0 {
			for _, item := range event.Get("response.output").Array() {
				a.output.WriteString(item.Get("arguments").String())
				for _, part := range item.Get("content").Array() {
					a.output.WriteString(part.Get("text").String())
					a.output.WriteString(part.Get("refusal").String())
				}
				for _, part := range item.Get("summary").Array() {
					a.output.WriteString(part.Get("text").String())
				}
			}
		}
	}
	if raw := event.Get("response.usage"); raw.IsObject() {
		var usage dto.Usage
		if common.UnmarshalJsonStr(raw.Raw, &usage) != nil {
			return
		}
		if raw.Get("input_tokens").Exists() {
			a.hasInput = true
			a.usage.PromptTokens = usage.InputTokens
		}
		if raw.Get("output_tokens").Exists() {
			a.hasOutput = true
			a.usage.CompletionTokens = usage.OutputTokens
		}
		if usage.InputTokensDetails != nil {
			a.usage.InputTokensDetails = usage.InputTokensDetails
			a.usage.PromptTokensDetails = *usage.InputTokensDetails
		}
		if usage.OutputTokensDetails != nil {
			a.usage.OutputTokensDetails = usage.OutputTokensDetails
			a.usage.CompletionTokenDetails = *usage.OutputTokensDetails
		}
	}
}

func (a *ResponsesUsageAccumulator) Finish() *dto.Usage {
	if a.finished {
		return &a.usage
	}
	a.finished = true
	if a.info != nil {
		if !a.hasOutput && a.output.Len() > 0 {
			a.usage.CompletionTokens = CountTextToken(a.output.String(), a.info.UpstreamModel())
		}
		// 前导事件没有交付价值，不因 response.created 产生输入估算或收费。
		if !a.hasInput && (a.semantic || a.usage.CompletionTokens > 0) {
			a.usage.PromptTokens = a.info.GetEstimatePromptTokens()
		}
		a.info.ResponsesObservedUsage = &a.usage
	}
	a.usage.InputTokens = a.usage.PromptTokens
	a.usage.OutputTokens = a.usage.CompletionTokens
	a.usage.TotalTokens = a.usage.PromptTokens + a.usage.CompletionTokens
	return &a.usage
}
