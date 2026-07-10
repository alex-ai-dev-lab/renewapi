package middleware

import (
	"bytes"
	"io"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func AnonymousRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		maxBytes := common.GetAnonymousRequestBodyLimitBytes()
		if maxBytes <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			_ = c.Request.Body.Close()
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}

		originalBody := c.Request.Body
		limitedBody, err := readAnonymousRequestBody(originalBody, maxBytes)
		_ = originalBody.Close()
		if err != nil {
			if common.IsRequestBodyTooLargeError(err) {
				c.AbortWithStatus(http.StatusRequestEntityTooLarge)
				return
			}
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(limitedBody))
		c.Request.ContentLength = int64(len(limitedBody))
		c.Next()
	}
}

func readAnonymousRequestBody(body io.Reader, maxBytes int64) ([]byte, error) {
	readLimit := maxBytes + 1
	if maxBytes == math.MaxInt64 {
		readLimit = math.MaxInt64
	}
	data, err := io.ReadAll(io.LimitReader(body, readLimit))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, common.ErrRequestBodyTooLarge
	}
	return data, nil
}
