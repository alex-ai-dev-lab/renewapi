package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyResponsesActionableOutput(t *testing.T) {
	for _, payload := range []string{
		`{"type":"response.output_item.added","item":{"type":"computer_call","action":{"type":"screenshot"}}}`,
		`{"type":"response.output_item.done","item":{"type":"local_shell_call","action":{"type":"exec","command":["pwd"]}}}`,
		`{"type":"response.output_item.added","item":{"type":"shell_call","action":{"commands":["pwd"]}}}`,
		`{"type":"response.completed","response":{"output":[{"type":"apply_patch_call","operation":{"type":"create_file","path":"a.txt","diff":"+hello"}}]}}`,
		`{"type":"response.output_text.done","text":"hello"}`,
		`{"type":"response.refusal.done","refusal":"Cannot help."}`,
		`{"type":"response.function_call_arguments.done","arguments":"{}"}`,
		`{"type":"response.custom_tool_call_input.done","input":"hello"}`,
	} {
		t.Run(payload, func(t *testing.T) {
			meaning := ClassifyStreamPayload(payload)
			require.True(t, meaning.Valid)
			require.True(t, meaning.Semantic)
			require.False(t, meaning.Failed)
		})
	}
}

func TestClassifyResponsesPreludeRemainsNonSemantic(t *testing.T) {
	for _, payload := range []string{
		`{"type":"response.created","response":{"status":"in_progress"}}`,
		`{"type":"response.output_item.added","item":{"type":"message","role":"assistant","content":[]}}`,
		`{"type":"response.output_item.added","item":{"type":"computer_call"}}`,
		`{"type":"response.output_item.added","item":{"type":"shell_call","action":{"commands":[]}}}`,
		`{"type":"response.web_search_call.in_progress"}`,
		`{"type":"response.completed","response":{"output":[],"usage":{"output_tokens":12}}}`,
		`{"type":"response.custom_tool_call_input.done","input":""}`,
	} {
		require.False(t, ClassifyStreamPayload(payload).Semantic, payload)
	}
	failed := ClassifyStreamPayload(`{"type":"response.failed","response":{"output":[{"type":"function_call","name":"lookup"}]}}`)
	require.True(t, failed.Failed)
	require.False(t, failed.Semantic)
}
