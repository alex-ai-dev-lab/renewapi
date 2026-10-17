package common_test

import (
	"bytes"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"testing"
)

func TestDeepCopyRawMessageOwnership(t *testing.T) {
	type record struct {
		Raw    common.RawMessage
		Nested []common.RawMessage
		Values map[string]common.RawMessage
		Zero   *int
		False  *bool
		Ptr    *common.RawMessage
		NilPtr *common.RawMessage
	}
	zero, no := 0, false
	src := record{Raw: common.RawMessage(`{"a":1}`), Nested: []common.RawMessage{nil, {}, common.RawMessage(`[]`)}, Values: map[string]common.RawMessage{"v": common.RawMessage(`true`)}, Zero: &zero, False: &no}
	pointed := common.RawMessage(`{"pointer":0}`)
	src.Ptr = &pointed
	dst, err := common.DeepCopy(&src)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Zero == nil || *dst.Zero != 0 || dst.Zero == src.Zero || dst.False == nil || *dst.False || dst.False == src.False {
		t.Fatal("explicit zeros lost or aliased")
	}
	if dst.Nested[0] != nil || dst.Nested[1] == nil {
		t.Fatal("nil/empty raw values changed")
	}
	if dst.NilPtr != nil || dst.Ptr == nil || dst.Ptr == src.Ptr {
		t.Fatal("RawMessage pointer ownership changed")
	}
	(*dst.Ptr)[2] = 'x'
	if string(*src.Ptr) != `{"pointer":0}` {
		t.Fatal("RawMessage pointer aliases source bytes")
	}
	dst.Raw[2] = 'b'
	dst.Nested[2][0] = '{'
	dst.Values["v"][0] = 'f'
	if string(src.Raw) != `{"a":1}` || string(src.Nested[2]) != `[]` || string(src.Values["v"]) != `true` {
		t.Fatal("copy aliases source JSON")
	}
	src.Raw[0] = '['
	if dst.Raw[0] != '{' {
		t.Fatal("source aliases copy JSON")
	}
	if _, err := common.DeepCopy[record](nil); err == nil {
		t.Fatal("nil source accepted")
	}
}

func BenchmarkDeepCopyRawMessage(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20} {
		name := "64KiB"
		if size > 64<<10 {
			name = "1MiB"
		}
		b.Run(name, func(b *testing.B) {
			src := dto.OpenAIResponsesRequest{Model: "test", Input: common.RawMessage(append(append([]byte(`[{"role":"user","content":"`), bytes.Repeat([]byte("a"), size)...), []byte(`"}]`)...))}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for b.Loop() {
				if _, err := common.DeepCopy(&src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
