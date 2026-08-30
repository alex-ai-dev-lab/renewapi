package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeAdaptorOpus47ReasoningEffortUsesAdaptiveThinking(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request := &dto.GeneralOpenAIRequest{
		Model:           "claude-opus-4-7",
		ReasoningEffort: "high",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Equal(t, "claude-opus-4-7", claudeRequest.Model)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "adaptive", claudeRequest.Thinking.Type)
	require.Equal(t, "summarized", claudeRequest.Thinking.Display)
	require.JSONEq(t, `{"effort":"high"}`, string(claudeRequest.OutputConfig))

	// Request conversion must not mutate the caller's model or explicit effort.
	require.Equal(t, "claude-opus-4-7", request.Model)
	require.Equal(t, "high", request.ReasoningEffort)
}

func TestClaudeAdaptorOpus47OmitsUnsupportedSamplingParams(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	temperature := 0.4
	topP := 0.8
	topK := 20

	request := &dto.GeneralOpenAIRequest{
		Model:       "claude-opus-4-7",
		Temperature: &temperature,
		TopP:        &topP,
		TopK:        &topK,
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.Nil(t, claudeRequest.Temperature)
	require.Nil(t, claudeRequest.TopP)
	require.Nil(t, claudeRequest.TopK)

	// Request conversion must not mutate caller-owned sampling parameters.
	require.NotNil(t, request.Temperature)
	require.NotNil(t, request.TopP)
	require.NotNil(t, request.TopK)
}

func TestClaudeAdaptorOpus46ReasoningEffortKeepsManualThinking(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request := &dto.GeneralOpenAIRequest{
		Model:           "claude-opus-4-6",
		ReasoningEffort: "high",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, request)
	require.NoError(t, err)
	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, claudeRequest.Thinking)
	require.Equal(t, "enabled", claudeRequest.Thinking.Type)
	require.NotNil(t, claudeRequest.Thinking.BudgetTokens)
	require.Equal(t, 4096, *claudeRequest.Thinking.BudgetTokens)
}
