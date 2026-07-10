package responsebridge

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service/openaicompat"

	"github.com/gin-gonic/gin"
)

const emitterContextKey = "responses_protocol_bridge_emitter"

type toolState struct {
	Index     int
	ItemID    string
	CallID    string
	Name      string
	Arguments strings.Builder
	Started   bool
}

type ResponsesStreamEmitter struct {
	ResponseID     string
	MessageID      string
	Model          string
	CreatedAt      int64
	Sequence       int
	Started        bool
	MessageStarted bool
	Text           strings.Builder
	Tools          map[int]*toolState
	ToolOrder      []int
	Usage          *dto.Usage
	FinishReason   string
	Completed      bool
}

func getEmitter(c *gin.Context, info *relaycommon.RelayInfo, chunk *dto.ChatCompletionsStreamResponse) *ResponsesStreamEmitter {
	if existing, ok := c.Get(emitterContextKey); ok {
		if emitter, ok := existing.(*ResponsesStreamEmitter); ok {
			return emitter
		}
	}
	id := ""
	model := ""
	created := int64(0)
	if chunk != nil {
		id = chunk.Id
		model = chunk.Model
		created = chunk.Created
	}
	if id == "" {
		id = helper.GetResponseID(c)
	}
	responseID := id
	if !strings.HasPrefix(responseID, "resp_") {
		responseID = "resp_" + strings.TrimPrefix(responseID, "chatcmpl-")
	}
	if model == "" && info != nil {
		model = info.UpstreamModelName
		if model == "" {
			model = info.OriginModelName
		}
	}
	if created == 0 {
		created = common.GetTimestamp()
	}
	emitter := &ResponsesStreamEmitter{
		ResponseID: responseID,
		MessageID:  "msg_" + strings.TrimPrefix(responseID, "resp_"),
		Model:      model,
		CreatedAt:  created,
		Tools:      make(map[int]*toolState),
	}
	c.Set(emitterContextKey, emitter)
	return emitter
}

