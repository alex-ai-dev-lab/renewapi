package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyModelEndpointDecision_DoesNotOverrideRelayModeOnSafeCorrection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(`{
		"enabled": true,
		"entries": [{
			"match_type": "prefix",
			"pattern": "gpt-5",
			"channel_type": 1,
			"default_endpoint": "openai",
			"supported_endpoints": ["openai"],
			"fallback_endpoint": "openai",
			"auto_correct": true
		}]
	}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(""))
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5.5")

	err := applyModelEndpointDecision(ctx)

	require.Nil(t, err)
	require.Equal(t, "openai", recorder.Header().Get("X-NewAPI-Model-Default-Endpoint"))
	require.Equal(t, "anthropic -> openai", recorder.Header().Get("X-NewAPI-Endpoint-Corrected"))
	_, exists := ctx.Get("relay_mode")
	require.False(t, exists)
}
