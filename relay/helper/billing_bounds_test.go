package helper

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExceedsMaxTokensLimitChecksAllFormats(t *testing.T) {
	valid := uint(maxTokensLimit)
	invalid := uint(math.MaxInt32)
	assert.False(t, exceedsMaxTokensLimit(nil, &valid))
	assert.True(t, exceedsMaxTokensLimit(&valid, &invalid))
}
