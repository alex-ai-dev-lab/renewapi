package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func BillingOwnsAccounting(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil || relayInfo.Billing == nil {
		return false
	}
	session, ok := relayInfo.Billing.(*BillingSession)
	return ok && session.ownsLedgerAccounting()
}

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
)

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo != nil && relayInfo.QuotaClamp != nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("pre-consume quota rejected after %s saturation", relayInfo.QuotaClamp.Kind), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

func PrepareAsyncTaskBilling(relayInfo *relaycommon.RelayInfo, platform constant.TaskPlatform) error {
	if relayInfo == nil || relayInfo.Billing == nil {
		return nil
	}
	session, ok := relayInfo.Billing.(*BillingSession)
	if !ok || !session.ownsLedgerAccounting() {
		return nil
	}
	task := model.InitTask(platform, relayInfo)
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
	}
	task.Quota = relayInfo.PriceData.Quota
	task.Action = relayInfo.Action
	return session.prepareTask(task)
}

// SettleBillingAndInsertTask atomically settles enforced ledger billing and
// persists the local async task. Shadow/off modes retain the compatible path.
func SettleBillingAndInsertTask(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int, task *model.Task) error {
	if relayInfo == nil || task == nil {
		return fmt.Errorf("billing relay info or task is nil")
	}
	if session, ok := relayInfo.Billing.(*BillingSession); ok && session.ownsLedgerAccounting() {
		return session.settleWithTask(task, actualQuota)
	}
	if err := SettleBilling(ctx, relayInfo, actualQuota); err != nil {
		return err
	}
	if session, ok := relayInfo.Billing.(*BillingSession); ok && session.ledgerID != 0 {
		task.BillingLedgerID = session.ledgerID
		if ledger, err := model.GetBillingLedger(session.ledgerID); err == nil {
			task.BillingState = ledger.State
			task.BillingVersion = ledger.Version
		}
	}
	return task.Insert()
}

func SettleBillingAndInsertMidjourney(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int, task *model.Midjourney) error {
	if relayInfo == nil || task == nil {
		return fmt.Errorf("billing relay info or midjourney task is nil")
	}
	if session, ok := relayInfo.Billing.(*BillingSession); ok && session.ownsLedgerAccounting() {
		return session.settleWithMidjourney(task, actualQuota)
	}
	if err := SettleBilling(ctx, relayInfo, actualQuota); err != nil {
		return err
	}
	if session, ok := relayInfo.Billing.(*BillingSession); ok && session.ledgerID != 0 {
		task.BillingLedgerID = session.ledgerID
		if ledger, err := model.GetBillingLedger(session.ledgerID); err == nil {
			task.BillingState = ledger.State
			task.BillingVersion = ledger.Version
		}
	}
	return task.Insert()
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算。如果 RelayInfo 上有 BillingSession 则通过 session 结算，
// 否则回退到旧的 PostConsumeQuota 路径（兼容按次计费等场景）。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}

		// 发送额度通知（订阅计费使用订阅剩余额度）
		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}
