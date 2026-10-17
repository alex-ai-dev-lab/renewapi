package service

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func assertReasoningSanitizerEquivalent(t testing.TB, raw []byte) {
	t.Helper()
	got, count, changed := sanitizeResponsesReasoningContentJSON(raw)
	want, wantCount, wantChanged := sanitizeResponsesReasoningContentJSONDecoded(raw)
	if count != wantCount || changed != wantChanged {
		t.Fatalf("flags for %q: %d/%t vs %d/%t", raw, count, changed, wantCount, wantChanged)
	}
	if !changed {
		if !bytes.Equal(got, raw) {
			t.Fatalf("unchanged input rewritten: %q", raw)
		}
		return
	}
	var a, b any
	if err := common.Unmarshal(got, &a); err != nil {
		t.Fatalf("invalid result %q for %q: %v", got, raw, err)
	}
	if err := common.Unmarshal(want, &b); err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("result mismatch for %q: %q vs %q (%v)", raw, got, want, err)
	}
}

func TestReasoningSanitizerCompatibility(t *testing.T) {
	for _, raw := range []string{
		`{"type":"reasoning","content":[1]}`,
		`{"content":[1],"type":"reasoning"}`,
		`{"content":[1]}`,
		`{"type":"reasoning", "a":{"content":[2]}, "content":[1], "b":{"content":[3]}}`,
		`{"type":"reasoning","content":[1],"type":"text"}`,
		`{"type":"text","type":"reasoning","content":[1],"content":[]}`,
		`{"ty\u0070e":"rea\u0073oning","cont\u0065nt":[1],"x":2}`,
		`{"type":"reasoning","content":[{"type":"reasoning","content":[1]}]}`,
		`{"type":"reasoning","content":[],"nested":[{"content":[1]}],"x":{"content":[1]}}`,
		` { "type" : "reasoning", "content" : [ null ] , "n":9007199254740993 } `,
		`[{"type":"reasoning","content":[1]},{"type":"reasoning","content":[2]}]`,
		`{"type":"reasoning","content":[1e400]}`,
		`{"type":"reasoning","content":[1],"n":1e400}`,
		`{"type":"reasoning","content":[1],"text":"` + string([]byte{0xff}) + `"}`,
		`{"type":"reasoning","content":[1],"text":"\ud800"}`,
		`{"type":"reasoning","content":[1],}`,
		`null`, `"reasoning"`, `[]`, ``,
	} {
		t.Run(raw, func(t *testing.T) { assertReasoningSanitizerEquivalent(t, []byte(raw)) })
	}
	raw := []byte(`{"type":"reasoning","content":[1],"n":9007199254740993}`)
	got, _, _ := sanitizeResponsesReasoningContentJSON(raw)
	if !bytes.Contains(got, []byte("9007199254740993")) {
		t.Fatal("untouched number changed precision")
	}
}

func FuzzReasoningSanitizerEquivalent(f *testing.F) {
	for _, seed := range []string{`{"type":"reasoning","content":[1]}`, `[{"type":"reasoning","a":{"content":[2]},"content":[1]}]`, `{"type":"reasoning","content":[1],"n":1e400}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) { assertReasoningSanitizerEquivalent(t, raw) })
}

func BenchmarkReasoningSanitizer(b *testing.B) {
	for _, change := range []bool{false, true} {
		name := "unchanged"
		prefix := `[{"type":"message","content":"`
		if change {
			name = "remove"
			prefix = `[{"type":"reasoning","content":[1]},{"type":"message","content":"`
		}
		raw := []byte(prefix + strings.Repeat("a", 1<<20) + `"}]`)
		for _, old := range []bool{true, false} {
			method := "optimized"
			if old {
				method = "decoded"
			}
			b.Run(name+"/"+method, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if old {
						sanitizeResponsesReasoningContentJSONDecoded(raw)
					} else {
						sanitizeResponsesReasoningContentJSON(raw)
					}
				}
			})
		}
	}
}
