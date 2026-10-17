package common

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestUpstreamBodyBoundary(t *testing.T) {
	for _, size := range []int{0, 7, 8, 9, 100000} {
		body, err := ReadUpstreamBody(strings.NewReader(strings.Repeat("x", size)), 8)
		if size > 8 {
			require.ErrorIs(t, err, ErrUpstreamBodyTooLarge)
			require.Nil(t, body)
		} else {
			require.NoError(t, err)
			require.Len(t, body, size)
		}
	}
}
