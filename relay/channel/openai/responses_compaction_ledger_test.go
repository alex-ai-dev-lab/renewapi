package openai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAggregateCompactionResponsesStreamRawRestoresUnknownItem(t *testing.T) {
	item := `{"type":"compaction_summary","encrypted_content":"opaque","future":{"unknown":true}}`
	stream := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":` + item + `}`,
		`data: {"type":"response.output_item.done","output_index":0,"item":` + item + `}`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response.compaction","output":[],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
		"",
	}, "\n\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(stream))}
	body, usage, relayErr := aggregateCompactionResponsesStreamRaw(resp)
	require.Nil(t, relayErr)
	require.Equal(t, item, gjson.GetBytes(body, "output.0").Raw)
	require.Equal(t, 1, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, 3, usage.TotalTokens)
}

func TestAggregateCompactionResponsesStreamRawRequiresCompleted(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`data: {"type":"response.output_item.done","output_index":0,"item":{"type":"compaction_summary","encrypted_content":"opaque"}}\n\n`))}
	_, _, relayErr := aggregateCompactionResponsesStreamRaw(resp)
	require.NotNil(t, relayErr)
}
