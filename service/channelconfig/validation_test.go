package channelconfig

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestNormalizeModelEndpointsValidatesAndNormalizesInputs(t *testing.T) {
	channelType := constant.ChannelTypeOpenAI
	endpoints, err := NormalizeModelEndpoints(93, []ModelEndpointInput{
		{Model: " glm-5.2 ", BaseURL: " https://example.com/v1 ", ChannelType: &channelType},
		{Model: "GLM-5.2"},
	})
	require.NoError(t, err)
	require.Len(t, endpoints, 2)
	require.Equal(t, "glm-5.2", endpoints[0].Model)
	require.Equal(t, "https://example.com/v1", endpoints[0].BaseURL)
	require.Equal(t, "GLM-5.2", endpoints[1].Model, "model IDs remain case-sensitive")
}

func TestNormalizeModelEndpointsRejectsUnsafeOrAmbiguousInputs(t *testing.T) {
	unknownType := 999999
	for _, testCase := range []struct {
		name    string
		payload []ModelEndpointInput
		want    string
	}{
		{name: "duplicate", payload: []ModelEndpointInput{{Model: "a"}, {Model: " a "}}, want: "duplicate model endpoint"},
		{name: "relative URL", payload: []ModelEndpointInput{{Model: "a", BaseURL: "/v1"}}, want: "absolute HTTP(S) URL"},
		{name: "userinfo", payload: []ModelEndpointInput{{Model: "a", BaseURL: "https://user:pass@example.com"}}, want: "must not contain user credentials"},
		{name: "unknown channel type", payload: []ModelEndpointInput{{Model: "a", ChannelType: &unknownType}}, want: "unknown channel_type"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NormalizeModelEndpoints(93, testCase.payload)
			require.ErrorContains(t, err, testCase.want)
		})
	}
}

func TestValidateChannelRejectsUnsafeURLAndInvalidOrderedMapping(t *testing.T) {
	baseURL := "https://user:pass@example.com/v1"
	mapping := `{"glm-5.2":[]}`
	channel := &model.Channel{
		Type:         constant.ChannelTypeOpenAI,
		Key:          "sk-test",
		Name:         "channel",
		Models:       "glm-5.2",
		Group:        "default",
		BaseURL:      &baseURL,
		ModelMapping: &mapping,
	}
	require.ErrorContains(t, ValidateChannel(channel, true), "不能包含用户凭据")

	channel.BaseURL = nil
	require.ErrorContains(t, ValidateChannel(channel, true), "must not be empty")
}
