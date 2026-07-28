package middleware

import (
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
