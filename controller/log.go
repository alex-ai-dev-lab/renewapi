package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) { getLogs(c, false) }

func GetUserLogs(c *gin.Context) { getLogs(c, true) }

func getLogs(c *gin.Context, self bool) {
	logTypes, err := parseLogTypeFilter(c.Query("types"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := model.LogQueryFilter{ViewerID: c.GetInt("id"), LogType: logType, LogTypes: logTypes,
		StartTimestamp: startTimestamp, EndTimestamp: endTimestamp, ModelName: c.Query("model_name"),
		Username: c.Query("username"), TokenName: c.Query("token_name"), Channel: channel,
		Group: c.Query("group"), RequestID: c.Query("request_id"), UpstreamRequestID: c.Query("upstream_request_id")}
	if self {
		if filter.ViewerID <= 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		filter.UserID, filter.Username, filter.Channel = filter.ViewerID, "", 0
	}
	pagination := c.Query("pagination")
	if pagination != "" && pagination != "cursor" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported log pagination"})
		return
	}
	if pagination == "cursor" || c.Query("cursor") != "" {
		data, err := model.GetLogCursorPage(c.Request.Context(), filter, c.Query("cursor"), pageInfo.GetPageSize())
		if err != nil {
			logPageError(c, err)
			return
		}
		common.ApiSuccess(c, data)
		return
	}
	var logs []*model.Log
	var total int64
	if self {
		logs, total, err = model.GetUserLogsWithContext(c.Request.Context(), filter.UserID, logType, startTimestamp, endTimestamp, filter.ModelName, filter.TokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), filter.Group, filter.RequestID, filter.UpstreamRequestID, logTypes...)
	} else {
		logs, total, err = model.GetAllLogsWithContext(c.Request.Context(), logType, startTimestamp, endTimestamp, filter.ModelName, filter.Username, filter.TokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), filter.Channel, filter.Group, filter.RequestID, filter.UpstreamRequestID, logTypes...)
	}
	if err != nil {
		logPageError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.TotalCapped = total > model.LogSearchCountLimit
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func logPageError(c *gin.Context, err error) {
	if errors.Is(err, model.ErrInvalidLogCursor) || errors.Is(err, model.ErrLogOffsetLimit) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiError(c, err)
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	stat, err := model.SumUsedQuotaWithContext(c.Request.Context(), logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	quotaNum, err := model.SumUsedQuotaWithContext(c.Request.Context(), logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}

// A plural filter narrows the existing single-type filter; omitting it keeps
// the legacy API unchanged. Values are parsed before reaching the SQL query.
func parseLogTypeFilter(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 8 {
		return nil, fmt.Errorf("too many log types")
	}
	result := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value > 7 {
			return nil, fmt.Errorf("invalid log type filter")
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}
