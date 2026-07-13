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
	"github.com/stretchr/testify/require"
)

func TestUpdateChannelAcceptsAtomicModelEndpointPayload(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ModelEndpoint{}))
	channel := model.Channel{
		Id:            93,
		Type:          1,
		Key:           "sk-existing",
		Status:        common.ChannelStatusEnabled,
		Name:          "before",
		Models:        "gpt-5.5",
		Group:         "default",
		CreatedTime:   common.GetTimestamp(),
		OtherSettings: "{}",
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	body, err := common.Marshal(map[string]any{
		"id":                      channel.Id,
		"expected_config_version": channel.ConfigVersion,
		"name":                    "after",
		"models":                  "gpt-5.5,gpt-5.6",
		"model_endpoints": []map[string]any{
			{
				"model":        "gpt-5.6",
				"base_url":     "https://example.com/v1",
				"channel_type": nil,
			},
		},
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 7)
		c.Set(common.RequestIdKey, "request-from-middleware")
	})
	router.PUT("/api/channel/", UpdateChannel)
	req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "after", stored.Name)
	require.Equal(t, "gpt-5.5,gpt-5.6", stored.Models)
	require.Equal(t, channel.ConfigVersion+1, stored.ConfigVersion)

	var endpoints []model.ModelEndpoint
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&endpoints).Error)
	require.Len(t, endpoints, 1)
	require.Equal(t, "gpt-5.6", endpoints[0].Model)

	var audits []model.ConfigAudit
	require.NoError(t, db.Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).Find(&audits).Error)
	require.Len(t, audits, 1)
	require.NotContains(t, audits[0].Diff, channel.Key)
	require.Contains(t, audits[0].Diff, "model_endpoint_count")
	require.Equal(t, "channel editor update", audits[0].Reason)
	require.Equal(t, "request-from-middleware", audits[0].RequestId)
	require.Equal(t, stored.ConfigVersion, audits[0].ConfigVersion)
}

func TestNormalizeModelEndpointPayloadsValidatesAndTrims(t *testing.T) {
	endpoints, err := normalizeModelEndpointPayloads(93, []modelEndpointPayload{{
		Model:   " glm-5.2 ",
		BaseURL: " https://example.com/v1 ",
	}})
	require.NoError(t, err)
	require.Len(t, endpoints, 1)
	require.Equal(t, "glm-5.2", endpoints[0].Model)
	require.Equal(t, "https://example.com/v1", endpoints[0].BaseURL)

	_, err = normalizeModelEndpointPayloads(93, []modelEndpointPayload{
		{Model: "glm-5.2"},
		{Model: " glm-5.2 "},
	})
	require.ErrorContains(t, err, "duplicate model endpoint")

	_, err = normalizeModelEndpointPayloads(93, []modelEndpointPayload{{
		Model:   "glm-5.2",
		BaseURL: "example.com/v1",
	}})
	require.ErrorContains(t, err, "absolute HTTP(S) URL")

	_, err = normalizeModelEndpointPayloads(93, []modelEndpointPayload{{Model: " "}})
	require.ErrorContains(t, err, "model is required")

	unknownType := 999
	_, err = normalizeModelEndpointPayloads(93, []modelEndpointPayload{{
		Model:       "glm-5.2",
		ChannelType: &unknownType,
	}})
	require.ErrorContains(t, err, "unknown channel_type")

	_, err = normalizeModelEndpointPayloads(93, []modelEndpointPayload{{
		Model:   "glm-5.2",
		BaseURL: "https://user:password@example.com/v1",
	}})
	require.ErrorContains(t, err, "must not contain user credentials")
}

func TestUpdateChannelExplicitlyClearsKey(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	channel := model.Channel{
		Id:            94,
		Type:          1,
		Key:           "sk-existing",
		Status:        common.ChannelStatusEnabled,
		Name:          "clear-key",
		Models:        "gpt-5.5",
		Group:         "default",
		CreatedTime:   common.GetTimestamp(),
		OtherSettings: "{}",
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	body, err := common.Marshal(map[string]any{
		"id":        channel.Id,
		"clear_key": true,
	})
	require.NoError(t, err)

	router := gin.New()
	router.PUT("/api/channel/", UpdateChannel)
	req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Empty(t, stored.Key)

	var audit model.ConfigAudit
	require.NoError(t, db.Where("resource_type = ? AND resource_id = ?", "channel", channel.Id).First(&audit).Error)
	require.Contains(t, audit.Diff, `"key_changed":true`)
	require.NotContains(t, audit.Diff, "sk-existing")
}

func TestUpdateChannelRejectsStaleConfigVersion(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	channel := model.Channel{
		Id: 95, Type: 1, Key: "sk-existing", Status: common.ChannelStatusEnabled,
		Name: "before", Models: "gpt-5.5", Group: "default", CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	current := channel.ConfigVersion
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", channel.Id).
		Update("config_version", current+1).Error)
	body, err := common.Marshal(map[string]any{
		"id": channel.Id, "name": "stale write", "expected_config_version": current,
	})
	require.NoError(t, err)
	router := gin.New()
	router.PUT("/api/channel/", UpdateChannel)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "CHANNEL_CONFIG_CONFLICT")
	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "before", stored.Name)
}

func TestResolveChannelKeyActionValidatesExplicitCommands(t *testing.T) {
	channel := &PatchChannel{Channel: model.Channel{Key: "replacement"}}
	action, err := resolveChannelKeyAction(channel, map[string]json.RawMessage{"key": json.RawMessage(`"replacement"`)})
	require.NoError(t, err)
	require.Equal(t, "replace", action)

	channel.KeyAction = "keep"
	_, err = resolveChannelKeyAction(channel, map[string]json.RawMessage{"key": json.RawMessage(`"replacement"`)})
	require.ErrorContains(t, err, "cannot include key")

	channel.KeyAction = "clear"
	_, err = resolveChannelKeyAction(channel, map[string]json.RawMessage{"key": json.RawMessage(`""`)})
	require.ErrorContains(t, err, "must not include key")
}
