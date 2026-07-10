package types

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestPriceDataRejectsInvalidOtherRatios(t *testing.T) {
	priceData := PriceData{}
	priceData.AddOtherRatio("valid", 2)
	priceData.AddOtherRatio("zero", 0)
	priceData.AddOtherRatio("nan", math.NaN())
	priceData.AddOtherRatio("inf", math.Inf(1))

	assert.Equal(t, map[string]float64{"valid": 2}, priceData.OtherRatios)
	assert.Equal(t, 6.0, priceData.ApplyOtherRatiosToFloat(3))
	assert.True(t, decimal.NewFromInt(6).Equal(priceData.ApplyOtherRatiosToDecimal(decimal.NewFromInt(3))))
}
