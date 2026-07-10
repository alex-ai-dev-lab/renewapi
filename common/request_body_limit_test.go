package common

import (
	"math"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestGetAnonymousRequestBodyLimitBytes(t *testing.T) {
	oldLimit := constant.AnonymousRequestBodyLimitKB
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = oldLimit })

	constant.AnonymousRequestBodyLimitKB = -1
	require.Equal(t, int64(512<<10), GetAnonymousRequestBodyLimitBytes())
	constant.AnonymousRequestBodyLimitKB = 0
	require.Zero(t, GetAnonymousRequestBodyLimitBytes())
	constant.AnonymousRequestBodyLimitKB = 1
	require.Equal(t, int64(1024), GetAnonymousRequestBodyLimitBytes())

	if strconv.IntSize == 64 {
		constant.AnonymousRequestBodyLimitKB = math.MaxInt
		require.Equal(t, int64(math.MaxInt64), GetAnonymousRequestBodyLimitBytes())
	}
}
