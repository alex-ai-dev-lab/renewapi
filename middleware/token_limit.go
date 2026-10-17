package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TokenRequestLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
		rpmLimit := common.GetContextKeyInt(c, constant.ContextKeyTokenRPMLimit)
		if rpmLimit > 0 && !service.CheckAndRecordTokenRPM(tokenID, rpmLimit) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "token RPM limit exceeded", types.ErrorCodeRateLimitExceeded)
			return
		}

		concurrencyLimit := common.GetContextKeyInt(c, constant.ContextKeyTokenConcurrencyLimit)
		if concurrencyLimit > 0 {
			slot, allowed := service.AcquireTokenConcurrency(c.Request.Context(), tokenID, concurrencyLimit)
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, "token concurrency limit exceeded", types.ErrorCodeRateLimitExceeded)
				return
			}
			defer slot.Release()
			c.Request = c.Request.WithContext(slot.Context)
		}
		c.Next()
	}
}
