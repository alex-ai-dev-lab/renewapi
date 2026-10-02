package controller

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

func quotaFlow(c *gin.Context, self bool) {
	now := time.Now().Unix()
	start, errStart := strconv.ParseInt(c.DefaultQuery("start_timestamp", strconv.FormatInt(now-7*86400, 10)), 10, 64)
	end, errEnd := strconv.ParseInt(c.DefaultQuery("end_timestamp", strconv.FormatInt(now, 10)), 10, 64)
	if errStart != nil || errEnd != nil || start < 0 || end < start {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid flow time range"})
		return
	}
	userID := 0
	if self {
		userID = c.GetInt("id")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	rows, err := model.GetQuotaFlow(ctx, userID, start, end, c.Query("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}
func GetAllQuotaFlow(c *gin.Context)  { quotaFlow(c, false) }
func GetSelfQuotaFlow(c *gin.Context) { quotaFlow(c, true) }
func GetAccountTokenUsage(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	summary, err := model.GetAccountTokenUsage(ctx, c.GetInt("id"), time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}
