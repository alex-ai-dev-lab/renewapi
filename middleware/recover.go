package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				PanicRecoveryHandler(c, err)
			}
		}()
		c.Next()
	}
}

// PanicRecoveryHandler is shared by the global and relay recovery middleware so
// every panic response follows the same redacted contract.
func PanicRecoveryHandler(c *gin.Context, recovered any) {
	requestID := c.GetString(common.RequestIdKey)
	common.SysLog(fmt.Sprintf("panic detected request_id=%s: %v", requestID, recovered))
	common.SysLog(fmt.Sprintf("stacktrace from panic request_id=%s: %s", requestID, string(debug.Stack())))

	message := "Internal server error"
	if requestID != "" {
		message = fmt.Sprintf("Internal server error (request_id=%s)", requestID)
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "new_api_panic",
		},
	})
	c.Abort()
}
