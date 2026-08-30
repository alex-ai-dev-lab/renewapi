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
