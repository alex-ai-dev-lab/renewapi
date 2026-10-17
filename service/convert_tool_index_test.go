package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestClaudeToolNameIndexMatchesHistoricalLookup(t *testing.T) {
	request := dto.ClaudeRequest{Model: "test", Messages: []dto.ClaudeMessage{
		{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: "later", Content: "forward"}, {Type: "tool_result", ToolUseId: "missing", Content: "unknown"}, {Type: "tool_result", ToolUseId: "later", Name: "explicit", Content: "explicit"}, {Type: "tool_result", ToolUseId: "", Content: "empty"}}},
		{Role: "assistant", Content: []dto.ClaudeMediaMessage{{Type: "tool_use", Id: "later", Name: "first", Input: map[string]any{}}, {Type: "tool_use", Id: "later", Name: "duplicate", Input: map[string]any{}}, {Type: "tool_use", Id: "", Name: "empty-id", Input: map[string]any{}}}},
	}}
	result, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	for _, message := range result.Messages {
		if message.Role != "tool" {
			continue
		}
		want := request.SearchToolNameByToolCallId(message.ToolCallId)
		if index == 2 {
			want = "explicit"
		}
		if message.Name == nil || *message.Name != want {
			t.Fatalf("tool %d name=%v want %q", index, message.Name, want)
		}
		index++
	}
	if index != 4 {
		t.Fatalf("tool results=%d", index)
	}
}

func BenchmarkClaudeToolHistory(b *testing.B) {
	for _, turns := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(turns), func(b *testing.B) {
			req := dto.ClaudeRequest{Model: "test"}
			for i := range turns {
				id := fmt.Sprint(i)
				req.Messages = append(req.Messages,
					dto.ClaudeMessage{Role: "assistant", Content: []dto.ClaudeMediaMessage{{Type: "tool_use", Id: id, Name: "tool", Input: map[string]any{"n": i}}}},
					dto.ClaudeMessage{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "tool_result", ToolUseId: id, Content: "ok"}}})
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := ClaudeToOpenAIRequest(req, info); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
