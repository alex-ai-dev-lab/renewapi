package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		if len(info.PriceData.OtherRatios) > 0 {
			var contents []string
			for key, ra := range info.PriceData.OtherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
			if len(contents) > 0 {
				logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
			}
		}
	}
	other := make(map[string]interface{})
	other["is_task"] = true
	other["request_path"] = c.Request.URL.Path
	other["model_price"] = info.PriceData.ModelPrice
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	attachQuotaSaturation(c, info, other)
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	if !BillingOwnsAccounting(info) {
		model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
		model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
	}
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		return model.DecreaseUserQuota(task.UserId, delta, false)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) error {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return nil
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return fmt.Errorf("token key is unavailable for task %s", task.TaskID)
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
	return err
}

func recordTaskBillingAdjustment(task *model.Task, logType int, quota int, reason string, preConsumed int, actual int, clamps ...*common.QuotaClamp) {
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	other["pre_consumed_quota"] = preConsumed
	other["actual_quota"] = actual
	if task.BillingLedgerID > 0 {
		other["billing_ledger_id"] = task.BillingLedgerID
		other["billing_state"] = task.BillingState
	}
	for _, clamp := range clamps {
		attachQuotaSaturationToOther(other, clamp)
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId: task.UserId, LogType: logType, Content: reason, ChannelId: task.ChannelId,
		ModelName: taskModelName(task), Quota: quota, TokenId: task.PrivateData.TokenId,
		Group: task.Group, Other: other,
	})
}

// FinalizeTaskBillingTransition makes the terminal task CAS and ledger mutation
// one transaction. It returns won=false when another poller already transitioned
// the task from fromStatus.
func FinalizeTaskBillingTransition(ctx context.Context, task *model.Task, fromStatus model.TaskStatus, actualQuota int, reason string, clamps ...*common.QuotaClamp) (bool, error) {
	if task == nil {
		return false, errors.New("task is nil")
	}
	preConsumed := task.Quota
	desired := model.BillingLedgerDesiredSettle
	target := actualQuota
	if task.Status == model.TaskStatusFailure {
		desired = model.BillingLedgerDesiredRefund
		target = 0
	} else if task.Status != model.TaskStatusSuccess {
		return false, fmt.Errorf("task %s is not terminal", task.TaskID)
	}
	if target < 0 {
		return false, errors.New("task billing target cannot be negative")
	}

	won, ledger, err := model.UpdateTaskWithBilling(task, fromStatus, desired, int64(target), reason)
	if err != nil {
		if task.BillingLedgerID > 0 {
			_ = model.MarkBillingLedgerForReconcile(task.BillingLedgerID, desired, int64(target), err)
		}
		return false, err
	}
	if !won {
		return false, nil
	}
	if ledger != nil {
		task.BillingLedgerID = ledger.ID
		task.BillingState = ledger.State
		task.BillingVersion = ledger.Version
		if desired == model.BillingLedgerDesiredSettle {
			task.Quota = int(ledger.AppliedQuota)
		}
		if ledger.Mode == BillingLedgerModeEnforce {
			model.InvalidateBillingBalanceCaches(task.UserId, task.PrivateData.TokenId)
			delta := target - preConsumed
			if desired == model.BillingLedgerDesiredRefund {
				recordTaskBillingAdjustment(task, model.LogTypeRefund, preConsumed, reason, preConsumed, 0, clamps...)
			} else if delta != 0 {
				logType, logQuota := model.LogTypeConsume, delta
				if delta < 0 {
					logType, logQuota = model.LogTypeRefund, -delta
				}
				recordTaskBillingAdjustment(task, logType, logQuota, reason, preConsumed, target, clamps...)
			}
			return true, nil
		}
	}

	delta := target - preConsumed
	if delta != 0 {
		if err := taskAdjustFunding(task, delta); err != nil {
			return true, err
		}
		if err := taskAdjustTokenQuota(ctx, task, delta); err != nil {
			return true, err
		}
	}
	if desired == model.BillingLedgerDesiredRefund {
		recordTaskBillingAdjustment(task, model.LogTypeRefund, preConsumed, reason, preConsumed, 0, clamps...)
	} else if delta != 0 {
		logType, logQuota := model.LogTypeConsume, delta
		if delta < 0 {
			logType, logQuota = model.LogTypeRefund, -delta
		}
		recordTaskBillingAdjustment(task, logType, logQuota, reason, preConsumed, target, clamps...)
	}
	return true, nil
}

