package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCopyVideoResponseHeadersOverridesPublicCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	copyVideoResponseHeaders(ctx, http.Header{"Cache-Control": {"public, max-age=86400"}, "Content-Type": {"video/mp4"}})
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", recorder.Header().Get("Pragma"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
}

func TestWriteVideoDataURLPrivateAndBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_PROXY_MAX_BYTES", "4")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	require.NoError(t, writeVideoDataURL(ctx, "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("1234"))))
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "1234", recorder.Body.String())

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	err := writeVideoDataURL(ctx, "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("12345")))
	require.ErrorContains(t, err, "exceeds")
	require.Empty(t, recorder.Body.String())
}
