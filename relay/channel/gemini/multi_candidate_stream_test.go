package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiAdaptorStreamKeepsToolCallsAndFinishReasonsPerCandidate(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	n := 2
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
		Request:     &dto.GeneralOpenAIRequest{N: &n},
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}
	stop := "STOP"
	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index:        0,
				FinishReason: &stop,
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: "plain"}},
				},
			},
			{
				Index:        1,
				FinishReason: &stop,
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{{
						FunctionCall: &dto.FunctionCall{
							FunctionName: "lookup",
							Arguments:    map[string]interface{}{"city": "Paris"},
						},
					}},
				},
			},
		},
	}
	data, err := common.Marshal(payload)
	require.NoError(t, err)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("data: " + string(data) + "\n\n")),
	}

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	var seenChoice0Stop bool
	var seenChoice1ToolFinish bool
	var seenChoice1ToolCall bool
	var badChoice0ToolFinish bool
	for _, line := range strings.Split(w.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Index == 0 && choice.FinishReason != nil {
				if *choice.FinishReason == "tool_calls" {
					badChoice0ToolFinish = true
				}
				if *choice.FinishReason == "stop" {
					seenChoice0Stop = true
				}
			}
			if choice.Index == 1 {
				if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
					seenChoice1ToolFinish = true
				}
				if len(choice.Delta.ToolCalls) > 0 && choice.Delta.ToolCalls[0].Function.Name == "lookup" {
					seenChoice1ToolCall = true
				}
			}
		}
	}
	require.True(t, seenChoice0Stop)
	require.True(t, seenChoice1ToolFinish)
	require.True(t, seenChoice1ToolCall)
	require.False(t, badChoice0ToolFinish)
}
