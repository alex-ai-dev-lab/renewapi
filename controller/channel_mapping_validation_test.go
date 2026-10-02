package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func mappingRequest(t *testing.T, router *gin.Engine, method, path string, payload any, version string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if version != "" {
		req.Header.Set("If-Match", version)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestChannelMappingWriteEntrypointsRejectInvalid(t *testing.T) {
	for _, raw := range []string{`null`, `{`, `{"a":"b","b":"a"}`, `{"version":2,"rules":[{"id":"r","from":"a","to":"b","priority":1.5}]}`, `{"version":2,"rules":[{"id":"r","from":"a","to":"b","priority":2147483648}]}`} {
		for _, entry := range []string{"create", "legacy", "config", "tag"} {
			t.Run(entry+raw, func(t *testing.T) {
				db := setupChannelUpdateControllerTestDB(t)
				tag := "test"
				channel := model.Channel{Type: 1, Key: "secret", Name: "before", Models: "explicit", Group: "default", Tag: &tag, Status: common.ChannelStatusEnabled}
				router := gin.New()
				router.POST("/channel", AddChannel)
				router.PUT("/channel", UpdateChannel)
				router.PUT("/channel/:id/config", UpdateChannelConfig)
				router.PUT("/tag", EditTagChannels)
				var recorder *httptest.ResponseRecorder
				if entry == "create" {
					channel.ModelMapping = &raw
					recorder = mappingRequest(t, router, http.MethodPost, "/channel", map[string]any{"mode": "single", "channel": channel}, "")
				} else {
					require.NoError(t, db.Create(&channel).Error)
					require.NoError(t, channel.AddAbilities(db))
					path, version := "/channel", ""
					payload := map[string]any{"id": channel.Id, "model_mapping": raw}
					if entry == "config" {
						path = "/channel/" + strconv.Itoa(channel.Id) + "/config"
						version = `"channel-1"`
					}
					if entry == "tag" {
						path = "/tag"
						payload = map[string]any{"tag": tag, "model_mapping": raw}
					}
					recorder = mappingRequest(t, router, http.MethodPut, path, payload, version)
				}
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				var result struct {
					Success bool
					Message string
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
				require.False(t, result.Success, recorder.Body.String())
				require.Contains(t, result.Message, "模型映射")
				var count int64
				require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&count).Error)
				require.Zero(t, count)
				if entry == "create" {
					require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
					require.Zero(t, count)
				} else {
					var stored model.Channel
					require.NoError(t, db.First(&stored, channel.Id).Error)
					require.Nil(t, stored.ModelMapping)
				}
			})
		}
	}
}

func TestChannelMappingLegacyPatchRejectsStaleMergedSnapshot(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	m1 := `{"version":2,"rules":[{"id":"r","from":"old-source","to":"old-target"}]}`
	m2 := `{"version":2,"rules":[{"id":"r","from":"new-source","to":"new-target"}]}`
	channel := model.Channel{Type: 1, Key: "secret", Name: "before", Models: "explicit", Group: "default", ModelMapping: &m1, Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(db))

	// Commit writer B immediately after A's controller has loaded M1/version 1,
	// before A merges its name-only patch and enters the service transaction.
	injected := false
	const hook = "test:concurrent_mapping_write"
	require.NoError(t, db.Callback().Query().After("gorm:after_query").Register(hook, func(query *gorm.DB) {
		if injected || query.Statement.Schema == nil || query.Statement.Schema.Name != "Channel" {
			return
		}
		injected = true
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{"model_mapping": m2, "config_version": 2}).Error; err != nil {
				return err
			}
			newChannel := channel
			newChannel.ModelMapping = &m2
			return newChannel.UpdateAbilities(tx)
		}))
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(hook)) })

	router := gin.New()
	router.PUT("/channel", UpdateChannel)
	result := mappingRequest(t, router, http.MethodPut, "/channel", map[string]any{"id": channel.Id, "name": "stale-name"}, "")
	require.True(t, injected)
	require.Equal(t, http.StatusConflict, result.Code, result.Body.String())
	require.Contains(t, result.Body.String(), "CHANNEL_CONFIG_CONFLICT")
	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, m2, stored.GetModelMapping())
	require.Equal(t, "before", stored.Name)
	require.Equal(t, int64(2), stored.ConfigVersion)
	var sources []string
	require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Pluck("model", &sources).Error)
	require.ElementsMatch(t, []string{"explicit", "new-source"}, sources)
	var audits int64
	require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&audits).Error)
	require.Zero(t, audits)
}

func TestChannelMappingHistoricalAndV2HTTPUpdates(t *testing.T) {
	for _, entry := range []string{"legacy", "config"} {
		t.Run(entry, func(t *testing.T) {
			db := setupChannelUpdateControllerTestDB(t)
			raw := "null"
			channel := model.Channel{Type: 1, Key: "secret", Name: "before", Models: "explicit", Group: "default", ModelMapping: &raw, Status: common.ChannelStatusEnabled}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, channel.AddAbilities(db))
			router := gin.New()
			router.PUT("/channel", UpdateChannel)
			router.PUT("/channel/:id/config", UpdateChannelConfig)
			path, version := "/channel", ""
			if entry == "config" {
				path = "/channel/" + strconv.Itoa(channel.Id) + "/config"
				version = `"channel-1"`
			}
			result := mappingRequest(t, router, http.MethodPut, path, map[string]any{"id": channel.Id, "name": "after"}, version)
			require.Equal(t, http.StatusOK, result.Code, result.Body.String())
			require.Contains(t, result.Body.String(), `"success":true`)
			v2 := `{"version":2,"rules":[{"id":"a","from":"A","to":"B"},{"id":"b","from":"B","to":"A"}]}`
			if entry == "config" {
				version = `"channel-2"`
			}
			result = mappingRequest(t, router, http.MethodPut, path, map[string]any{"id": channel.Id, "model_mapping": v2}, version)
			require.Contains(t, result.Body.String(), `"success":true`)
			if entry == "config" {
				version = `"channel-3"`
			}
			result = mappingRequest(t, router, http.MethodPut, path, map[string]any{"id": channel.Id, "model_mapping": `{"A":"B"}`}, version)
			require.Equal(t, http.StatusOK, result.Code, result.Body.String())
			require.Contains(t, result.Body.String(), `"success":false`)
			var stored model.Channel
			require.NoError(t, db.First(&stored, channel.Id).Error)
			require.Equal(t, v2, stored.GetModelMapping())
		})
	}
}
