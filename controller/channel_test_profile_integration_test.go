package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestChannelTestPromptCRUD(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelTestPrompt{}))
	r := gin.New()
	r.GET("/prompts", GetChannelTestPrompts)
	r.POST("/prompts", CreateChannelTestPrompt)
	r.PUT("/prompts/:id", UpdateChannelTestPrompt)
	r.DELETE("/prompts/:id", DeleteChannelTestPrompt)
	call := func(method, path, body string) gjson.Result {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return gjson.Parse(w.Body.String())
	}
	created := call("POST", "/prompts", `{"name":"默认","prompt":"请计算 2+2","is_default":true}`)
	require.True(t, created.Get("success").Bool(), created.String())
	id := created.Get("data.id").Int()
	require.True(t, created.Get("data.enabled").Bool())
	channel := model.Channel{Name: "引用", Key: "test", ChannelTestPromptID: int(id)}
	require.NoError(t, db.Create(&channel).Error)
	list := call("GET", "/prompts", "")
	require.EqualValues(t, 1, list.Get("data.0.reference_count").Int())
	path := fmt.Sprintf("/prompts/%d", id)
	require.False(t, call("DELETE", path, "").Get("success").Bool())
	updated := call("PUT", path, `{"name":"已停用","prompt":"新内容","enabled":false,"is_default":true}`)
	require.True(t, updated.Get("success").Bool(), updated.String())
	require.False(t, updated.Get("data.is_default").Bool())
	require.NoError(t, db.Delete(&channel).Error)
	require.True(t, call("DELETE", path, "").Get("success").Bool())
	require.False(t, call("POST", "/prompts", `{"name":"","prompt":""}`).Get("success").Bool())
}

func TestAutomaticChannelTestOnlyAutoDisabledAndPerChannelPrompt(t *testing.T) {
	db := setupEmptyStreamRecoveryDB(t)
	seedRecoveryPrincipal(t, db)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("role", common.RoleRootUser).Error)
	require.NoError(t, db.AutoMigrate(&model.ChannelTestPrompt{}))
	previousTest := operation_setting.ChannelTestSetting2JsonString()
	previousMonitor := *operation_setting.GetMonitorSetting()
	previousTracking := testTracking
	previousEnable := common.AutomaticEnableChannelEnabled
	t.Cleanup(func() {
		_ = operation_setting.UpdateChannelTestSettingByJsonString(previousTest)
		*operation_setting.GetMonitorSetting() = previousMonitor
		testTracking = previousTracking
		common.AutomaticEnableChannelEnabled = previousEnable
	})
	require.NoError(t, operation_setting.UpdateChannelTestSettingByJsonString(`{"auto_test_only_auto_disabled":true,"stream_mode":"off","endpoint_type":"openai"}`))
	operation_setting.GetMonitorSetting().AutoTestChannelEnabled = true
	common.AutomaticEnableChannelEnabled = true
	testTracking = &channelTestTracker{channelLastTest: make(map[int]time.Time), channelFailCount: make(map[int]int)}
	var mu sync.Mutex
	calls := make(map[string][]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls[r.URL.Path] = append(calls[r.URL.Path], gjson.GetBytes(body, "messages.0.content").String())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl_test","object":"chat.completion","model":"recovery-integration-model","choices":[{"index":0,"message":{"role":"assistant","content":"4"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
	}))
	t.Cleanup(server.Close)
	channels := make([]*model.Channel, 0, 3)
	for i, status := range []int{common.ChannelStatusEnabled, common.ChannelStatusAutoDisabled, common.ChannelStatusManuallyDisabled} {
		prompt := model.ChannelTestPrompt{Name: fmt.Sprintf("配置%d", i), Prompt: fmt.Sprintf("渠道 %d 的独立测试", i), Enabled: true}
		require.NoError(t, model.SaveChannelTestPrompt(&prompt, true))
		channel := recoveryIntegrationChannel(8000+i, fmt.Sprintf("test-%d", i), server.URL+fmt.Sprintf("/%d", i), 1, failoverTestModel)
		channel.Status, channel.ChannelTestPromptID = status, prompt.ID
		channel.SetSetting(dto.ChannelSettings{AutoTestTimeWindowStart: "00:00", AutoTestTimeWindowEnd: "23:59", AntiPoisonEnabled: common.GetPointer(false)})
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
		channels = append(channels, channel)
	}
	model.InitChannelCache()
	runIndependentChannelTest(context.Background())
	mu.Lock()
	require.Empty(t, calls["/0/v1/chat/completions"])
	require.Equal(t, []string{"渠道 1 的独立测试"}, calls["/1/v1/chat/completions"])
	require.Empty(t, calls["/2/v1/chat/completions"])
	mu.Unlock()
	var recovered model.Channel
	require.NoError(t, db.First(&recovered, channels[1].Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, recovered.Status)
	// 手动单测不受自动测试筛选影响，且仍使用该渠道自己的提示词。
	result := testChannel(channels[0], 1, "", "openai", false)
	require.NoError(t, result.localErr)
	require.Nil(t, result.newAPIError)
	mu.Lock()
	require.Equal(t, []string{"渠道 0 的独立测试"}, calls["/0/v1/chat/completions"])
	mu.Unlock()

	unsupported := channels[2]
	unsupported.Status, unsupported.Type = common.ChannelStatusAutoDisabled, constant.ChannelTypeMidjourney
	require.NoError(t, db.Save(unsupported).Error)
	testSingleChannelWithRetries(context.Background(), unsupported, 1, 1, 1)
	recovered = model.Channel{}
	require.NoError(t, db.First(&recovered, unsupported.Id).Error)
	require.Equal(t, common.ChannelStatusAutoDisabled, recovered.Status)
}

func TestChannelTestAntiPoisonKeepsAdministratorPrompt(t *testing.T) {
	for _, endpoint := range []string{"openai", "openai-response", "anthropic", "gemini"} {
		request := buildTestRequest("test", endpoint, &model.Channel{}, false, "NEWAPI_TEST_unique", "请计算 2+2")
		encoded, err := common.Marshal(request)
		require.NoError(t, err)
		require.Contains(t, string(encoded), "请计算 2+2")
		require.Contains(t, string(encoded), "NEWAPI_TEST_unique")
		require.NoError(t, validateTestResponseBody([]byte(`{"choices":[{"message":{"content":"4\nNEWAPI_TEST_unique"}}]}`), false, "NEWAPI_TEST_unique"))
	}
}