func EmitChatChunk(c *gin.Context, info *relaycommon.RelayInfo, chunk *dto.ChatCompletionsStreamResponse) error {
	if c == nil || chunk == nil {
		return nil
	}
	emitter := getEmitter(c, info, chunk)
	if emitter.Completed {
		return nil
	}
	if err := emitter.start(c); err != nil {
		return err
	}
	if chunk.Model != "" {
		emitter.Model = chunk.Model
	}
	if chunk.Usage != nil {
		copyUsage := *chunk.Usage
		emitter.Usage = &copyUsage
	}
	for _, choice := range chunk.Choices {
		text := choice.Delta.GetContentString()
		if text != "" {
			if err := emitter.ensureMessage(c); err != nil {
				return err
			}
			emitter.Text.WriteString(text)
			if err := emitter.send(c, "response.output_text.delta", map[string]any{
				"item_id": emitter.MessageID, "output_index": 0, "content_index": 0, "delta": text,
			}); err != nil {
				return err
			}
		}
		for _, tool := range choice.Delta.ToolCalls {
			index := 0
			if tool.Index != nil {
				index = *tool.Index
			}
			state := emitter.Tools[index]
			if state == nil {
				state = &toolState{Index: index}
				emitter.Tools[index] = state
				emitter.ToolOrder = append(emitter.ToolOrder, index)
			}
			if tool.ID != "" {
				state.CallID = tool.ID
			}
			if tool.Function.Name != "" {
				state.Name = tool.Function.Name
			}
			if !state.Started {
				if state.CallID == "" {
					state.CallID = fmt.Sprintf("call_%d", index)
				}
				state.ItemID = "fc_" + state.CallID
				state.Started = true
				if err := emitter.send(c, "response.output_item.added", map[string]any{
					"output_index": emitter.toolOutputIndex(index),
					"item":         map[string]any{"type": "function_call", "id": state.ItemID, "call_id": state.CallID, "name": state.Name, "arguments": "", "status": "in_progress"},
				}); err != nil {
					return err
				}
			}
			if tool.Function.Arguments != "" {
				state.Arguments.WriteString(tool.Function.Arguments)
				if err := emitter.send(c, "response.function_call_arguments.delta", map[string]any{
					"item_id": state.ItemID, "output_index": emitter.toolOutputIndex(index), "delta": tool.Function.Arguments,
				}); err != nil {
					return err
				}
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			emitter.FinishReason = *choice.FinishReason
		}
	}
	return nil
}

func CompleteChatStream(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, finishReason string) error {
	emitter := getEmitter(c, info, nil)
	if emitter.Completed {
		return nil
	}
	if usage != nil {
		copyUsage := *usage
		emitter.Usage = &copyUsage
	}
	if finishReason != "" {
		emitter.FinishReason = finishReason
	}
	if err := emitter.start(c); err != nil {
		return err
	}
	if emitter.MessageStarted {
		text := emitter.Text.String()
		if err := emitter.send(c, "response.output_text.done", map[string]any{"item_id": emitter.MessageID, "output_index": 0, "content_index": 0, "text": text}); err != nil {
			return err
		}
		if err := emitter.send(c, "response.content_part.done", map[string]any{"item_id": emitter.MessageID, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}); err != nil {
			return err
		}
		if err := emitter.send(c, "response.output_item.done", map[string]any{"output_index": 0, "item": emitter.messageItem("completed")}); err != nil {
			return err
		}
	}
	for _, index := range emitter.ToolOrder {
		state := emitter.Tools[index]
		if state == nil || !state.Started {
			continue
		}
		arguments := state.Arguments.String()
		if err := emitter.send(c, "response.function_call_arguments.done", map[string]any{"item_id": state.ItemID, "output_index": emitter.toolOutputIndex(index), "arguments": arguments}); err != nil {
			return err
		}
		if err := emitter.send(c, "response.output_item.done", map[string]any{"output_index": emitter.toolOutputIndex(index), "item": emitter.toolItem(state, "completed")}); err != nil {
			return err
		}
	}
	response, err := emitter.responseObject()
	if err != nil {
		return err
	}
	eventType := "response.completed"
	status := "completed"
	if emitter.FinishReason == constant.FinishReasonLength {
		eventType = "response.incomplete"
		status = "incomplete"
	} else if emitter.FinishReason == constant.FinishReasonContentFilter {
		eventType = "response.failed"
		status = "failed"
	}
	statusRaw, _ := common.Marshal(status)
	response.Status = statusRaw
	if status == "incomplete" {
		response.IncompleteDetails = &dto.IncompleteDetails{Reasoning: emitter.FinishReason}
	}
	if err := emitter.send(c, eventType, map[string]any{"response": response}); err != nil {
		return err
	}
	emitter.Completed = true
	return nil
}

func EmitChatResponseAsStream(c *gin.Context, info *relaycommon.RelayInfo, response *dto.OpenAITextResponse) error {
	if response == nil {
		return fmt.Errorf("chat response is nil")
	}
	created := int64(0)
	switch value := response.Created.(type) {
	case int64:
		created = value
	case int:
		created = int64(value)
	case float64:
		created = int64(value)
	}
	chunk := &dto.ChatCompletionsStreamResponse{Id: response.Id, Model: response.Model, Created: created, Usage: &response.Usage}
	for _, choice := range response.Choices {
		streamChoice := dto.ChatCompletionsStreamResponseChoice{Index: choice.Index, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}}
		streamChoice.Delta.SetContentString(choice.Message.StringContent())
		for _, tool := range choice.Message.ParseToolCalls() {
			index := len(streamChoice.Delta.ToolCalls)
			streamChoice.Delta.ToolCalls = append(streamChoice.Delta.ToolCalls, dto.ToolCallResponse{Index: &index, ID: tool.ID, Type: tool.Type, Function: dto.FunctionResponse{Name: tool.Function.Name, Arguments: tool.Function.Arguments}})
		}
		finish := choice.FinishReason
		streamChoice.FinishReason = &finish
		chunk.Choices = append(chunk.Choices, streamChoice)
	}
	if err := EmitChatChunk(c, info, chunk); err != nil {
		return err
	}
	finishReason := ""
	if len(response.Choices) > 0 {
		finishReason = response.Choices[0].FinishReason
	}
	return CompleteChatStream(c, info, &response.Usage, finishReason)
}

