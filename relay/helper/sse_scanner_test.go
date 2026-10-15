package helper

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestSSEScannerBoundsWholeMultilineFrame(t *testing.T) {
	previous := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() { constant.StreamScannerMaxBufferMB = previous })
	body := "data: {\n" + strings.Repeat("data: "+strings.Repeat("x", 16*1024)+"\n", 65)
	scanner := NewSSEScanner(strings.NewReader(body))
	require.False(t, scanner.Scan())
	require.ErrorContains(t, scanner.Err(), "SSE frame exceeds maximum buffer size")
}
