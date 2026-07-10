package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachQuotaSaturationUsesAdminInfo(t *testing.T) {
	other := map[string]interface{}{}
	clamp := &common.QuotaClamp{Op: "QuotaRound", Kind: common.QuotaClampOverflow, Original: 1e30, Clamped: common.MaxQuota}
	attachQuotaSaturationToOther(other, clamp)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	audit, ok := adminInfo["quota_saturation"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, common.QuotaClampOverflow, audit["kind"])
	assert.Equal(t, common.MaxQuota, audit["clamped"])
}
