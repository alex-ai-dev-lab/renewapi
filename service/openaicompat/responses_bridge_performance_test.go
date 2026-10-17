package openaicompat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestResponsesBridgeUnchangedInputOwnsBytesAndPreservesNumbers(t *testing.T) {
	raw := common.RawMessage(` [ { "role":"user", "content":"hello", "n":9007199254740993 } ] `)
	req := &dto.OpenAIResponsesRequest{Model: "test", Input: raw}
	prepared, _, err := PrepareResponsesRequestForTextBridge(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prepared.Input, raw) {
		t.Fatalf("unchanged conversation rewritten: %s", prepared.Input)
	}
	prepared.Input[0] = '\n'
	if raw[0] != ' ' {
		t.Fatal("bridge input aliases original request")
	}
	stringRaw := common.RawMessage(` "hello\nworld" `)
	prepared, _, err = PrepareResponsesRequestForTextBridge(&dto.OpenAIResponsesRequest{Input: stringRaw})
	if err != nil || !bytes.Equal(prepared.Input, stringRaw) {
		t.Fatalf("unchanged string rewritten: %+v %v", prepared, err)
	}
	invalidUTF8 := common.RawMessage(`[{"role":"user","content":"` + string([]byte{0xff}) + `"}]`)
	prepared, _, err = PrepareResponsesRequestForTextBridge(&dto.OpenAIResponsesRequest{Input: invalidUTF8})
	if err != nil || bytes.Contains(prepared.Input, []byte{0xff}) {
		t.Fatal("invalid UTF8 bypassed decoder normalization")
	}
}

func TestResponsesBridgeFastPathDoesNotSkipValidationOrNamespace(t *testing.T) {
	for _, input := range []string{`[{"role":"user"},]`, `[{"role":"user"}][]`, `true`, `{}`, `[{"type":"additional_tools","tools":null}]`, `[{"type":"additional_tools","tools":"bad"}]`} {
		if _, _, err := PrepareResponsesRequestForTextBridge(&dto.OpenAIResponsesRequest{Input: common.RawMessage(input)}); err == nil {
			t.Fatalf("invalid input accepted: %s", input)
		}
	}
	input := common.RawMessage(`[{"ty\u0070e":"additional_tools","tools":[{"type":"function","name":"tool"}]}]`)
	prepared, _, err := PrepareResponsesRequestForTextBridge(&dto.OpenAIResponsesRequest{Input: input})
	if err != nil || string(prepared.Input) != "[]" || !bytes.Contains(prepared.Tools, []byte("tool")) {
		t.Fatalf("escaped additional tools not lifted: %+v %v", prepared, err)
	}
	tools := common.RawMessage(`[{"type":"namespace","name":"team","tools":[{"type":"function","name":"send"}]}]`)
	input = common.RawMessage(`[{"type":"function_call","namespace":"team","name":"send","arguments":"{}"}]`)
	prepared, mapping, err := PrepareResponsesRequestForTextBridge(&dto.OpenAIResponsesRequest{Input: input, Tools: tools})
	if err != nil || mapping.NamespaceTools["team__send"].Name != "send" || !bytes.Contains(prepared.Input, []byte("team__send")) {
		t.Fatalf("namespace not rewritten: %+v %v", prepared, err)
	}
}

func BenchmarkResponsesBridgeUnchanged(b *testing.B) {
	req := &dto.OpenAIResponsesRequest{Model: "test", Input: common.RawMessage(`[{"role":"user","content":"` + strings.Repeat("a", 1<<20) + `"}]`)}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := PrepareResponsesRequestForTextBridge(req); err != nil {
			b.Fatal(err)
		}
	}
}
