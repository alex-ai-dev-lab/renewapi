package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPanicRecoveryHandlerRedactsPanicAndReturnsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId())
	engine.Use(gin.CustomRecoveryWithWriter(nil, PanicRecoveryHandler))
	engine.GET("/panic", func(*gin.Context) {
		panic("sensitive panic detail")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	requestID := recorder.Header().Get(common.RequestIdKey)
	require.NotEmpty(t, requestID)
	require.Contains(t, recorder.Body.String(), "Internal server error")
	require.Contains(t, recorder.Body.String(), requestID)
	require.NotContains(t, recorder.Body.String(), "sensitive panic detail")
}

func TestRelayPanicRecoverUsesSameRedactedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId())
	engine.Use(RelayPanicRecover())
	engine.GET("/panic", func(*gin.Context) {
		panic("relay secret")
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.False(t, strings.Contains(recorder.Body.String(), "relay secret"))
	require.Contains(t, recorder.Body.String(), `"type":"new_api_panic"`)
}
