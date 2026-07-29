package service

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// preConsumeRefundMaxAttempts 预扣费还原的最大尝试次数。
// 还原失败意味着用户被多扣了额度，属于直接资损，不能“打一行日志就算了”。
const preConsumeRefundMaxAttempts = 3

func ReturnPreConsumedQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil || relayInfo.FinalPreConsumedQuota == 0 {
		return
	}
	quota := relayInfo.FinalPreConsumedQuota
	userId := relayInfo.UserId
	tokenId := relayInfo.TokenId
	logger.LogInfo(c, fmt.Sprintf("用户 %d 请求失败, 返还预扣费额度 %s", userId, logger.FormatQuota(quota)))
	relayInfoCopy := *relayInfo
	gopool.Go(func() {
		var lastErr error
		for attempt := 1; attempt <= preConsumeRefundMaxAttempts; attempt++ {
			infoCopy := relayInfoCopy
			lastErr = PostConsumeQuota(&infoCopy, -quota, 0, false)
			if lastErr == nil {
				return
			}
			common.SysLog(fmt.Sprintf(
				"error return pre-consumed quota (attempt %d/%d), user %d, token %d, quota %d: %s",
				attempt, preConsumeRefundMaxAttempts, userId, tokenId, quota, lastErr.Error(),
			))
			if attempt < preConsumeRefundMaxAttempts {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
		}
		// 最终失败：用一个固定可检索的前缀输出，方便告警与人工对账。
		// TODO: 应落到 billing outbox / 异步重试队列，而不是只靠日志。
		common.SysLog(fmt.Sprintf(
			"[QUOTA_REFUND_LOST] user %d, token %d, quota %d, last error: %v",
			userId, tokenId, quota, lastErr,
		))
	})
}

// PreConsumeQuota checks if the user has enough quota to pre-consume.
// It returns the pre-consumed quota if successful, or an error if not.
func PreConsumeQuota(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo != nil && relayInfo.QuotaClamp != nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("pre-consume quota rejected after %s saturation", relayInfo.QuotaClamp.Kind), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if userQuota <= 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("用户额度不足, 剩余额度: %s", logger.FormatQuota(userQuota)), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	// 注意: 这里只是快速失败的前置检查, 真正的原子性由 model.DecreaseUserQuota
	// 里的 `WHERE quota >= ?` 保证; 并发场景下本检查可能通过而后续扣减失败,
	// 那种情况会在下面被映射成额度不足错误。
	if userQuota-preConsumedQuota < 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("预扣费额度失败, 用户剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}

	trustQuota := common.GetTrustQuota()

	relayInfo.UserQuota = userQuota
	// 只有在“扣掉本次预估消耗之后仍然高于信任阈值”时才免预扣。
	// 旧条件只判断 userQuota > trustQuota, 于是一个刚好略高于阈值的账户可以并发
	// 发起大量长请求且全部免预扣, 最终稳定透支为负额度。
	if userQuota > trustQuota && userQuota-preConsumedQuota > trustQuota {
		// 用户额度充足，判断令牌额度是否充足
		if !relayInfo.TokenUnlimited {
			// 非无限令牌，判断令牌额度是否充足
			tokenQuota := c.GetInt("token_quota")
			if tokenQuota > trustQuota && tokenQuota-preConsumedQuota > trustQuota {
				// 令牌额度充足，信任令牌
				preConsumedQuota = 0
				logger.LogInfo(c, fmt.Sprintf("用户 %d 剩余额度 %s 且令牌 %d 额度 %d 充足, 信任且不需要预扣费", relayInfo.UserId, logger.FormatQuota(userQuota), relayInfo.TokenId, tokenQuota))
			}
		} else {
			// in this case, we do not pre-consume quota
			// because the user has enough quota
			preConsumedQuota = 0
			logger.LogInfo(c, fmt.Sprintf("用户 %d 额度充足且为无限额度令牌, 信任且不需要预扣费", relayInfo.UserId))
		}
	}

	if preConsumedQuota > 0 {
		err := PreConsumeTokenQuota(relayInfo, preConsumedQuota)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		// FIXME: 此处令牌额度已扣、用户额度未扣, 两步不在同一事务内。
		// 下面失败时令牌额度不会被回滚(对用户不利), 需要后续统一到一次事务或补偿流程。
		err = model.DecreaseUserQuota(relayInfo.UserId, preConsumedQuota, false)
		if err != nil {
			if errors.Is(err, model.ErrInsufficientQuota) {
				// 并发下被别的请求先扣走了: 这是额度不足, 不是数据库故障。
				// 原实现统一返回 UpdateDataError, 既误导用户也会让上层按“服务端错误”重试。
				return types.NewErrorWithStatusCode(
					fmt.Errorf("预扣费额度失败, 用户额度不足, 需要预扣费额度: %s", logger.FormatQuota(preConsumedQuota)),
					types.ErrorCodeInsufficientUserQuota,
					http.StatusForbidden,
					types.ErrOptionWithSkipRetry(),
					types.ErrOptionWithNoRecordErrorLog(),
				)
			}
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		logger.LogInfo(c, fmt.Sprintf("用户 %d 预扣费 %s, 预扣费后剩余额度: %s", relayInfo.UserId, logger.FormatQuota(preConsumedQuota), logger.FormatQuota(userQuota-preConsumedQuota)))
	}
	relayInfo.FinalPreConsumedQuota = preConsumedQuota
	return nil
}
