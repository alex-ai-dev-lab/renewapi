package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPanicRecoveryKeepsInternalCauseOutOfResponse(t *testing.T) {
	for name, recovery := range map[string]gin.HandlerFunc{
		"relay":  RelayPanicRecover(),
		"global": gin.CustomRecovery(func(c *gin.Context, _ any) { WritePanicResponse(c) }),
	} {
		t.Run(name, func(t *testing.T) {
			router := gin.New()
			router.Use(recovery)
			router.GET("/panic", func(c *gin.Context) { panic("private upstream failure details") })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
			require.Equal(t, http.StatusInternalServerError, recorder.Code)
			require.Equal(t, "new_api_panic", gjson.Get(recorder.Body.String(), "error.type").String())
			require.NotContains(t, recorder.Body.String(), "private upstream")
			require.NotEmpty(t, gjson.Get(recorder.Body.String(), "error.message").String())
		})
	}
}

func TestPanicRecoveryPreservesCommittedStream(t *testing.T) {
	for name, recovery := range map[string]gin.HandlerFunc{
		"relay":  RelayPanicRecover(),
		"global": gin.CustomRecovery(func(c *gin.Context, _ any) { WritePanicResponse(c) }),
	} {
		t.Run(name, func(t *testing.T) {
			const frame = "data: {\"delta\":\"hello\"}\n\n"
			router := gin.New()
			router.Use(recovery)
			router.GET("/stream", func(c *gin.Context) {
				c.Header("Content-Type", "text/event-stream")
				_, _ = c.Writer.WriteString(frame)
				c.Writer.Flush()
				panic("private stream failure")
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, frame, recorder.Body.String())
			require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
		})
	}
}
