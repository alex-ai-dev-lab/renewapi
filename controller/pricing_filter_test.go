package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingByUsableGroupsStaysAuthoritative(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "all-model", EnableGroup: []string{"all"}},
		{ModelName: "default-model", EnableGroup: []string{"default"}},
		{ModelName: "vip-model", EnableGroup: []string{"vip"}},
	}

	filtered := filterPricingByUsableGroups(pricing, map[string]string{"default": "Default"})
	require.Len(t, filtered, 2)
	require.Equal(t, "all-model", filtered[0].ModelName)
	require.Equal(t, "default-model", filtered[1].ModelName)

	// A model is only kept when it is universally enabled or one of its groups
	// is in the authoritative usable-group set.
	require.Empty(t, filterPricingByUsableGroups(pricing, map[string]string{}))
	require.Empty(t, filterPricingByUsableGroups(nil, map[string]string{"default": "Default"}))
}
