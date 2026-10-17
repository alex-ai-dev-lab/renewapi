package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TestTokenConcurrencyMiddlewareReleasesAfterTerminalPaths(t *testing.T) {
	previous := common.RedisEnabled
	common.RedisEnabled = false
	service.ResetTokenLimitForTest()
	t.Cleanup(func() {
		common.RedisEnabled = previous
		service.ResetTokenLimitForTest()
	})
	router := gin.New()
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) { c.AbortWithStatus(http.StatusInternalServerError) }))
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyTokenId, 43)
		common.SetContextKey(c, constant.ContextKeyTokenConcurrencyLimit, 1)
	})
	router.Use(TokenRequestLimit())
	router.GET("/:mode", func(c *gin.Context) {
		switch c.Param("mode") {
		case "panic":
			panic("test handler panic")
		case "cancel":
			if c.Request.Context().Err() == nil {
				t.Error("middleware lost the request cancellation")
			}
			c.Status(499)
		default:
			c.Status(http.StatusNoContent)
		}
	})
	for _, path := range []struct {
		mode   string
		status int
	}{{"success", http.StatusNoContent}, {"panic", http.StatusInternalServerError}, {"cancel", 499}} {
		t.Run(path.mode, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+path.mode, nil)
			if path.mode == "cancel" {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != path.status {
				t.Fatalf("handler returned %d, want %d", response.Code, path.status)
			}
			next := httptest.NewRecorder()
			router.ServeHTTP(next, httptest.NewRequest(http.MethodGet, "/success", nil))
			if next.Code != http.StatusNoContent {
				t.Fatalf("%s leaked the slot: next request returned %d", path.mode, next.Code)
			}
		})
	}
}
