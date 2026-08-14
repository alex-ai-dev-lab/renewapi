package openaicompat

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatResponsesPenaltyRoundTrip(t *testing.T) {
	for _, value := range []float64{0, -0.75, 1.25} {
		t.Run(valueString(value), func(t *testing.T) {
			chat := &dto.GeneralOpenAIRequest{
				Model:            "test-model",
				Messages:         []dto.Message{{Role: "user", Content: "hello"}},
				FrequencyPenalty: &value,
				PresencePenalty:  &value,
			}
			responses, err := ChatCompletionsRequestToResponsesRequest(chat)
			require.NoError(t, err)
			require.JSONEq(t, valueString(value), string(responses.FrequencyPenalty))
			require.JSONEq(t, valueString(value), string(responses.PresencePenalty))

			roundTrip, err := ResponsesRequestToChatCompletionsRequest(responses)
			require.NoError(t, err)
			require.NotNil(t, roundTrip.FrequencyPenalty)
			require.NotNil(t, roundTrip.PresencePenalty)
			require.Equal(t, value, *roundTrip.FrequencyPenalty)
			require.Equal(t, value, *roundTrip.PresencePenalty)
		})
	}
}

func TestResponsesPenaltyRejectsNonNumber(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model:            "test-model",
		Input:            []byte(`"hello"`),
		FrequencyPenalty: []byte(`"0.5"`),
	})
	require.ErrorContains(t, err, "invalid frequency_penalty")
}

func valueString(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
