package router

import (
	"embed"
	"net/http"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded frontend assets for the default frontend.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
}

func isWebStaticAssetPath(requestPath string) bool {
	if strings.HasPrefix(requestPath, "/static/") || strings.HasPrefix(requestPath, "/assets/") {
		return true
	}

	// Files copied from a frontend's public directory are served from the root.
	// Limit this exemption to a single root-level file so application routes and
	// nested API-style paths continue to consume the web request budget.
	rootFile := strings.TrimPrefix(requestPath, "/")
	return rootFile != "" && !strings.Contains(rootFile, "/") && path.Ext(rootFile) != ""
}

func webRateLimitMiddleware() gin.HandlerFunc {
	limiter := middleware.GlobalWebRateLimit()
	return func(c *gin.Context) {
		if (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) && isWebStaticAssetPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		limiter(c)
	}
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(webRateLimitMiddleware())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", defaultFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			if target := common.ThemeAwarePath(c.Request.URL.RequestURI()); target != c.Request.URL.RequestURI() {
				c.Redirect(http.StatusFound, target)
				return
			}
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
	})
}
