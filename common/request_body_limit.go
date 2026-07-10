package common

import (
	"math"

	"github.com/QuantumNous/new-api/constant"
)

const defaultAnonymousRequestBodyLimitKB = 512

func GetAnonymousRequestBodyLimitBytes() int64 {
	limitKB := constant.AnonymousRequestBodyLimitKB
	if limitKB < 0 {
		limitKB = defaultAnonymousRequestBodyLimitKB
	}
	if int64(limitKB) > math.MaxInt64>>10 {
		return math.MaxInt64
	}
	return int64(limitKB) << 10
}
