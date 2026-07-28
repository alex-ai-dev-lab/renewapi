package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelUpdateControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RedisEnabled = oldRedisEnabled
	})

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelStatus{}, &model.ConfigAudit{}, &model.ModelEndpoint{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	return db
}

func TestUpdateChannelStatusPatchPreservesExistingFields(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)

	priority := int64(997)
	weight := uint(3)
	autoBan := 1
	baseURL := "https://muyuan.do"
	setting := `{"proxy":"http://127.0.0.1:10808"}`
	otherSettings := `{"anti_poison_guard_enabled":true}`
	channel := model.Channel{
		Id:            71,
		Type:          14,
		Key:           "sk-existing",
		Status:        common.ChannelStatusAutoDisabled,
		Name:          "君",
		Weight:        &weight,
		BaseURL:       &baseURL,
		Models:        "claude-opus-4-6",
		Group:         "Test,default",
		Priority:      &priority,
		AutoBan:       &autoBan,
		Setting:       &setting,
		OtherSettings: otherSettings,
		CreatedTime:   common.GetTimestamp(),
		UsedQuota:     12345,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	router := gin.New()
	router.PUT("/api/channel/", UpdateChannel)
	body, err := json.Marshal(map[string]any{
		"id":     channel.Id,
		"status": common.ChannelStatusEnabled,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	var got model.Channel
	require.NoError(t, db.First(&got, channel.Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, got.Status)
	require.Equal(t, channel.Name, got.Name)
	require.Equal(t, channel.Type, got.Type)
	require.Equal(t, channel.Key, got.Key)
	require.Equal(t, channel.Models, got.Models)
	require.Equal(t, channel.Group, got.Group)
	require.NotNil(t, got.Priority)
	require.Equal(t, priority, *got.Priority)
	require.NotNil(t, got.Weight)
	require.Equal(t, weight, *got.Weight)
	require.NotNil(t, got.BaseURL)
	require.Equal(t, baseURL, *got.BaseURL)
	require.NotNil(t, got.Setting)
	require.Equal(t, setting, *got.Setting)
	require.Equal(t, otherSettings, got.OtherSettings)
	require.Equal(t, channel.UsedQuota, got.UsedQuota)
}

func TestUpdateChannelConfigRequiresIfMatch(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	channel := model.Channel{Type: 1, Key: "sk-test", Status: common.ChannelStatusEnabled, Name: "before", Models: "gpt-test", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(db))

	router := gin.New()
	router.PUT("/api/channel/:id/config", UpdateChannelConfig)
	body, err := common.Marshal(map[string]any{"name": "after"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPut, "/api/channel/"+strconv.Itoa(channel.Id)+"/config", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusPreconditionRequired, recorder.Code)

	request = httptest.NewRequest(http.MethodPut, "/api/channel/"+strconv.Itoa(channel.Id)+"/config", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("If-Match", `"channel-1"`)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, `"channel-2"`, recorder.Header().Get("ETag"))
}

func TestAddChannelRejectsNilChannelWithoutPanic(t *testing.T) {
	setupChannelUpdateControllerTestDB(t)

	router := gin.New()
	router.POST("/api/channel/", AddChannel)
	body, err := json.Marshal(map[string]any{
		"mode":    "single",
		"channel": nil,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	require.NotPanics(t, func() {
		router.ServeHTTP(recorder, req)
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Contains(t, response.Message, "channel cannot be empty")
}

func TestBatchAddChannelKeyPrefixNamesUseOriginalName(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)

	router := gin.New()
	router.POST("/api/channel/", AddChannel)
	body, err := json.Marshal(map[string]any{
		"mode":                            "batch",
		"batch_add_set_key_prefix_2_name": true,
		"channel": map[string]any{
			"type":   1,
			"key":    "alpha123456\nbeta123456",
			"status": common.ChannelStatusEnabled,
			"name":   "Batch",
			"models": "gpt-4o",
			"group":  "default",
		},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	var channels []model.Channel
	require.NoError(t, db.Order("id asc").Find(&channels).Error)
	require.Len(t, channels, 2)
	require.Equal(t, "Batch alpha123", channels[0].Name)
	require.Equal(t, "Batch beta1234", channels[1].Name)
}

func TestAddChannelRejectsWhitespaceOnlyKey(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)

	router := gin.New()
	router.POST("/api/channel/", AddChannel)
	body, err := json.Marshal(map[string]any{
		"mode": "batch",
		"channel": map[string]any{
			"type":   1,
			"key":    "   \n\t",
			"status": common.ChannelStatusEnabled,
			"name":   "Blank",
			"models": "gpt-4o",
			"group":  "default",
		},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Contains(t, response.Message, "channel cannot be empty")

	var count int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateChannelRejectsTooLongModelName(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)

	channel := model.Channel{
		Id:          81,
		Type:        1,
		Key:         "sk-existing",
		Status:      common.ChannelStatusEnabled,
		Name:        "OpenAI",
		Models:      "gpt-4o",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	router := gin.New()
	router.PUT("/api/channel/", UpdateChannel)
	body, err := json.Marshal(map[string]any{
		"id":     channel.Id,
		"models": string(bytes.Repeat([]byte("a"), 256)),
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Contains(t, response.Message, "模型名称过长")

	var got model.Channel
	require.NoError(t, db.First(&got, channel.Id).Error)
	require.Equal(t, channel.Models, got.Models)
}

func TestUpdateChannelRejectsEmptyModelAndGroupLists(t *testing.T) {
	testCases := []struct {
		name          string
		payload       map[string]any
		wantMessage   string
		wantModels    string
		wantGroup     string
		wantAbilities int64
	}{
		{
			name:          "empty models",
			payload:       map[string]any{"models": " , "},
			wantMessage:   "模型不能为空",
			wantModels:    "gpt-4o",
			wantGroup:     "default",
			wantAbilities: 1,
		},
		{
			name:          "empty groups",
			payload:       map[string]any{"group": " , "},
			wantMessage:   "分组不能为空",
			wantModels:    "gpt-4o",
			wantGroup:     "default",
			wantAbilities: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupChannelUpdateControllerTestDB(t)
			channel := model.Channel{
				Id:          91,
				Type:        1,
				Key:         "sk-existing",
				Status:      common.ChannelStatusEnabled,
				Name:        "OpenAI",
				Models:      tc.wantModels,
				Group:       tc.wantGroup,
				CreatedTime: common.GetTimestamp(),
			}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, channel.AddAbilities(nil))

			router := gin.New()
			router.PUT("/api/channel/", UpdateChannel)
			bodyPayload := map[string]any{"id": channel.Id}
			for key, value := range tc.payload {
				bodyPayload[key] = value
			}
			body, err := json.Marshal(bodyPayload)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Contains(t, response.Message, tc.wantMessage)

			var got model.Channel
			require.NoError(t, db.First(&got, channel.Id).Error)
			require.Equal(t, tc.wantModels, got.Models)
			require.Equal(t, tc.wantGroup, got.Group)
			var count int64
			require.NoError(t, db.Model(&model.Ability{}).Count(&count).Error)
			require.Equal(t, tc.wantAbilities, count)
		})
	}
}

func TestChannelAbilitiesIgnoreBlankModelAndGroupItems(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)

	channel := model.Channel{
		Id:          101,
		Type:        1,
		Key:         "sk-existing",
		Status:      common.ChannelStatusEnabled,
		Name:        "OpenAI",
		Models:      "gpt-4o, ,gpt-4.1,",
		Group:       "default, ,vip,",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	var abilities []model.Ability
	require.NoError(t, db.Order("`group`, model").Find(&abilities).Error)
	require.Len(t, abilities, 4)
	for _, ability := range abilities {
		require.NotEmpty(t, ability.Group)
		require.NotEmpty(t, ability.Model)
	}
}
