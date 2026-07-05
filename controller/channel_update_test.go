package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelStatus{}))
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
