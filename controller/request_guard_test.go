package controller

import (
	"bytes"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/requestguard"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestRequestGuardConfigViewNeverReturnsSecret(t *testing.T) {
	key := requestguard.EndpointSecretOptionKey("primary")
	common.OptionMapRWMutex.Lock()
	previous, existed := common.OptionMap[key]
	common.OptionMap[key] = "super-secret-value"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[key] = previous
		} else {
			delete(common.OptionMap, key)
		}
	})

	view := buildRequestGuardConfigView(operation_setting.RequestGuardSetting{Endpoints: []operation_setting.RequestGuardEndpoint{{ID: "primary"}}})
	encoded, err := common.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "super-secret-value")
	require.Contains(t, string(encoded), `"has_secret":true`)
	require.Contains(t, string(encoded), `"secret_status":"configured"`)
}

func TestRequestGuardDefaultCollectionsEncodeAsArrays(t *testing.T) {
	view := buildRequestGuardConfigView(operation_setting.GetRequestGuardSetting())
	encoded, err := common.Marshal(view)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"groups":[]`)
	require.Contains(t, string(encoded), `"endpoints":[]`)
	require.NotContains(t, string(encoded), `"groups":null`)
	require.NotContains(t, string(encoded), `"endpoints":null`)
}

func TestRequestGuardPreflightCallOrder(t *testing.T) {
	source, err := os.ReadFile("relay.go")
	require.NoError(t, err)
	markers := [][]byte{
		[]byte("relayInfo.SetEstimatePromptTokens(tokens)"),
		[]byte("compat.Hooks().OnRequestPreflight"),
		[]byte("prepareDistributorResponsesRoutePlan"),
		[]byte("helper.ModelPriceHelper"),
		[]byte("service.PreConsumeBilling"),
	}
	previous := -1
	for _, marker := range markers {
		index := bytes.Index(source, marker)
		require.Greater(t, index, previous, string(marker))
		previous = index
	}
}
