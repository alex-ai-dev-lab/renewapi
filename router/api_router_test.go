package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestChannelCollectionRoutesKeepAuthenticationWithoutRedirect(t *testing.T) {
	originalMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(originalMode) })
	router := gin.New()
	router.Use(sessions.Sessions("channel-route-test", cookie.NewStore([]byte("channel-route-test-cookie-secret"))))
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	SetVideoRouter(router)

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		for _, path := range []string{"/api/channel", "/api/channel/"} {
			t.Run(method+path, func(t *testing.T) {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(method, path+"?p=1&page_size=100", nil))
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("渠道集合接口必须直接进入鉴权：%s %s 返回 %d，正文 %s", method, path, response.Code, response.Body.String())
				}
				if response.Header().Get("Location") != "" {
					t.Fatal("渠道集合接口不应依赖自动重定向")
				}
			})
		}
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/channel-test-prompts", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("提示词接口必须保留独立的管理员鉴权，返回 %d", response.Code)
	}
}
