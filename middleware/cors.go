package middleware

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type corsOrigin struct {
	value    string
	hostname string
}

func CORS() gin.HandlerFunc {
	return newCORSMiddleware(
		system_setting.ServerAddress,
		os.Getenv("FRONTEND_BASE_URL"),
		corsDebugEnabled(),
	)
}

func corsDebugEnabled() bool {
	return common.DebugEnabled || os.Getenv("GIN_MODE") == gin.DebugMode
}

func newCORSMiddleware(serverAddress string, frontendBaseURL string, debug bool) gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowMethods = []string{
		"GET",
		"POST",
		"PUT",
		"PATCH",
		"DELETE",
		"OPTIONS",
	}
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
	config.ExposeHeaders = []string{
		"X-New-Api-Version",
		"X-Request-Id",
	}
	config.AllowOriginFunc = func(origin string) bool {
		return isOriginAllowed(origin, serverAddress, frontendBaseURL, debug)
	}

	return cors.New(config)
}

func isOriginAllowed(origin string, serverAddress string, frontendBaseURL string, debug bool) bool {
	requestOrigin, ok := parseRequestOrigin(origin)
	if !ok {
		return false
	}

	if configuredOrigin, ok := parseConfiguredOrigin(serverAddress); ok &&
		requestOrigin.value == configuredOrigin.value {
		return true
	}

	if configuredOrigin, ok := parseConfiguredOrigin(frontendBaseURL); ok &&
		requestOrigin.value == configuredOrigin.value {
		return true
	}

	if !debug {
		return false
	}

	switch requestOrigin.hostname {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func parseRequestOrigin(raw string) (corsOrigin, bool) {
	parsed, err := url.Parse(raw)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Path != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return corsOrigin{}, false
	}

	return normalizeOrigin(parsed)
}

func parseConfiguredOrigin(raw string) (corsOrigin, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return corsOrigin{}, false
	}

	parsed, err := url.Parse(raw)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" ||
		parsed.User != nil {
		return corsOrigin{}, false
	}

	return normalizeOrigin(parsed)
}

func normalizeOrigin(parsed *url.URL) (corsOrigin, bool) {
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return corsOrigin{}, false
	}

	hostName := strings.ToLower(parsed.Hostname())
	if hostName == "" {
		return corsOrigin{}, false
	}

	port := parsed.Port()
	if (scheme == "http" && port == "80") ||
		(scheme == "https" && port == "443") {
		port = ""
	}

	host := hostName
	if port != "" {
		host = net.JoinHostPort(hostName, port)
	} else if strings.Contains(hostName, ":") {
		host = "[" + hostName + "]"
	}

	return corsOrigin{
		value:    scheme + "://" + host,
		hostname: hostName,
	}, true
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
