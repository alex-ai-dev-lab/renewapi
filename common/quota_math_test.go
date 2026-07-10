package common

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

const overflowingQuotaProduct = 2000 * 1.8446744073686647e19

func TestQuotaConversionsSaturate(t *testing.T) {
	assert.Equal(t, 42, QuotaFromFloat(42.9))
	assert.Equal(t, -42, QuotaFromFloat(-42.9))
	assert.Equal(t, 43, QuotaRound(42.5))
	assert.Equal(t, -43, QuotaRound(-42.5))
	assert.Equal(t, MaxQuota, QuotaFromFloat(overflowingQuotaProduct))
	assert.Equal(t, MinQuota, QuotaFromFloat(-overflowingQuotaProduct))
	assert.Equal(t, MaxQuota, QuotaFromFloat(math.Inf(1)))
	assert.Equal(t, MinQuota, QuotaFromFloat(math.Inf(-1)))
	assert.Equal(t, 0, QuotaFromFloat(math.NaN()))
	assert.Equal(t, 43, QuotaFromDecimal(decimal.NewFromFloat(42.5)))
}

func TestQuotaClampDescribesSaturation(t *testing.T) {
	quota, clamp := QuotaRoundChecked(overflowingQuotaProduct)
	assert.Equal(t, MaxQuota, quota)
	if assert.NotNil(t, clamp) {
		assert.Equal(t, QuotaClampOverflow, clamp.Kind)
		assert.Equal(t, MaxQuota, clamp.Clamped)
	}
	quota, clamp = QuotaFromFloatChecked(12.3)
	assert.Equal(t, 12, quota)
	assert.Nil(t, clamp)
}
