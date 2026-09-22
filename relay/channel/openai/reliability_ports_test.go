package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	service.InitTokenEncoders()
	os.Exit(m.Run())
}

func TestResponsesTruncationRetainsUsageWithoutSettling(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{IsStream: true, DisablePing: true, RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
	info.SetEstimatePromptTokens(23)
	body := "data: " + `{"type":"response.function_call_arguments.delta","delta":"{\"city\":\"Taipei\"}"}` + "\n\n"
	usage, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))})
	require.NotNil(t, apiErr)
	require.Nil(t, usage)
	require.NotNil(t, info.ResponsesObservedUsage)
	require.Equal(t, 23, info.ResponsesObservedUsage.PromptTokens)
	require.Positive(t, info.ResponsesObservedUsage.CompletionTokens)
}

func TestResponsesErrorEventsRetainStatusBeforeCommit(t *testing.T) {
	for name, handler := range map[string]func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError){
		"responses": OaiResponsesStreamHandler,
		"chat":      OaiResponsesToChatStreamHandler,
	} {
		t.Run(name, func(t *testing.T) {
			for _, event := range []struct {
				payload string
				status  int
			}{
				{`{"type":"error","status":429,"error":{"message":"private native failure","code":"rate_limit_exceeded"}}`, 429},
				{`{"type":"error","status":503,"message":"private native failure","code":"server_error"}`, 503},
			} {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				info := &relaycommon.RelayInfo{IsStream: true, DisablePing: true, RelayFormat: types.RelayFormatOpenAIResponses,
					ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
				usage, apiErr := handler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: " + event.payload + "\n\n"))})
				require.Nil(t, usage)
				require.NotNil(t, apiErr)
				require.Equal(t, event.status, apiErr.StatusCode)
				require.Equal(t, "private native failure", apiErr.Error())
				require.Empty(t, recorder.Body.String())
			}
		})
	}
}

func TestCrossProtocolGeminiRequestsUsageOnlyWhenSupported(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, supported := range []bool{false, true} {
			info := &relaycommon.RelayInfo{IsStream: streaming, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, SupportStreamOptions: supported, UpstreamModelName: "gpt-4o"}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			request := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}}}
			converted, err := (&Adaptor{}).ConvertGeminiRequest(c, info, request)
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)
			if streaming && supported {
				require.Contains(t, string(body), `"include_usage":true`)
			} else {
				require.NotContains(t, string(body), "include_usage")
			}
		}
	}
}

func TestResponsesArgumentsDoneCompletesWithoutDuplicatingToolCall(t *testing.T) {
	for _, delta := range []string{"", "{\"q\":"} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		info := &relaycommon.RelayInfo{IsStream: true, DisablePing: true, RelayFormat: types.RelayFormatOpenAI,
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"}}
		data, err := common.Marshal(map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "delta": delta})
		require.NoError(t, err)
		body := strings.Join([]string{
			`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}`,
			"data: " + string(data),
			`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{\"q\":\"x\"}"}`,
			`data: {"type":"response.completed","response":{"status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":3}}}`,
			"",
		}, "\n\n")
		_, apiErr := OaiResponsesToChatStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))})
		require.Nil(t, apiErr)
		var arguments strings.Builder
		for _, frame := range strings.Split(recorder.Body.String(), "\n\n") {
			var chunk dto.ChatCompletionsStreamResponse
			if common.UnmarshalJsonStr(strings.TrimPrefix(frame, "data: "), &chunk) != nil {
				continue
			}
			for _, choice := range chunk.Choices {
				for _, tool := range choice.Delta.ToolCalls {
					arguments.WriteString(tool.Function.Arguments)
				}
			}
		}
		require.Equal(t, `{"q":"x"}`, arguments.String())
	}
}
