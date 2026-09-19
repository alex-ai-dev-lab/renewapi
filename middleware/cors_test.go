package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsOriginAllowedProduction(t *testing.T) {
	serverAddress := "https://api.example.com/"
	frontendBaseURL := "https://app.example.com/app"

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{name: "server origin", origin: "https://api.example.com", allowed: true},
		{name: "frontend origin", origin: "https://app.example.com", allowed: true},
		{name: "default https port", origin: "https://app.example.com:443", allowed: true},
		{name: "different scheme", origin: "http://app.example.com", allowed: false},
		{name: "different port", origin: "https://app.example.com:8443", allowed: false},
		{name: "trusted prefix attack", origin: "https://app.example.com.evil.test", allowed: false},
		{name: "trusted suffix attack", origin: "https://evil-app.example.com", allowed: false},
		{name: "localhost disabled in production", origin: "http://localhost:3000", allowed: false},
		{name: "ipv4 loopback disabled in production", origin: "http://127.0.0.1:3000", allowed: false},
		{name: "null origin", origin: "null", allowed: false},
		{name: "malformed origin", origin: "not-an-origin", allowed: false},
		{name: "origin with path", origin: "https://app.example.com/path", allowed: false},
		{name: "origin with query", origin: "https://app.example.com?x=1", allowed: false},
		{name: "origin with credentials", origin: "https://user@app.example.com", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isOriginAllowed(test.origin, serverAddress, frontendBaseURL, false)
			if got != test.allowed {
				t.Fatalf("isOriginAllowed(%q) = %v, want %v", test.origin, got, test.allowed)
			}
		})
	}
}

func TestIsOriginAllowedDebugLoopback(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{name: "localhost", origin: "http://localhost:3000", allowed: true},
		{name: "localhost https", origin: "https://localhost:5173", allowed: true},
		{name: "ipv4 loopback", origin: "http://127.0.0.1:3000", allowed: true},
		{name: "ipv6 loopback", origin: "http://[::1]:3000", allowed: true},
		{name: "localhost suffix attack", origin: "http://localhost.evil.test:3000", allowed: false},
		{name: "other loopback address", origin: "http://127.0.0.2:3000", allowed: false},
		{name: "other ipv6 address", origin: "http://[::2]:3000", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isOriginAllowed(test.origin, "", "", true)
			if got != test.allowed {
				t.Fatalf("isOriginAllowed(%q) = %v, want %v", test.origin, got, test.allowed)
			}
		})
	}
}

func TestCORSDebugEnabledByGinMode(t *testing.T) {
	t.Setenv("GIN_MODE", gin.DebugMode)

	if !corsDebugEnabled() {
		t.Fatal("corsDebugEnabled() = false with GIN_MODE=debug, want true")
	}
	if !isOriginAllowed(
		"http://localhost:5173",
		"https://api.example.com",
		"https://app.example.com",
		corsDebugEnabled(),
	) {
		t.Fatal("localhost origin rejected with GIN_MODE=debug")
	}
}

func TestCORSAllowedCredentialedRequest(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "https://app.example.com")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.example.com")
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
	if !headerContainsToken(response.Header().Get("Access-Control-Expose-Headers"), "X-New-Api-Version") {
		t.Fatalf("Access-Control-Expose-Headers = %q, missing X-New-Api-Version", response.Header().Get("Access-Control-Expose-Headers"))
	}
	if !headerContainsToken(response.Header().Get("Access-Control-Expose-Headers"), "X-Request-Id") {
		t.Fatalf("Access-Control-Expose-Headers = %q, missing X-Request-Id", response.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestCORSAllowedPreflight(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodOptions, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, New-Api-User, X-Request-Id")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.example.com")
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
	if !headerContainsToken(response.Header().Get("Access-Control-Allow-Methods"), http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods = %q, missing POST", response.Header().Get("Access-Control-Allow-Methods"))
	}
	for _, header := range []string{"Authorization", "Content-Type", "New-Api-User", "X-Request-Id"} {
		if !headerContainsToken(response.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", response.Header().Get("Access-Control-Allow-Headers"), header)
		}
	}
}

func TestCORSAllowedPatchPreflight(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodOptions, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("PATCH preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.example.com")
	}
	if !headerContainsToken(response.Header().Get("Access-Control-Allow-Methods"), http.MethodPatch) {
		t.Fatalf("Access-Control-Allow-Methods = %q, missing PATCH", response.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSRejectsUntrustedOrigin(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodOptions, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "https://app.example.com.evil.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("untrusted preflight status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q for untrusted origin, want empty", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q for untrusted origin, want empty", got)
	}
}

func TestCORSProductionRejectsDebugOrigin(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "http://localhost:3000")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("localhost status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestCORSDebugAllowsLoopbackOrigin(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", true)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("Origin", "http://localhost:5173")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
}

func TestCORSRequestWithoutOriginIsUnaffected(t *testing.T) {
	router := newCORSTestRouter("https://api.example.com", "https://app.example.com", false)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Host = "api.example.com"

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q without Origin header, want empty", got)
	}
}

func newCORSTestRouter(serverAddress string, frontendBaseURL string, debug bool) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(newCORSMiddleware(serverAddress, frontendBaseURL, debug))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	return router
}

func headerContainsToken(value string, token string) bool {
	for _, candidate := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), token) {
			return true
		}
	}
	return false
}
