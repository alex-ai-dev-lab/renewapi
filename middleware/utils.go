package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	userId := c.GetInt("id")
	if statusCode >= http.StatusInternalServerError {
		public := types.PublicRelayError(types.NewErrorWithStatusCode(fmt.Errorf("%s", message), types.ErrorCode(codeStr), statusCode))
		if public.GetErrorCode() == types.ErrorCodeModelCapacity {
			switch {
			case strings.HasPrefix(c.Request.URL.Path, "/v1/messages"):
				c.JSON(public.StatusCode, gin.H{"type": "error", "error": public.ToClaudeError()})
			case strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/"):
				c.JSON(public.StatusCode, gin.H{"error": gin.H{"code": public.StatusCode, "status": "UNAVAILABLE", "message": public.Error()}})
			default:
				c.JSON(public.StatusCode, gin.H{"error": public.ToOpenAIError()})
			}
			c.Abort()
			logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, common.RedactErrorCredentials(message)))
			return
		}
	}
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
