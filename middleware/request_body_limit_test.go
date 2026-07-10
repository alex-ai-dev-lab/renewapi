package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func runAnonymousBodyLimitRequest(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/", AnonymousRequestBodyLimit(), func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.Data(http.StatusOK, "application/octet-stream", body)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestAnonymousRequestBodyLimitPreservesBodyAtLimit(t *testing.T) {
	oldLimit := constant.AnonymousRequestBodyLimitKB
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = oldLimit })
	constant.AnonymousRequestBodyLimitKB = 1

	body := strings.Repeat("x", 1024)
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	response := runAnonymousBodyLimitRequest(t, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, body, response.Body.String())
}

func TestAnonymousRequestBodyLimitRejectsKnownAndChunkedOversizeBodies(t *testing.T) {
	oldLimit := constant.AnonymousRequestBodyLimitKB
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = oldLimit })
	constant.AnonymousRequestBodyLimitKB = 1
	body := strings.Repeat("x", 1025)

	knownLength := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	response := runAnonymousBodyLimitRequest(t, knownLength)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)

	chunked := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	chunked.ContentLength = -1
	response = runAnonymousBodyLimitRequest(t, chunked)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}

func TestAnonymousRequestBodyLimitCanBeDisabledAndHandlesReadErrors(t *testing.T) {
	oldLimit := constant.AnonymousRequestBodyLimitKB
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = oldLimit })

	constant.AnonymousRequestBodyLimitKB = 0
	body := strings.Repeat("x", 2048)
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	response := runAnonymousBodyLimitRequest(t, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, body, response.Body.String())

	constant.AnonymousRequestBodyLimitKB = 1
	request = httptest.NewRequest(http.MethodPost, "/", nil)
	request.Body = failingReadCloser{}
	request.ContentLength = -1
	response = runAnonymousBodyLimitRequest(t, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
}
