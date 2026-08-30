package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestClaudeAdaptorDefaultsBlankMessageRoleWithoutMutatingCaller(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "claude-test",
		Messages: []dto.Message{
			{Content: "hello"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Len(t, claudeRequest.Messages, 1)
	require.Equal(t, "user", claudeRequest.Messages[0].Role)
	require.Equal(t, "", request.Messages[0].Role)
}

func TestClaudeAdaptorPreservesParameterlessFunctionTool(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:    "claude-test",
		Messages: []dto.Message{{Role: "user", Content: "ping"}},
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name: "ping",
			},
		}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	tools, ok := claudeRequest.Tools.([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(*dto.Tool)
	require.True(t, ok)
	require.Equal(t, "ping", tool.Name)
	require.Equal(t, "object", tool.InputSchema["type"])
	require.Equal(t, map[string]any{}, tool.InputSchema["properties"])
	require.Equal(t, []string{}, tool.InputSchema["required"])
	require.Nil(t, request.Tools[0].Function.Parameters)
}

func TestClaudeAdaptorNormalizesOmittedObjectSchemaFieldsWithoutMutatingCaller(t *testing.T) {
	parameters := map[string]any{"type": "object"}
	request := &dto.GeneralOpenAIRequest{
		Model:    "claude-test",
		Messages: []dto.Message{{Role: "user", Content: "ping"}},
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:       "ping",
				Parameters: parameters,
			},
		}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	tools, ok := claudeRequest.Tools.([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(*dto.Tool)
	require.True(t, ok)
	require.Equal(t, map[string]any{}, tool.InputSchema["properties"])
	require.Equal(t, []string{}, tool.InputSchema["required"])

	_, callerHasProperties := parameters["properties"]
	_, callerHasRequired := parameters["required"]
	require.False(t, callerHasProperties)
	require.False(t, callerHasRequired)
}
