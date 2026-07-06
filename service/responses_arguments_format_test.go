package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeResponsesFunctionCallArgumentsObjectConvertsString(t *testing.T) {
	input, err := common.Marshal([]map[string]any{{
		"type":      "function_call",
		"call_id":   "call_1",
		"name":      "lookup",
		"arguments": `{"query":"weather"}`,
	}})
	require.NoError(t, err)
	req := &dto.OpenAIResponsesRequest{Input: input}

	changed, err := NormalizeResponsesFunctionCallArguments(req, dto.ResponsesFunctionCallArgumentsFormatObject)

	require.NoError(t, err)
	require.True(t, changed)
	var got []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &got))
	args, ok := got[0]["arguments"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "weather", args["query"])
}

func TestNormalizeResponsesFunctionCallArgumentsObjectRejectsInvalidString(t *testing.T) {
	input, err := common.Marshal([]map[string]any{{
		"type":      "function_call",
		"call_id":   "call_1",
		"name":      "lookup",
		"arguments": `not-json`,
	}})
	require.NoError(t, err)
	req := &dto.OpenAIResponsesRequest{Input: input}

	changed, err := NormalizeResponsesFunctionCallArguments(req, dto.ResponsesFunctionCallArgumentsFormatObject)

	require.Error(t, err)
	require.False(t, changed)
}

func TestNormalizeResponsesFunctionCallArgumentsStringConvertsObject(t *testing.T) {
	input, err := common.Marshal([]map[string]any{{
		"type":      "function_call",
		"call_id":   "call_1",
		"name":      "lookup",
		"arguments": map[string]any{"query": "weather"},
	}})
	require.NoError(t, err)
	req := &dto.OpenAIResponsesRequest{Input: input}

	changed, err := NormalizeResponsesFunctionCallArguments(req, dto.ResponsesFunctionCallArgumentsFormatString)

	require.NoError(t, err)
	require.True(t, changed)
	var got []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &got))
	require.Equal(t, `{"query":"weather"}`, got[0]["arguments"])
}

func TestEffectiveResponsesFunctionCallArgumentsFormat(t *testing.T) {
	require.Equal(
		t,
		dto.ResponsesFunctionCallArgumentsFormatString,
		EffectiveResponsesFunctionCallArgumentsFormat(constant.ChannelTypeOpenAI, dto.ChannelSettings{}, false),
	)
	require.Equal(
		t,
		dto.ResponsesFunctionCallArgumentsFormatObject,
		EffectiveResponsesFunctionCallArgumentsFormat(constant.ChannelTypeCodex, dto.ChannelSettings{}, false),
	)
	require.Equal(
		t,
		dto.ResponsesFunctionCallArgumentsFormatObject,
		EffectiveResponsesFunctionCallArgumentsFormat(constant.ChannelTypeOpenAI, dto.ChannelSettings{}, true),
	)
	require.True(t, ShouldEnforceResponsesFunctionCallArgumentsFormat(
		constant.ChannelTypeOpenAI,
		dto.ChannelSettings{ResponsesFunctionCallArgumentsFormat: dto.ResponsesFunctionCallArgumentsFormatString},
		false,
	))
	require.False(t, ShouldEnforceResponsesFunctionCallArgumentsFormat(constant.ChannelTypeOpenAI, dto.ChannelSettings{}, false))
}

func TestIsResponsesFunctionCallArgumentsObjectTypeError(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("Invalid type for 'input[8].arguments': expected an object, but got a string instead."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	require.True(t, IsResponsesFunctionCallArgumentsObjectTypeError(err))
}
