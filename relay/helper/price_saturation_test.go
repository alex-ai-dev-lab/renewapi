package helper

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestRejectSaturatedPreConsume(t *testing.T) {
	for name, value := range map[string]float64{"nan": math.NaN(), "positive overflow": math.Inf(1), "negative overflow": math.Inf(-1)} {
		t.Run(name, func(t *testing.T) {
			quota, clamp := common.QuotaFromFloatChecked(value)
			info := &relaycommon.RelayInfo{}
			quota, err := rejectSaturatedPreConsume(info, quota, clamp)
			require.Error(t, err)
			require.Zero(t, quota)
			require.NotNil(t, info.QuotaClamp)
		})
	}
}
