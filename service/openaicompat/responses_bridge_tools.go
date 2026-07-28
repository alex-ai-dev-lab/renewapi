package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

const responsesBridgeToolMappingContextKey = "responses_bridge_tool_mapping"

type ResponsesNamespaceToolName struct {
	Namespace string
	Name      string
}

// ResponsesBridgeToolMapping contains only request-local, reversible bridge
// state. It must never be applied to native Responses pass-through traffic.
type ResponsesBridgeToolMapping struct {
	NamespaceTools map[string]ResponsesNamespaceToolName
}

func SetResponsesBridgeToolMapping(c *gin.Context, mapping ResponsesBridgeToolMapping) {
	if c == nil {
		return
	}
	c.Set(responsesBridgeToolMappingContextKey, mapping)
}

func GetResponsesBridgeToolMapping(c *gin.Context) ResponsesBridgeToolMapping {
	if c == nil {
		return ResponsesBridgeToolMapping{}
	}
	value, ok := c.Get(responsesBridgeToolMappingContextKey)
	if !ok {
		return ResponsesBridgeToolMapping{}
	}
	mapping, _ := value.(ResponsesBridgeToolMapping)
	return mapping
}

func PrepareResponsesRequestForTextBridge(req *dto.OpenAIResponsesRequest) (*dto.OpenAIResponsesRequest, ResponsesBridgeToolMapping, error) {
	if req == nil {
		return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("responses request is nil")
	}
	prepared, err := common.DeepCopy(req)
	if err != nil {
		return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("copy Responses bridge request: %w", err)
	}

	tools, input, err := effectiveResponsesBridgeTools(prepared.Tools, prepared.Input)
	if err != nil {
		return nil, ResponsesBridgeToolMapping{}, err
	}
	flattened, mapping, err := flattenResponsesNamespaceTools(tools)
	if err != nil {
		return nil, ResponsesBridgeToolMapping{}, err
	}
	rewriteNamespaceQualifiedCalls(input, mapping.NamespaceTools)
	if err := normalizeResponsesBridgeToolChoice(prepared, flattened, mapping.NamespaceTools); err != nil {
		return nil, ResponsesBridgeToolMapping{}, err
	}

	prepared.Input, err = common.Marshal(input)
	if err != nil {
		return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("encode Responses bridge input: %w", err)
	}
	if len(flattened) == 0 {
		prepared.Tools = nil
	} else {
		prepared.Tools, err = common.Marshal(flattened)
		if err != nil {
			return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("encode Responses bridge tools: %w", err)
		}
	}
	return prepared, mapping, nil
}

func effectiveResponsesBridgeTools(toolsRaw, inputRaw json.RawMessage) ([]any, any, error) {
	var tools []any
	if len(toolsRaw) != 0 {
		if err := common.Unmarshal(toolsRaw, &tools); err != nil {
			return nil, nil, fmt.Errorf("invalid Responses tools: %w", err)
		}
	}
	if len(inputRaw) == 0 || common.GetJsonType(inputRaw) == "string" {
		var scalar any
		if len(inputRaw) != 0 {
			if err := common.Unmarshal(inputRaw, &scalar); err != nil {
				return nil, nil, fmt.Errorf("invalid Responses input: %w", err)
			}
		}
		return tools, scalar, nil
	}
	if common.GetJsonType(inputRaw) != "array" {
		return nil, nil, fmt.Errorf("Responses input must be a string or array for the text bridge")
	}
	var input []any
	if err := common.Unmarshal(inputRaw, &input); err != nil {
		return nil, nil, fmt.Errorf("invalid Responses input: %w", err)
	}
	kept := make([]any, 0, len(input))
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(common.Interface2String(item["type"])) != "additional_tools" {
			kept = append(kept, raw)
			continue
		}
		additional, ok := item["tools"].([]any)
		if !ok {
			return nil, nil, fmt.Errorf("Responses additional_tools item must contain a tools array")
		}
		tools = append(tools, additional...)
	}
	return tools, kept, nil
}

