package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetOverviewStats 获取总览统计
func GetOverviewStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)

	stats, err := model.GetOverviewStats(startTime)
	if err != nil {
		respondStatsError(c, "overview", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetChannelStats 获取渠道统计
func GetChannelStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)

	stats, err := model.GetChannelStats(startTime)
	if err != nil {
		respondStatsError(c, "channel", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetModelStats 获取模型统计
func GetModelStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)

	stats, err := model.GetModelStats(startTime)
	if err != nil {
		respondStatsError(c, "model", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetUserStats 获取用户统计（仅管理员）
func GetUserStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)

	stats, err := model.GetUserStats(startTime)
	if err != nil {
		respondStatsError(c, "user", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetChannelUserStats 获取某渠道下的用户消费与性能统计（仅管理员）
func GetChannelUserStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)
	channelID, err := strconv.Atoi(c.DefaultQuery("channel_id", "0"))
	if err != nil || channelID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid channel_id",
		})
		return
	}

	stats, err := model.GetChannelUserStats(startTime, channelID)
	if err != nil {
		respondStatsError(c, "channel-user", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetChannelTrendStats 获取某渠道的时序趋势（仅管理员）
func GetChannelTrendStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)
	channelID, err := strconv.Atoi(c.DefaultQuery("channel_id", "0"))
	if err != nil || channelID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid channel_id",
		})
		return
	}

	stats, err := model.GetChannelTrendStats(startTime, channelID)
	if err != nil {
		respondStatsError(c, "channel-trend", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetModelTrendStats 获取某模型的时序趋势（仅管理员）
func GetModelTrendStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)
	modelName := strings.TrimSpace(c.DefaultQuery("model_name", ""))
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid model_name",
		})
		return
	}

	stats, err := model.GetModelTrendStats(startTime, modelName)
	if err != nil {
		respondStatsError(c, "model-trend", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetUserTrendStats 获取某用户的时序趋势（仅管理员）
func GetUserTrendStats(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "7d")
	startTime := calculateStartTime(timeRange)
	userID, err := strconv.Atoi(c.DefaultQuery("user_id", "0"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user_id",
		})
		return
	}

	stats, err := model.GetUserTrendStats(startTime, userID)
	if err != nil {
		respondStatsError(c, "user-trend", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func respondStatsError(c *gin.Context, operation string, err error) {
	common.SysError(fmt.Sprintf("stats query failed: operation=%s request_id=%s error=%v", operation, c.GetString(common.RequestIdKey), model.SanitizeDBError(err)))
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"message": "Failed to query statistics",
	})
}

// calculateStartTime 根据时间范围计算起始时间
func calculateStartTime(timeRange string) time.Time {
	now := time.Now()
	switch timeRange {
	case "1d":
		return now.AddDate(0, 0, -1)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	case "1y":
		return now.AddDate(-1, 0, 0)
	case "all":
		return time.Time{} // 零值表示全部时间
	default:
		return now.AddDate(0, 0, -7)
	}
}
