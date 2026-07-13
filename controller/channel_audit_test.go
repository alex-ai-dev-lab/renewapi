package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelConfigAuditsReturnsStructuredSanitizedDTO(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	require.NoError(t, db.Create(&model.ConfigAudit{
		ResourceType:  "channel",
		ResourceId:    93,
		Action:        "update",
		OperatorId:    7,
		Reason:        "test",
		RequestId:     "request-test",
		Diff:          `{"changed_fields":["name"],"key_changed":true,"secret":"must-not-appear"}`,
		ConfigVersion: 4,
		CreatedAt:     common.GetTimestamp(),
	}).Error)

	router := gin.New()
	router.GET("/api/channel/:id/audit", GetChannelConfigAudits)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/channel/93/audit?limit=10", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "must-not-appear")
	require.NotContains(t, recorder.Body.String(), `"diff"`)
	var response struct {
		Success bool                    `json:"success"`
		Data    []channelConfigAuditDTO `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	require.Equal(t, "test", response.Data[0].Reason)
	require.Equal(t, int64(4), response.Data[0].ConfigVersion)
	require.Equal(t, []string{"name"}, response.Data[0].Summary.ChangedFields)
	require.True(t, response.Data[0].Summary.KeyChanged)
}

func TestGetChannelConfigAuditsSupportsBeforeID(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	for _, version := range []int64{1, 2, 3} {
		require.NoError(t, db.Create(&model.ConfigAudit{
			ResourceType:  "channel",
			ResourceId:    93,
			Action:        "update",
			Diff:          `{}`,
			ConfigVersion: version,
			CreatedAt:     common.GetTimestamp(),
		}).Error)
	}
	var middle model.ConfigAudit
	require.NoError(t, db.Where("resource_type = ? AND resource_id = ? AND config_version = ?", "channel", 93, 2).First(&middle).Error)

	router := gin.New()
	router.GET("/api/channel/:id/audit", GetChannelConfigAudits)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet,
		"/api/channel/93/audit?before_id="+strconv.FormatInt(middle.Id, 10),
		nil,
	))

	var response struct {
		Data []channelConfigAuditDTO `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	require.Equal(t, int64(1), response.Data[0].ConfigVersion)
}
