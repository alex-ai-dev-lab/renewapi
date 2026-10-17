package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenOrUserAuthRejectsDisabledUserDespiteEnabledSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldRedis := common.RedisEnabled
	t.Cleanup(func() { model.DB, common.RedisEnabled = oldDB, oldRedis })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RedisEnabled = false
	user := model.User{Username: "disabled", Password: "password123", Status: common.UserStatusDisabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("secret"))))
	router.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("status", common.UserStatusEnabled)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/video", TokenOrUserAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })

	seed := httptest.NewRecorder()
	router.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/session", nil))
	req := httptest.NewRequest(http.MethodGet, "/video", nil)
	for _, cookie := range seed.Result().Cookies() {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	require.Equal(t, http.StatusForbidden, response.Code)
}

func TestPanelSessionUsesCurrentAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	t.Cleanup(func() { model.DB = oldDB })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	user := model.User{Username: "demoted", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("test-signing-secret"))))
	router.GET("/session", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", user.Id)
		s.Set("username", user.Username)
		s.Set("role", common.RoleAdminUser)
		s.Set("status", common.UserStatusEnabled)
		require.NoError(t, s.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/admin", AdminAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/user", UserAuth(), func(c *gin.Context) {
		require.Equal(t, common.RoleCommonUser, c.GetInt("role"))
		require.Equal(t, "new-group", c.GetString("group"))
		c.Status(http.StatusOK)
	})
	seed := httptest.NewRecorder()
	router.ServeHTTP(seed, httptest.NewRequest("GET", "/session", nil))
	request := func(path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("New-Api-User", fmt.Sprint(user.Id))
		for _, co := range seed.Result().Cookies() {
			req.AddCookie(co)
		}
		router.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusOK, request("/admin"))
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{"role": common.RoleCommonUser, "group": "new-group"}).Error)
	queries := 0
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("audit:count", func(*gorm.DB) { queries++ }))
	require.Equal(t, http.StatusForbidden, request("/admin"))
	require.Equal(t, 1, queries)
	queries = 0
	require.Equal(t, http.StatusOK, request("/user"))
	require.Equal(t, 1, queries)
	require.NoError(t, db.Model(&user).Update("status", common.UserStatusDisabled).Error)
	require.Equal(t, http.StatusForbidden, request("/user"))
	require.NoError(t, db.Delete(&user).Error)
	require.Equal(t, http.StatusUnauthorized, request("/admin"))
	require.NoError(t, db.Migrator().DropTable(&model.User{}))
	require.Equal(t, http.StatusInternalServerError, request("/admin"))
}
