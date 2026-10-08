package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

func BenchmarkClaudeResponsesConversion(b *testing.B) {
	request := &dto.ClaudeRequest{Model: "gpt-test", Messages: []dto.ClaudeMessage{{Role: "user", Content: strings.Repeat("A synthetic request with stable tool identifiers. ", 180)}}}
	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := openaicompat.ClaudeRequestToResponsesRequest(request); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("via_chat", func(b *testing.B) {
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			chat, err := ClaudeToOpenAIRequest(*request, info)
			if err != nil {
				b.Fatal(err)
			}
			if _, err = openaicompat.ChatCompletionsRequestToResponsesRequest(chat); err != nil {
				b.Fatal(err)
			}
		}
	})
}
