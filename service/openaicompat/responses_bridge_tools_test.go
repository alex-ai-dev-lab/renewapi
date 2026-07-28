package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesBridgeToolsLiftFlattenRewriteAndRestore(t *testing.T) {
	input, _ := common.Marshal([]any{
		map[string]any{"type": "additional_tools", "tools": []any{
			map[string]any{"type": "namespace", "name": "team", "tools": []any{
				map[string]any{"type": "function", "name": "send", "description": "send", "parameters": map[string]any{"type": "object"}},
			}},
		}},
		map[string]any{"type": "function_call", "call_id": "call_1", "namespace": "team", "name": "send", "arguments": `{}`},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
	})
	choice, _ := common.Marshal(map[string]any{"type": "function", "namespace": "team", "name": "send"})
	req := &dto.OpenAIResponsesRequest{Model: "test", Input: input, ToolChoice: choice}

	chat, mapping, err := ResponsesRequestToChatCompletionsRequestWithMapping(req)
	require.NoError(t, err)
	require.Len(t, chat.Tools, 1)
	require.Equal(t, "team__send", chat.Tools[0].Function.Name)
	require.Len(t, chat.Messages, 2, "additional_tools must not become a chat message")
	require.Equal(t, "team__send", chat.Messages[0].ParseToolCalls()[0].Function.Name)
	require.Equal(t, "team__send", chat.ToolChoice.(map[string]any)["function"].(map[string]any)["name"])
	require.Equal(t, ResponsesNamespaceToolName{Namespace: "team", Name: "send"}, mapping.NamespaceTools["team__send"])

	response := &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{{Type: "function_call", Name: "team__send"}}}
	RestoreResponsesBridgeOutput(response, mapping)
	require.Equal(t, "send", response.Output[0].Name)
	require.Equal(t, "team", response.Output[0].Namespace)

	var originalInput []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &originalInput))
	require.Equal(t, "additional_tools", originalInput[0]["type"], "bridge preparation must not mutate the parsed client request")
}

func TestResponsesBridgeFunctionOutputArrayPreservesParts(t *testing.T) {
	input, _ := common.Marshal([]any{map[string]any{
		"type": "function_call_output", "call_id": "call_1", "output": []any{
			map[string]any{"type": "input_text", "text": "image loaded"},
			map[string]any{"type": "input_image", "image_url": "data:image/png;base64,YQ=="},
		},
	}})
	chat, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{Model: "test", Input: input})
	require.NoError(t, err)
	require.Len(t, chat.Messages, 1)
	parts := chat.Messages[0].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, "image loaded", parts[0].Text)
	require.Equal(t, "data:image/png;base64,YQ==", parts[1].ImageUrl)
}

func TestResponsesBridgeRejectsNamespaceCollision(t *testing.T) {
	tools, _ := common.Marshal([]any{
		map[string]any{"type": "function", "name": "team__send"},
		map[string]any{"type": "namespace", "name": "team", "tools": []any{map[string]any{"type": "function", "name": "send"}}},
	})
	_, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{Model: "test", Tools: tools})
	require.ErrorContains(t, err, "collides")
}

func TestResponsesBridgeToolChoicePolicy(t *testing.T) {
	auto, _ := common.Marshal("auto")
	chat, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{Model: "test", ToolChoice: auto})
	require.NoError(t, err)
	require.Nil(t, chat.ToolChoice, "orphan auto choice must not be sent after tools are removed")

	required, _ := common.Marshal(map[string]any{"type": "function", "name": "missing"})
	_, err = ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{Model: "test", ToolChoice: required})
	require.ErrorContains(t, err, "requires at least one")
}
