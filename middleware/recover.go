package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// WritePanicResponse 保留既有错误类型，但不将内部异常内容返回给客户端。
func WritePanicResponse(c *gin.Context) {
	if c.Writer.Written() {
		// 已提交的流不能再追加另一种格式的 JSON 错误。
		c.Abort()
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"message": "Internal server error. Please try again later.",
			"type":    "new_api_panic",
		},
	})
}

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				common.SysLog(fmt.Sprintf("panic detected: %v", err))
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				WritePanicResponse(c)
			}
		}()
		c.Next()
	}
}
