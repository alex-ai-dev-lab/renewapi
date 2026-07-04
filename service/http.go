package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

var allowedUpstreamResponseHeaders = map[string]struct{}{
	"Cache-Control":       {},
	"Content-Disposition": {},
	"Content-Encoding":    {},
	"Content-Language":    {},
	"Content-Range":       {},
	"Content-Type":        {},
	"ETag":                {},
	"Expires":             {},
	"Last-Modified":       {},
	"Retry-After":         {},
}

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. Sensitive upstream headers such as
// Set-Cookie, WWW-Authenticate and CORS policy are intentionally not forwarded.
// When the upstream header is X-Oneapi-Request-Id, the value is captured into the
// Gin context for later logging.
func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	_, ok := allowedUpstreamResponseHeaders[http.CanonicalHeaderKey(k)]
	return ok
}

func CopyAllowedUpstreamHeaders(c *gin.Context, src http.Header) {
	if c == nil || c.Writer == nil {
		return
	}
	for k, values := range src {
		if !ShouldCopyUpstreamHeader(c, k, values) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(k, value)
		}
	}
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		CopyAllowedUpstreamHeaders(c, src.Header)
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