func FinalizeMidjourneyBillingTransition(ctx context.Context, task *model.Midjourney, fromStatus string, reason string) (bool, error) {
	if task == nil {
		return false, errors.New("midjourney task is nil")
	}
	preConsumed := task.Quota
	desired := model.BillingLedgerDesiredSettle
	target := int64(preConsumed)
	if task.Status == string(model.TaskStatusFailure) {
		desired = model.BillingLedgerDesiredRefund
		target = 0
	} else if task.Status != string(model.TaskStatusSuccess) {
		return false, fmt.Errorf("midjourney task %s is not terminal", task.MjId)
	}

	won, ledger, err := model.UpdateMidjourneyWithBilling(task, fromStatus, desired, target, reason)
	if err != nil {
		if task.BillingLedgerID > 0 {
			_ = model.MarkBillingLedgerForReconcile(task.BillingLedgerID, desired, target, err)
		}
		return false, err
	}
	if !won {
		return false, nil
	}
	if ledger != nil {
		task.BillingLedgerID = ledger.ID
		task.BillingState = ledger.State
		task.BillingVersion = ledger.Version
		if ledger.Mode == BillingLedgerModeEnforce {
			model.InvalidateBillingBalanceCaches(task.UserId, ledger.TokenID)
			if desired == model.BillingLedgerDesiredRefund && preConsumed > 0 {
				recordMidjourneyBillingAdjustment(task, ledger.TokenID, model.LogTypeRefund, preConsumed, reason)
			}
			return true, nil
		}
	}
	if desired == model.BillingLedgerDesiredRefund && preConsumed > 0 {
		if task.BillingSource == BillingSourceSubscription {
			if err := model.PostConsumeUserSubscriptionDelta(task.SubscriptionID, int64(-preConsumed)); err != nil {
				return true, err
			}
		} else if err := model.IncreaseUserQuota(task.UserId, preConsumed, false); err != nil {
			return true, err
		}
		if task.TokenID > 0 {
			key := resolveTokenKey(ctx, task.TokenID, task.MjId)
			if key == "" {
				return true, fmt.Errorf("token key is unavailable for midjourney task %s", task.MjId)
			}
			if err := model.IncreaseTokenQuota(task.TokenID, key, preConsumed); err != nil {
				return true, err
			}
		}
		recordMidjourneyBillingAdjustment(task, task.TokenID, model.LogTypeRefund, preConsumed, reason)
	}
	return true, nil
}

func recordMidjourneyBillingAdjustment(task *model.Midjourney, tokenID int, logType int, quota int, reason string) {
	other := map[string]interface{}{
		"task_id": task.MjId,
		"reason":  reason,
	}
	if task.BillingLedgerID > 0 {
		other["billing_ledger_id"] = task.BillingLedgerID
		other["billing_state"] = task.BillingState
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId: task.UserId, LogType: logType, Content: reason, ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action), Quota: quota, TokenId: tokenID,
		Other: other,
	})
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		if bc.ModelRatio > 0 {
			other["model_ratio"] = bc.ModelRatio
		}
		other["group_ratio"] = bc.GroupRatio
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	return other
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，将预扣的 quota 退还给用户（支持钱包和订阅），并退还令牌额度。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) {
	quota := task.Quota
	if quota == 0 {
		return
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 2. 退还令牌额度
	if err := taskAdjustTokenQuota(ctx, task, -quota); err != nil {
		return
	}

	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string, clamps ...*common.QuotaClamp) {
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// 调整资金来源
	if err := taskAdjustFunding(task, quotaDelta); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 调整令牌额度
	if err := taskAdjustTokenQuota(ctx, task, quotaDelta); err != nil {
		return
	}

	task.Quota = actualQuota
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算回写 quota 失败 task %s: %s", task.TaskID, err.Error()))
	}

	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
		model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
		model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	for _, clamp := range clamps {
		attachQuotaSaturationToOther(other, clamp)
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) {
	actualQuota, reason, clamp, ok := CalculateTaskQuotaByTokens(task, totalTokens)
	if !ok {
		return
	}
	RecalculateTaskQuota(ctx, task, actualQuota, reason, clamp)
	if clamp != nil {
		logger.LogWarn(ctx, fmt.Sprintf("task quota saturation: task=%s kind=%s original=%g clamped=%d", task.TaskID, clamp.Kind, clamp.Original, clamp.Clamped))
	}
}

func CalculateTaskQuotaByTokens(task *model.Task, totalTokens int) (int, string, *common.QuotaClamp, bool) {
	if totalTokens <= 0 {
		return 0, "", nil, false
	}

	modelName := taskModelName(task)

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return 0, "", nil, false
	}

	// 获取用户和组的倍率信息
	group := task.Group
	if group == "" {
		user, err := model.GetUserById(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return 0, "", nil, false
	}

	groupRatio := ratio_setting.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)

	var finalGroupRatio float64
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	} else {
		finalGroupRatio = groupRatio
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil {
		priceData := types.PriceData{}
		priceData.ReplaceOtherRatios(bc.OtherRatios)
		otherMultiplier = priceData.OtherRatioMultiplier()
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier
	actualQuota, clamp := common.QuotaFromFloatChecked(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f", totalTokens, modelRatio, finalGroupRatio, otherMultiplier)
	return actualQuota, reason, clamp, true
}
