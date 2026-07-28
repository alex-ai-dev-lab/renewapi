package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestNormalizeResponsesRequestForwardsCompactCacheFields(t *testing.T) {
	compact := &dto.OpenAIResponsesCompactionRequest{
		Model:                "gpt-5.4",
		PromptCacheKey:       []byte(`"cache-key"`),
		PromptCacheOptions:   []byte(`{"retention":"24h"}`),
		PromptCacheRetention: []byte(`"24h"`),
		ParallelToolCalls:    []byte("true"),
		ServiceTier:          "priority",
	}
	request, err := normalizeResponsesRequest(compact)
	require.NoError(t, err)
	require.JSONEq(t, `"cache-key"`, string(request.PromptCacheKey))
	require.JSONEq(t, `{"retention":"24h"}`, string(request.PromptCacheOptions))
	require.JSONEq(t, `"24h"`, string(request.PromptCacheRetention))
	require.JSONEq(t, "true", string(request.ParallelToolCalls))
	require.Equal(t, "priority", request.ServiceTier)
}
