package middleware

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccessLoggerDoesNotRecordQueryCredentials(t *testing.T) {
	var output bytes.Buffer
	old := gin.DefaultWriter
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = old })
	r := gin.New()
	SetUpLogger(r)
	r.GET("/v1beta/models", func(c *gin.Context) {
		require.Equal(t, "FAKE_SECRET", c.Query("key"))
		c.Status(401)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/v1beta/models?key=FAKE_SECRET&x=1", nil))
	require.Contains(t, output.String(), "/v1beta/models")
	require.NotContains(t, output.String(), "FAKE_SECRET")
	require.NotContains(t, output.String(), "?key")
}
