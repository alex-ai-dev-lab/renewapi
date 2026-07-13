package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/adminconfig"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelEffectiveConfigMasksSecretsAndReusesRouteDecision(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	priority := int64(999)
	weight := uint(4)
	baseURL := "https://example.com/v1"
	mapping := `{"glm-5.2":["@cf/zhipu-ai/glm-5.2","TCADP/glm-5.2"]}`
	channel := model.Channel{
		Id:           93,
		Type:         1,
		Key:          "must-not-appear",
		Status:       common.ChannelStatusEnabled,
		Name:         "effective-config",
		BaseURL:      &baseURL,
		Models:       "glm-5.2",
		Group:        "default,Test",
		Priority:     &priority,
		Weight:       &weight,
		ModelMapping: &mapping,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)

	router := gin.New()
	router.GET("/api/channel/:id/effective_config", GetChannelEffectiveConfig)
	req := httptest.NewRequest(http.MethodGet, "/api/channel/93/effective_config?model=glm-5.2", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), channel.Key)
	require.NotContains(t, recorder.Body.String(), mapping)
	require.NotContains(t, recorder.Body.String(), "@cf/zhipu-ai/glm-5.2")

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Route struct {
				Source string `json:"source"`
			} `json:"route"`
			Items []adminconfig.EffectiveValue `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.Route.Source)

	var keyItem *adminconfig.EffectiveValue
	var mappingItem *adminconfig.EffectiveValue
	for i := range response.Data.Items {
		if strings.EqualFold(response.Data.Items[i].Key, "key") {
			keyItem = &response.Data.Items[i]
		}
		if strings.EqualFold(response.Data.Items[i].Key, "model_mapping") {
			mappingItem = &response.Data.Items[i]
		}
	}
	require.NotNil(t, keyItem)
	require.True(t, keyItem.Masked)
	require.Nil(t, keyItem.Value)
	require.Len(t, keyItem.Chain, 4)
	require.Equal(t, adminconfig.SourceGlobal, keyItem.Chain[0].Source)
	require.Equal(t, adminconfig.SourceGroup, keyItem.Chain[1].Source)
	require.Equal(t, adminconfig.SourceChannel, keyItem.Chain[2].Source)
	require.True(t, keyItem.Chain[2].Present)
	require.Nil(t, keyItem.Chain[2].Value)
	require.Equal(t, adminconfig.SourceRequest, keyItem.Chain[3].Source)
	require.NotNil(t, mappingItem)
	require.True(t, mappingItem.Masked)
	require.Nil(t, mappingItem.Value)
	require.True(t, mappingItem.Chain[2].Present)
	require.Nil(t, mappingItem.Chain[2].Value)
}

func TestGetChannelEffectiveConfigRejectsInvalidOrMissingChannel(t *testing.T) {
	setupChannelUpdateControllerTestDB(t)
	router := gin.New()
	router.GET("/api/channel/:id/effective_config", GetChannelEffectiveConfig)

	for _, testCase := range []struct {
		path       string
		statusCode int
	}{
		{path: "/api/channel/not-a-number/effective_config", statusCode: http.StatusBadRequest},
		{path: "/api/channel/999/effective_config", statusCode: http.StatusNotFound},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, testCase.path, nil))
		require.Equal(t, testCase.statusCode, recorder.Code)
		require.NotContains(t, recorder.Body.String(), "Authorization")
	}
}

func TestGetChannelEffectiveConfigRejectsUnsupportedClientEndpoint(t *testing.T) {
	db := setupChannelUpdateControllerTestDB(t)
	channel := model.Channel{
		Type: 1, Key: "sk-test", Status: common.ChannelStatusEnabled,
		Name: "endpoint-validation", Models: "glm-5.2", Group: "default",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)

	router := gin.New()
	router.GET("/api/channel/:id/effective_config", GetChannelEffectiveConfig)
	recorder := httptest.NewRecorder()
	path := "/api/channel/" + strconv.Itoa(channel.Id) + "/effective_config?model=glm-5.2&client_endpoint=unknown"
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), channel.Key)
}
