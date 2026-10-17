package controller

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVideoProxyUpstreamFailureRedactsSignedURLAndStoredKeys(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	previousMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemory })
	fetchSetting := system_setting.GetFetchSetting()
	previousFetch := *fetchSetting
	fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp = false, true
	t.Cleanup(func() { *fetchSetting = previousFetch })
	const secret = "fake+stored/key"
	const signature = "fake-signed-video-credential"
	received := make(chan bool, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Query().Get("key") == secret
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()
	channel := model.Channel{Name: "audit-video", Key: "sk-fake-channel-private-key", Type: 0}
	require.NoError(t, db.Create(&channel).Error)
	task := model.Task{TaskID: "audit-video", UserId: 71, ChannelId: channel.Id, Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{Key: secret, ResultURL: upstream.URL + "/video?key=" + url.QueryEscape(secret) + "&signature=" + signature}}
	require.NoError(t, db.Create(&task).Error)
	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter, previousErrorWriter := gin.DefaultWriter, gin.DefaultErrorWriter
	gin.DefaultWriter, gin.DefaultErrorWriter = &logs, &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter, gin.DefaultErrorWriter = previousWriter, previousErrorWriter
		common.LogWriterMu.Unlock()
	})
	router := gin.New()
	router.GET("/video/:task_id", func(c *gin.Context) { c.Set("id", 71); VideoProxy(c) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/video/audit-video", nil))
	require.True(t, <-received, "real upstream request keeps its credential")
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, logs.String(), "503")
	for _, credential := range []string{secret, url.QueryEscape(secret), channel.Key, signature} {
		require.NotContains(t, logs.String(), credential)
		require.NotContains(t, recorder.Body.String(), credential)
	}
}

func TestCopyVideoResponseHeadersOverridesPublicCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	copyVideoResponseHeaders(ctx, http.Header{"Cache-Control": {"public, max-age=86400"}, "Content-Type": {"video/mp4"}})
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", recorder.Header().Get("Pragma"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
}

func TestWriteVideoDataURLPrivateAndBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_PROXY_MAX_BYTES", "4")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	require.NoError(t, writeVideoDataURL(ctx, "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("1234"))))
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "1234", recorder.Body.String())

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	err := writeVideoDataURL(ctx, "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("12345")))
	require.ErrorContains(t, err, "exceeds")
	require.Empty(t, recorder.Body.String())
}
