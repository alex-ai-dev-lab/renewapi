package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{
		"Accept",
		"Authorization",
		"Content-Type",
		"New-Api-User",
		"Origin",
		"User-Agent",
		"X-Requested-With",
		"X-Request-Id",
	}
	config.ExposeHeaders = []string{"X-New-Api-Version", "X-Request-Id"}
	config.AllowOriginFunc = func(origin string) bool {
		return isTrustedCORSOrigin(origin)
	}
	return cors.New(config)
}

func isTrustedCORSOrigin(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		// Non-browser and same-origin requests legitimately omit Origin; CORS
		// should not reject those paths before normal authentication runs.
		return true
	}
	allowed := map[string]struct{}{}
	addOrigin := func(raw string) {
		parsedOrigin := normalizeOrigin(raw)
		if parsedOrigin != "" {
			allowed[parsedOrigin] = struct{}{}
		}
	}
	addOrigin(system_setting.ServerAddress)
	addOrigin(os.Getenv("FRONTEND_BASE_URL"))
	if _, ok := allowed[normalizeOrigin(origin)]; ok {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if !common.DebugEnabled && os.Getenv("GIN_MODE") != "debug" {
		return false
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func normalizeOrigin(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
