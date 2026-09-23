package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestUnsupportedChannelTestReturnsErrorWithoutPanic(t *testing.T) {
	for _, tc := range []struct {
		name        string
		channelType int
		endpoint    string
	}{
		{name: "异步音乐渠道", channelType: constant.ChannelTypeSunoAPI},
		{name: "音频端点", channelType: constant.ChannelTypeOpenAI, endpoint: string(constant.EndpointTypeAudioSpeech)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupChannelUpdateControllerTestDB(t)
			channel := model.Channel{Name: tc.name, Type: tc.channelType, Key: "unused-test-key", Models: "test-model", Status: common.ChannelStatusEnabled, ResponseTime: 123}
			require.NoError(t, db.Create(&channel).Error)
			router := gin.New()
			router.GET("/test/:id", func(c *gin.Context) {
				c.Set("id", 1)
				TestChannel(c)
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test/%d?endpoint_type=%s", channel.Id, url.QueryEscape(tc.endpoint)), nil)
			require.NotPanics(t, func() { router.ServeHTTP(recorder, request) })
			require.Equal(t, http.StatusOK, recorder.Code)
			require.False(t, gjson.Get(recorder.Body.String(), "success").Bool())
			require.Contains(t, gjson.Get(recorder.Body.String(), "message").String(), "not supported")
			var unchanged model.Channel
			require.NoError(t, db.First(&unchanged, channel.Id).Error)
			require.Equal(t, 123, unchanged.ResponseTime)
			require.Equal(t, common.ChannelStatusEnabled, unchanged.Status)
		})
	}
}

func TestUnsupportedAutomaticChannelTestPreservesMeasurements(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	channel := model.Channel{Name: "不支持的自动测试", Type: constant.ChannelTypeSunoAPI, Key: "unused-test-key", Models: "test-model", Status: common.ChannelStatusEnabled, ResponseTime: 123}
	require.NoError(t, db.Create(&channel).Error)
	testSingleChannelWithRetries(context.Background(), &channel, 1, 1, 1)
	var unchanged model.Channel
	require.NoError(t, db.First(&unchanged, channel.Id).Error)
	require.Equal(t, 123, unchanged.ResponseTime)
	require.Equal(t, common.ChannelStatusEnabled, unchanged.Status)
}

func TestInteractiveChannelTestCancellationStopsUpstream(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	seedRecoveryPrincipal(t, db)
	require.NoError(t, db.AutoMigrate(&model.ChannelTestPrompt{}))
	previous := operation_setting.ChannelTestSetting2JsonString()
	t.Cleanup(func() { _ = operation_setting.UpdateChannelTestSettingByJsonString(previous) })
	require.NoError(t, operation_setting.UpdateChannelTestSettingByJsonString(`{"stream_mode":"off","endpoint_type":"openai"}`))
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-release:
		}
	}))
	defer upstream.Close()
	defer close(release)
	channel := recoveryIntegrationChannel(8201, "可取消的测试", upstream.URL, 1, failoverTestModel)
	channel.ResponseTime = 123
	channel.SetSetting(dto.ChannelSettings{AntiPoisonEnabled: common.GetPointer(false)})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()
	router := gin.New()
	router.GET("/test/:id", func(c *gin.Context) { c.Set("id", 1); TestChannel(c) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test/%d?endpoint_type=openai", channel.Id), nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); router.ServeHTTP(recorder, request) }()
	select {
	case <-entered:
	case <-done:
		t.Fatalf("未发起预期测试请求：%s", recorder.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("上游测试请求未开始")
	}
	cancel()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("客户端取消未传递到上游")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("取消后处理器未退出")
	}
	var unchanged model.Channel
	require.NoError(t, db.First(&unchanged, channel.Id).Error)
	require.Equal(t, 123, unchanged.ResponseTime)
	require.Equal(t, common.ChannelStatusEnabled, unchanged.Status)
}
