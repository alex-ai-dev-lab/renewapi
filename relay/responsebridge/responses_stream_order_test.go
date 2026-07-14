package responsebridge

import (
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

type capturedResponsesEvent struct {
	Type        string                       `json:"type"`
	OutputIndex *int                         `json:"output_index"`
	ItemID      string                       `json:"item_id"`
	Delta       string                       `json:"delta"`
	Item        dto.ResponsesOutput          `json:"item"`
	Response    *dto.OpenAIResponsesResponse `json:"response"`
}

func newResponsesEmitterTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set(common.RequestIdKey, "bridge-order-test")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, OriginModelName: "test-model"}
	return c, recorder, info
}

func captureResponsesEvents(t *testing.T, body string) []capturedResponsesEvent {
	t.Helper()
	events := make([]capturedResponsesEvent, 0)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event capturedResponsesEvent
		require.NoError(t, common.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event))
		events = append(events, event)
	}
	return events
}

func toolStreamChunk(index int, id, name, arguments string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id: "chatcmpl-order", Created: 123, Model: "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{{
				Index: &index, ID: id, Type: "function",
				Function: dto.FunctionResponse{Name: name, Arguments: arguments},
			}}},
		}},
	}
}

func textStreamChunk(text string) *dto.ChatCompletionsStreamResponse {
	chunk := &dto.ChatCompletionsStreamResponse{
		Id: "chatcmpl-order", Created: 123, Model: "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"},
		}},
	}
	chunk.Choices[0].Delta.SetContentString(text)
	return chunk
}

func eventsOfType(events []capturedResponsesEvent, eventType string) []capturedResponsesEvent {
	result := make([]capturedResponsesEvent, 0)
	for _, event := range events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

func requireOutputIndex(t *testing.T, event capturedResponsesEvent, expected int) {
	t.Helper()
	require.NotNil(t, event.OutputIndex)
	require.Equal(t, expected, *event.OutputIndex)
}

func TestResponsesStreamEmitterKeepsToolFirstIndexesAndIDsStable(t *testing.T) {
	c, recorder, info := newResponsesEmitterTestContext(t)
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(0, "call_1", "lookup", `{"id":1}`)))
	require.NoError(t, EmitChatChunk(c, info, textStreamChunk("hello")))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{}, "tool_calls"))

	events := captureResponsesEvents(t, recorder.Body.String())
	added := eventsOfType(events, "response.output_item.added")
	require.Len(t, added, 2)
	requireOutputIndex(t, added[0], 0)
	require.Equal(t, "function_call", added[0].Item.Type)
	require.Equal(t, "fc_call_1", added[0].Item.ID)
	require.Equal(t, "call_1", added[0].Item.CallId)
	requireOutputIndex(t, added[1], 1)
	require.Equal(t, "message", added[1].Item.Type)
	require.Equal(t, "msg_order", added[1].Item.ID)

	done := eventsOfType(events, "response.output_item.done")
	require.Len(t, done, 2)
	requireOutputIndex(t, done[0], 1)
	require.Equal(t, "msg_order", done[0].Item.ID)
	requireOutputIndex(t, done[1], 0)
	require.Equal(t, "fc_call_1", done[1].Item.ID)

	completed := eventsOfType(events, "response.completed")
	require.Len(t, completed, 1)
	require.NotNil(t, completed[0].Response)
	require.Len(t, completed[0].Response.Output, 2)
	require.Equal(t, "fc_call_1", completed[0].Response.Output[0].ID)
	require.Equal(t, "msg_order", completed[0].Response.Output[1].ID)
}

func TestResponsesStreamEmitterBuffersArgumentsUntilIdentityIsStable(t *testing.T) {
	c, recorder, info := newResponsesEmitterTestContext(t)
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(7, "", "", `{"id":`)))
	emitter := getEmitter(c, info, nil)
	require.False(t, emitter.Tools[7].Started)
	require.Equal(t, 0, emitter.NextOutputIndex)
	require.NotContains(t, recorder.Body.String(), "response.function_call_arguments.delta")

	require.NoError(t, EmitChatChunk(c, info, textStreamChunk("hello")))
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(7, "call_7", "lookup", `1}`)))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{}, "tool_calls"))

	events := captureResponsesEvents(t, recorder.Body.String())
	deltas := eventsOfType(events, "response.function_call_arguments.delta")
	require.Len(t, deltas, 1)
	requireOutputIndex(t, deltas[0], 1)
	require.Equal(t, `{"id":1}`, deltas[0].Delta)
	require.Equal(t, "fc_call_7", deltas[0].ItemID)

	completed := eventsOfType(events, "response.completed")
	require.Len(t, completed, 1)
	require.Equal(t, "message", completed[0].Response.Output[0].Type)
	require.Equal(t, "function_call", completed[0].Response.Output[1].Type)
	require.Equal(t, "call_7", completed[0].Response.Output[1].CallId)
}

func TestResponsesStreamEmitterHandlesInterleavedNonContiguousTools(t *testing.T) {
	c, recorder, info := newResponsesEmitterTestContext(t)
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(5, "call_b", "second", `{"b":`)))
	require.NoError(t, EmitChatChunk(c, info, textStreamChunk("middle")))
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(2, "call_a", "first", `{}`)))
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(5, "", "", `1}`)))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{}, "tool_calls"))

	events := captureResponsesEvents(t, recorder.Body.String())
	completed := eventsOfType(events, "response.completed")
	require.Len(t, completed, 1)
	output := completed[0].Response.Output
	require.Len(t, output, 3)
	require.Equal(t, "call_b", output[0].CallId)
	require.Equal(t, "message", output[1].Type)
	require.Equal(t, "call_a", output[2].CallId)

	deltas := eventsOfType(events, "response.function_call_arguments.delta")
	require.Len(t, deltas, 3)
	requireOutputIndex(t, deltas[0], 0)
	requireOutputIndex(t, deltas[1], 2)
	requireOutputIndex(t, deltas[2], 0)
}

func TestResponsesStreamEmitterCompletesToolOnlyWithoutIdentity(t *testing.T) {
	c, recorder, info := newResponsesEmitterTestContext(t)
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(9, "", "", `{}`)))
	require.NoError(t, CompleteChatStream(c, info, &dto.Usage{}, "tool_calls"))

	events := captureResponsesEvents(t, recorder.Body.String())
	completed := eventsOfType(events, "response.completed")
	require.Len(t, completed, 1)
	require.Len(t, completed[0].Response.Output, 1)
	require.Equal(t, "call_9", completed[0].Response.Output[0].CallId)
	require.Equal(t, "fc_call_9", completed[0].Response.Output[0].ID)
}

func TestResponsesStreamEmitterRejectsToolIdentityChanges(t *testing.T) {
	c, _, info := newResponsesEmitterTestContext(t)
	require.NoError(t, EmitChatChunk(c, info, toolStreamChunk(0, "call_1", "lookup", "")))
	err := EmitChatChunk(c, info, toolStreamChunk(0, "call_2", "lookup", ""))
	require.Error(t, err)
	require.Contains(t, err.Error(), "changed call_id")
}
