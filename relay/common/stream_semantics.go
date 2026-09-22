package common

import (
	"strings"

	"github.com/tidwall/gjson"
)

type StreamFrameMeaning struct {
	Valid    bool
	Semantic bool
	Terminal bool
	Failed   bool
}

// ClassifyStreamPayload 识别适配器支持的内容和工具输出，协议前导、心跳和 usage 不算语义输出。
func ClassifyStreamPayload(data string) StreamFrameMeaning {
	if strings.TrimSpace(data) == "[DONE]" {
		return StreamFrameMeaning{Valid: true, Terminal: true}
	}
	if !gjson.Valid(data) {
		return StreamFrameMeaning{}
	}
	r := gjson.Parse(data)
	typ := r.Get("type").String()
	m := StreamFrameMeaning{Valid: true}
	if r.Get("error").Exists() && r.Get("error").Type != gjson.Null ||
		typ == "error" || typ == "response.failed" || typ == "response.error" || r.Get("response.status").String() == "failed" {
		m.Failed, m.Terminal = true, true
		return m
	}
	switch typ {
	case "response.output_text.delta", "response.refusal.delta", "response.reasoning_text.delta",
		"response.reasoning_summary_text.delta", "response.function_call_arguments.delta",
		"response.audio.delta", "response.output_audio.delta", "response.audio_transcript.delta":
		m.Semantic = nonemptyStreamValue(r.Get("delta"))
	case "response.image_generation_call.partial_image":
		m.Semantic = nonemptyStreamValue(r.Get("partial_image_b64"))
	case "response.output_item.added", "response.output_item.done":
		m.Semantic = streamOutputItem(r.Get("item"))
	case "response.completed", "response.incomplete":
		m.Terminal = true
		for _, item := range r.Get("response.output").Array() {
			m.Semantic = m.Semantic || streamOutputItem(item)
		}
	case "content_block_start":
		m.Semantic = streamOutputItem(r.Get("content_block"))
	case "content_block_delta":
		for _, field := range []string{"delta.text", "delta.thinking", "delta.partial_json"} {
			m.Semantic = m.Semantic || nonemptyStreamValue(r.Get(field))
		}
	case "message_stop":
		m.Terminal = true
	case "message_delta":
		m.Terminal = nonemptyStreamValue(r.Get("delta.stop_reason"))
	}
	for _, choice := range r.Get("choices").Array() {
		m.Terminal = m.Terminal || nonemptyStreamValue(choice.Get("finish_reason"))
		m.Semantic = m.Semantic || nonemptyStreamValue(choice.Get("text")) || streamChatMessage(choice.Get("delta")) || streamChatMessage(choice.Get("message"))
	}
	for _, candidate := range r.Get("candidates").Array() {
		m.Terminal = m.Terminal || nonemptyStreamValue(candidate.Get("finishReason"))
		for _, part := range candidate.Get("content.parts").Array() {
			m.Semantic = m.Semantic || nonemptyStreamValue(part.Get("text")) || nonemptyStreamValue(part.Get("functionCall.name")) || nonemptyStreamValue(part.Get("inlineData.data"))
		}
	}
	// Ollama、Dify 和部分文本供应商由现有适配器输出这些字段。
	for _, field := range []string{"answer", "response", "output.text", "result"} {
		if value := r.Get(field); value.Type == gjson.String {
			m.Semantic = m.Semantic || nonemptyStreamValue(value)
		}
	}
	m.Semantic = m.Semantic || streamChatMessage(r.Get("message"))
	m.Terminal = m.Terminal || r.Get("done").Bool() || r.Get("event").String() == "message_end"
	return m
}

func nonemptyStreamValue(v gjson.Result) bool {
	return v.Exists() && v.Type != gjson.Null && v.String() != ""
}

func streamChatMessage(r gjson.Result) bool {
	for _, field := range []string{"content", "reasoning_content", "reasoning", "refusal", "audio.data", "function_call.name", "function_call.arguments"} {
		if value := r.Get(field); value.Type == gjson.String && nonemptyStreamValue(value) {
			return true
		}
	}
	for _, call := range r.Get("tool_calls").Array() {
		if nonemptyStreamValue(call.Get("function.name")) || nonemptyStreamValue(call.Get("function.arguments")) {
			return true
		}
	}
	return false
}

func streamOutputItem(r gjson.Result) bool {
	switch r.Get("type").String() {
	case "function_call", "tool_use":
		return nonemptyStreamValue(r.Get("name"))
	case "compaction", "context_compaction", "compaction_summary":
		return nonemptyStreamValue(r.Get("encrypted_content"))
	case "image_generation_call":
		return nonemptyStreamValue(r.Get("result"))
	case "text", "output_text", "refusal", "thinking":
		return nonemptyStreamValue(r.Get("text")) || nonemptyStreamValue(r.Get("refusal")) || nonemptyStreamValue(r.Get("thinking"))
	}
	for _, content := range r.Get("content").Array() {
		if streamOutputItem(content) {
			return true
		}
	}
	return false
}
