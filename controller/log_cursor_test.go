package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLogCursorHTTPContractAndSelfIsolation(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	previous := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previous })
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	for i := 1; i <= 6; i++ {
		require.NoError(t, db.Create(&model.Log{Id: i, UserId: 71 + i%2, Type: model.LogTypeConsume, CreatedAt: 100}).Error)
	}
	request := func(query string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("GET", "/api/log/self?"+query, nil)
		ctx.Set("id", 71)
		GetUserLogs(ctx)
		return recorder
	}
	first := request("pagination=cursor&page_size=2")
	require.Equal(t, 200, first.Code)
	var body struct {
		Success bool                `json:"success"`
		Data    model.LogCursorPage `json:"data"`
	}
	require.NoError(t, common.Unmarshal(first.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Items, 2)
	require.NotEmpty(t, body.Data.NextCursor)
	for _, row := range body.Data.Items {
		require.Equal(t, 71, row.UserId)
	}
	second := request("pagination=cursor&page_size=2&cursor=" + body.Data.NextCursor)
	require.Equal(t, 200, second.Code)
	body.Data = model.LogCursorPage{}
	require.NoError(t, common.Unmarshal(second.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 1)
	require.Empty(t, body.Data.NextCursor)
	require.NotEmpty(t, body.Data.PreviousCursor)
	require.Equal(t, 400, request("pagination=cursor&cursor=invalid").Code)
	require.Equal(t, 400, request("p=5000&page_size=100").Code)
	require.Equal(t, 400, request("pagination=unknown").Code)
}
