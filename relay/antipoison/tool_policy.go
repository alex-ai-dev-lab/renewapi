package antipoison

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

var (
	ErrToolCallNotDeclared = errors.New("tool call returned without declared tools")
	ErrToolCallNameDenied  = errors.New("tool call name is not declared")
	ErrToolCallBadArgs     = errors.New("tool call arguments are not a JSON object")
)

func CaptureOpenAIToolPolicy(info *relaycommon.RelayInfo, req *dto.GeneralOpenAIRequest) {
	if info == nil || req == nil {
		return
	}
	allowed := make(map[string]bool)
	for _, tool := range req.Tools {
		name := strings.TrimSpace(tool.Function.Name)
		if name != "" {
			allowed[name] = true
		}
	}
	if len(req.Functions) > 0 && common.GetJsonType(req.Functions) == "array" {
		var funcs []dto.FunctionRequest
		if err := common.Unmarshal(req.Functions, &funcs); err == nil {
			for _, fn := range funcs {
				name := strings.TrimSpace(fn.Name)
				if name != "" {
					allowed[name] = true
				}
			}
		}
	}
	info.AntiPoisonToolsDeclared = len(allowed) > 0
	if len(allowed) > 0 {
		info.AntiPoisonAllowedTools = allowed
	}
}

func CaptureResponsesToolPolicy(info *relaycommon.RelayInfo, req *dto.OpenAIResponsesRequest) {
	if info == nil || req == nil {
		return
	}
	allowed := make(map[string]bool)
	for _, item := range req.GetToolsMap() {
		name := strings.TrimSpace(common.Interface2String(item["name"]))
		if name == "" {
			name = strings.TrimSpace(common.Interface2String(item["function_name"]))
		}
		if name != "" {
			allowed[name] = true
		}
	}
	info.AntiPoisonToolsDeclared = len(allowed) > 0
	if len(allowed) > 0 {
		info.AntiPoisonAllowedTools = allowed
	}
}

func CaptureClaudeToolPolicy(info *relaycommon.RelayInfo, req *dto.ClaudeRequest) {
	if info == nil || req == nil {
		return
	}
	allowed := make(map[string]bool)
	for _, raw := range req.GetTools() {
		switch tool := raw.(type) {
		case *dto.Tool:
			name := strings.TrimSpace(tool.Name)
			if name != "" {
				allowed[name] = true
			}
		case dto.Tool:
			name := strings.TrimSpace(tool.Name)
			if name != "" {
				allowed[name] = true
			}
		case map[string]any:
			name := strings.TrimSpace(common.Interface2String(tool["name"]))
			if name != "" {
				allowed[name] = true
			}
		}
	}
	info.AntiPoisonToolsDeclared = len(allowed) > 0
	if len(allowed) > 0 {
		info.AntiPoisonAllowedTools = allowed
	}
}

func ValidateToolCallsAgainstPolicy(info *relaycommon.RelayInfo, toolNames []string, argPayloads []string) error {
	if info == nil {
		return nil
	}
	toolNames = compactNonEmpty(toolNames)
	if len(toolNames) == 0 {
		return nil
	}
	if !info.AntiPoisonToolsDeclared {
		return ErrToolCallNotDeclared
	}
	for _, name := range toolNames {
		if !info.AntiPoisonAllowedTools[name] {
			return fmt.Errorf("%w: %s", ErrToolCallNameDenied, name)
		}
	}
	for _, args := range argPayloads {
		args = strings.TrimSpace(args)
		if args == "" {
			continue
		}
		if common.GetJsonType(json.RawMessage(args)) != "object" {
			return fmt.Errorf("%w: expected JSON object", ErrToolCallBadArgs)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(args), &obj); err != nil {
			return fmt.Errorf("%w: %v", ErrToolCallBadArgs, err)
		}
	}
	return nil
}

func RecordToolPolicyFailure(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if c != nil {
		common.SetContextKey(c, constant.ContextKeyAntiPoisonToolGuardResult, ResultFail)
		RecordRisk(c, RiskHard, "tool_call_policy", "block")
	}
}

func OpenAIToolCallsFromResponse(resp *dto.OpenAITextResponse) (names []string, args []string) {
	if resp == nil {
		return nil, nil
	}
	for _, choice := range resp.Choices {
		for _, call := range toolCallsFromRaw(choice.Message.ToolCalls) {
			name := strings.TrimSpace(call.Function.Name)
			if name != "" {
				names = append(names, name)
				args = append(args, call.Function.Arguments)
			}
		}
	}
	return names, args
}

func ResponsesToolCallsFromResponse(resp *dto.OpenAIResponsesResponse) (names []string, args []string) {
	if resp == nil {
		return nil, nil
	}
	for _, out := range resp.Output {
		if out.Type != "function_call" {
			continue
		}
		name := strings.TrimSpace(out.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		args = append(args, out.ArgumentsString())
	}
	return names, args
}

func ClaudeToolCallsFromResponse(resp *dto.ClaudeResponse) (names []string, args []string) {
	if resp == nil {
		return nil, nil
	}
	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}
		name := strings.TrimSpace(block.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		if block.Input != nil {
			if b, err := common.Marshal(block.Input); err == nil {
				args = append(args, string(b))
			}
		}
	}
	return names, args
}

func toolCallsFromRaw(raw json.RawMessage) []dto.ToolCallResponse {
	if len(raw) == 0 || common.GetJsonType(raw) == "null" {
		return nil
	}
	var calls []dto.ToolCallResponse
	if err := common.Unmarshal(raw, &calls); err != nil {
		return nil
	}
	return calls
}
