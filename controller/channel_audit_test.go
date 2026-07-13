package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelConfigAudits(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	require.NoError(t, db.Create(&model.ConfigAudit{
		ResourceType: "channel",
		ResourceId:   93,
		Action:       "update",
		OperatorId:   7,
		Reason:       "test",
		Diff:         `{"changed_fields":["name"]}`,
		CreatedAt:    common.GetTimestamp(),
	}).Error)

	router := gin.New()
	router.GET("/api/channel/:id/audit", GetChannelConfigAudits)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/channel/93/audit", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                `json:"success"`
		Data    []model.ConfigAudit `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	require.Equal(t, "test", response.Data[0].Reason)
}

func TestBuildChannelConfigAuditRejectsOversizedReason(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := buildChannelConfigAudit(context, &PatchChannel{
		Channel:      model.Channel{Id: 93},
		ChangeReason: strings.Repeat("变", 256),
	}, map[string]json.RawMessage{"name": json.RawMessage(`"test"`)})
	require.ErrorContains(t, err, "255")
}

func TestBuildChannelConfigAuditOmitsSensitiveValues(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", nil)
	endpoints := []modelEndpointPayload{{Model: "glm-5.2"}}
	audit, err := buildChannelConfigAudit(context, &PatchChannel{
		Channel:        model.Channel{Id: 93, Key: "replacement-secret"},
		ModelEndpoints: &endpoints,
	}, map[string]json.RawMessage{
		"key":             json.RawMessage(`"replacement-secret"`),
		"model_mapping":   json.RawMessage(`{"glm-5.2":["sensitive-upstream-model"]}`),
		"header_override": json.RawMessage(`{"Authorization":"sensitive-authorization"}`),
	})
	require.NoError(t, err)
	require.NotContains(t, audit.Diff, "replacement-secret")
	require.NotContains(t, audit.Diff, "sensitive-upstream-model")
	require.NotContains(t, audit.Diff, "sensitive-authorization")
	require.Contains(t, audit.Diff, `"key_changed":true`)
	require.Contains(t, audit.Diff, `"model_endpoint_count":1`)
}