func flattenResponsesNamespaceTools(tools []any) ([]any, ResponsesBridgeToolMapping, error) {
	mapping := ResponsesBridgeToolMapping{NamespaceTools: make(map[string]ResponsesNamespaceToolName)}
	declared := make(map[string]bool)
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses tool must be an object")
		}
		typ := strings.TrimSpace(common.Interface2String(tool["type"]))
		if typ == "" {
			typ = "function"
		}
		if typ == "function" {
			name := strings.TrimSpace(common.Interface2String(tool["name"]))
			if name == "" {
				return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses function tool name is required")
			}
			if declared[name] {
				return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses tool name %q is ambiguous", name)
			}
			declared[name] = true
		}
	}

	flattened := make([]any, 0, len(tools))
	for _, raw := range tools {
		tool := raw.(map[string]any)
		typ := strings.TrimSpace(common.Interface2String(tool["type"]))
		if typ == "" || typ == "function" {
			flattened = append(flattened, tool)
			continue
		}
		if typ != "namespace" {
			return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses tool type %q is not supported by the safe text bridge", typ)
		}
		namespace := strings.TrimSpace(common.Interface2String(tool["name"]))
		if namespace == "" {
			return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses namespace tool name is required")
		}
		children, ok := tool["tools"].([]any)
		if !ok || len(children) == 0 {
			return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses namespace %q must contain a non-empty tools array", namespace)
		}
		for _, childRaw := range children {
			child, ok := childRaw.(map[string]any)
			if !ok || strings.TrimSpace(common.Interface2String(child["type"])) != "function" {
				return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses namespace %q contains a non-function tool", namespace)
			}
			name := strings.TrimSpace(common.Interface2String(child["name"]))
			if name == "" {
				return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses namespace %q contains an unnamed function", namespace)
			}
			flat := namespace + "__" + name
			if declared[flat] || mapping.NamespaceTools[flat].Name != "" {
				return nil, ResponsesBridgeToolMapping{}, fmt.Errorf("Responses namespace tool %q collides with another tool", flat)
			}
			copyTool := make(map[string]any, len(child))
			for key, value := range child {
				copyTool[key] = value
			}
			copyTool["name"] = flat
			flattened = append(flattened, copyTool)
			mapping.NamespaceTools[flat] = ResponsesNamespaceToolName{Namespace: namespace, Name: name}
		}
	}
	if len(mapping.NamespaceTools) == 0 {
		mapping.NamespaceTools = nil
	}
	return flattened, mapping, nil
}

func rewriteNamespaceQualifiedCalls(value any, names map[string]ResponsesNamespaceToolName) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			rewriteNamespaceQualifiedCalls(child, names)
		}
	case map[string]any:
		if strings.TrimSpace(common.Interface2String(typed["type"])) == "function_call" {
			namespace := strings.TrimSpace(common.Interface2String(typed["namespace"]))
			name := strings.TrimSpace(common.Interface2String(typed["name"]))
			flat := namespace + "__" + name
			if expected, ok := names[flat]; ok && expected.Namespace == namespace && expected.Name == name {
				typed["name"] = flat
				delete(typed, "namespace")
			}
		}
		for _, child := range typed {
			rewriteNamespaceQualifiedCalls(child, names)
		}
	}
}

func normalizeResponsesBridgeToolChoice(req *dto.OpenAIResponsesRequest, tools []any, names map[string]ResponsesNamespaceToolName) error {
	if len(req.ToolChoice) == 0 {
		return nil
	}
	if len(tools) == 0 {
		if common.GetJsonType(req.ToolChoice) == "string" {
			var choice string
			if err := common.Unmarshal(req.ToolChoice, &choice); err == nil && (choice == "auto" || choice == "none") {
				req.ToolChoice = nil
				return nil
			}
		}
		return fmt.Errorf("Responses tool_choice requires at least one bridge-compatible tool")
	}
	if common.GetJsonType(req.ToolChoice) != "object" {
		return nil
	}
	var choice map[string]any
	if err := common.Unmarshal(req.ToolChoice, &choice); err != nil {
		return fmt.Errorf("invalid Responses tool_choice: %w", err)
	}
	if strings.TrimSpace(common.Interface2String(choice["type"])) != "function" {
		return nil
	}
	name := strings.TrimSpace(common.Interface2String(choice["name"]))
	namespace := strings.TrimSpace(common.Interface2String(choice["namespace"]))
	if namespace != "" {
		flat := namespace + "__" + name
		if expected, ok := names[flat]; !ok || expected.Namespace != namespace || expected.Name != name {
			return fmt.Errorf("Responses tool_choice references unknown namespace tool %q", flat)
		}
		choice["name"] = flat
		delete(choice, "namespace")
		name = flat
	}
	known := false
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if strings.TrimSpace(common.Interface2String(tool["name"])) == name {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("Responses tool_choice references unknown function %q", name)
	}
	var err error
	req.ToolChoice, err = common.Marshal(choice)
	return err
}

func RestoreResponsesBridgeOutput(resp *dto.OpenAIResponsesResponse, mapping ResponsesBridgeToolMapping) {
	if resp == nil || len(mapping.NamespaceTools) == 0 {
		return
	}
	for i := range resp.Output {
		output := &resp.Output[i]
		if output.Type != "function_call" {
			continue
		}
		if original, ok := mapping.NamespaceTools[output.Name]; ok {
			output.Name = original.Name
			output.Namespace = original.Namespace
		}
	}
}

func RestoreResponsesBridgeOutputFromContext(c *gin.Context, resp *dto.OpenAIResponsesResponse) {
	RestoreResponsesBridgeOutput(resp, GetResponsesBridgeToolMapping(c))
}
