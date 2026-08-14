package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validRequestGuardSettingForTest() RequestGuardSetting {
	return RequestGuardSetting{
		Enabled: true, Mode: RequestGuardModeEnforce, FailurePolicy: RequestGuardFailureClosed,
		InputMode: RequestGuardInputFullClientControlled, MaxInputRunes: 16000,
		EvaluationTimeoutMs: 2500,
		Scope:               RequestGuardScope{AllGroups: true, Models: []string{"*"}, Protocols: []string{"openai_chat"}},
		Bulkhead:            RequestGuardBulkhead{MaxConcurrent: 64, MaxPerEndpoint: 16},
		Observe:             RequestGuardObserve{WorkerCount: 4, QueueCapacity: 4096},
		Endpoints: []RequestGuardEndpoint{{
			ID: "primary", Enabled: true, Priority: 100, BaseURL: "https://guard.example/v1",
			Model: "guard", Codec: RequestGuardCodecJSONPolicy, TimeoutMs: 1500,
			InputLimitRunes: 16000, ProxyPolicy: RequestGuardProxyDisabled,
		}},
	}
}

func TestRequestGuardDefaultIsFailClosedAndOff(t *testing.T) {
	setting := GetRequestGuardSetting()
	require.False(t, setting.Enabled)
	require.Equal(t, RequestGuardModeOff, setting.Mode)
	require.Equal(t, RequestGuardFailureClosed, setting.FailurePolicy)
}

func TestValidateRequestGuardSetting(t *testing.T) {
	setting := validRequestGuardSettingForTest()
	require.NoError(t, ValidateRequestGuardSetting(setting))

	duplicate := setting
	duplicate.Endpoints = append(duplicate.Endpoints, duplicate.Endpoints[0])
	require.ErrorContains(t, ValidateRequestGuardSetting(duplicate), "duplicate endpoint id")

	noEndpoint := setting
	noEndpoint.Endpoints = nil
	require.ErrorContains(t, ValidateRequestGuardSetting(noEndpoint), "at least one enabled endpoint")

	privateCredentials := setting
	privateCredentials.Endpoints = append([]RequestGuardEndpoint(nil), setting.Endpoints...)
	privateCredentials.Endpoints[0].BaseURL = "https://user:pass@guard.example/v1"
	require.ErrorContains(t, ValidateRequestGuardSetting(privateCredentials), "must not contain credentials")

	badProxy := setting
	badProxy.Endpoints = append([]RequestGuardEndpoint(nil), setting.Endpoints...)
	badProxy.Endpoints[0].ProxyPolicy = RequestGuardProxyExplicit
	badProxy.Endpoints[0].ProxyURL = ""
	require.ErrorContains(t, ValidateRequestGuardSetting(badProxy), "proxy_url is required")

	credentialedProxy := setting
	credentialedProxy.Endpoints = append([]RequestGuardEndpoint(nil), setting.Endpoints...)
	credentialedProxy.Endpoints[0].ProxyPolicy = RequestGuardProxyExplicit
	credentialedProxy.Endpoints[0].ProxyURL = "http://user:password@proxy.example:8080"
	require.ErrorContains(t, ValidateRequestGuardSetting(credentialedProxy), "proxy_url must not contain credentials")
}