func (e *ResponsesStreamEmitter) start(c *gin.Context) error {
	if e.Started {
		return nil
	}
	helper.SetEventStreamHeaders(c)
	response, err := e.responseObject()
	if err != nil {
		return err
	}
	status, _ := common.Marshal("in_progress")
	response.Status = status
	if err := e.send(c, "response.created", map[string]any{"response": response}); err != nil {
		return err
	}
	if err := e.send(c, "response.in_progress", map[string]any{"response": response}); err != nil {
		return err
	}
	e.Started = true
	return nil
}

func (e *ResponsesStreamEmitter) ensureMessage(c *gin.Context) error {
	if e.MessageStarted {
		return nil
	}
	e.MessageStarted = true
	if err := e.send(c, "response.output_item.added", map[string]any{"output_index": 0, "item": e.messageItem("in_progress")}); err != nil {
		return err
	}
	return e.send(c, "response.content_part.added", map[string]any{"item_id": e.MessageID, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
}

func (e *ResponsesStreamEmitter) messageItem(status string) map[string]any {
	return map[string]any{"type": "message", "id": e.MessageID, "status": status, "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": e.Text.String(), "annotations": []any{}}}}
}

func (e *ResponsesStreamEmitter) toolItem(state *toolState, status string) map[string]any {
	return map[string]any{"type": "function_call", "id": state.ItemID, "status": status, "call_id": state.CallID, "name": state.Name, "arguments": state.Arguments.String()}
}

func (e *ResponsesStreamEmitter) toolOutputIndex(toolIndex int) int {
	base := 0
	if e.MessageStarted {
		base = 1
	}
	for order, index := range e.ToolOrder {
		if index == toolIndex {
			return base + order
		}
	}
	return base + toolIndex
}

func (e *ResponsesStreamEmitter) responseObject() (*dto.OpenAIResponsesResponse, error) {
	message := dto.Message{Role: "assistant"}
	message.SetStringContent(e.Text.String())
	toolCalls := make([]dto.ToolCallResponse, 0, len(e.ToolOrder))
	for _, index := range e.ToolOrder {
		state := e.Tools[index]
		if state == nil {
			continue
		}
		toolCalls = append(toolCalls, dto.ToolCallResponse{ID: state.CallID, Type: "function", Function: dto.FunctionResponse{Name: state.Name, Arguments: state.Arguments.String()}})
	}
	if len(toolCalls) > 0 {
		message.SetToolCalls(toolCalls)
	}
	finishReason := e.FinishReason
	if finishReason == "" {
		finishReason = constant.FinishReasonStop
	}
	chat := &dto.OpenAITextResponse{Id: e.ResponseID, Object: "chat.completion", Created: e.CreatedAt, Model: e.Model, Choices: []dto.OpenAITextResponseChoice{{Index: 0, Message: message, FinishReason: finishReason}}}
	if e.Usage != nil {
		chat.Usage = *e.Usage
	}
	return openaicompat.ChatResponseToResponses(chat)
}

func (e *ResponsesStreamEmitter) send(c *gin.Context, eventType string, fields map[string]any) error {
	payload := make(map[string]any, len(fields)+2)
	for key, value := range fields {
		payload[key] = value
	}
	payload["type"] = eventType
	payload["sequence_number"] = e.Sequence
	e.Sequence++
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventType}, string(data))
	return nil
}
