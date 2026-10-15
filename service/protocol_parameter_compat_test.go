package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/stretchr/testify/require"
)

func TestResponsesBridgeRejectsUnrepresentableParameters(t *testing.T) {
	for _, field := range []string{
		`"frequency_penalty":0`, `"presence_penalty":0`,
		`"prompt_cache_options":{"enabled":false}`, `"top_logprobs":0`,
	} {
		t.Run(field, func(t *testing.T) {
			var req dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(`{"model":"test","input":"hello",`+field+`}`), &req))
			require.Error(t, ValidateNormalizedBridgeRequest(&req, constant.EndpointTypeAnthropic))
			_, err := openaicompat.ResponsesRequestToClaudeRequest(&req)
			require.Error(t, err)
			require.NoError(t, ValidateNormalizedBridgeRequest(&req, constant.EndpointTypeOpenAIResponse))
			_, err = openaicompat.ResponsesRequestToChatCompletionsRequest(&req)
			if req.TopLogProbs != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChatResponsesRejectsUnsupportedLogprobs(t *testing.T) {
	for _, value := range []string{`"logprobs":true`, `"top_logprobs":0`} {
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.Unmarshal([]byte(`{"model":"test","messages":[{"role":"user","content":"hi"}],`+value+`}`), &req))
		require.Error(t, ValidateNormalizedBridgeRequest(&req, constant.EndpointTypeOpenAIResponse))
		_, err := openaicompat.ChatCompletionsRequestToResponsesRequest(&req)
		require.Error(t, err)
		require.NoError(t, ValidateNormalizedBridgeRequest(&req, constant.EndpointTypeOpenAI))
	}
}
