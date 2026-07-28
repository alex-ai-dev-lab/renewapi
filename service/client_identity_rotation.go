package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

var rotationWorkerStarted bool

// StartClientIdentityRotationWorker 启动客户端标识符轮换后台任务
func StartClientIdentityRotationWorker() {
	if rotationWorkerStarted {
		return
	}
	rotationWorkerStarted = true

	go RunClientIdentityRotationWorker(context.Background())
}

func RunClientIdentityRotationWorker(ctx context.Context) {
	checkAndRotateClientIdentity()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	common.SysLog("client identity rotation worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkAndRotateClientIdentity()
		}
	}
}

// checkAndRotateClientIdentity 检查并轮换客户端标识符
func checkAndRotateClientIdentity() {
	setting := model.GetClientIdentitySetting()
	if !setting.Enabled {
		return
	}

	now := time.Now().Unix()
	changed := false

	// 检查 Codex 轮换
	if setting.Codex.Enabled && setting.Codex.RotateEnabled {
		if setting.Codex.NextRotateAt == 0 {
			// 首次初始化轮换时间
			setting.Codex.NextRotateAt = model.CalculateNextRotateTime(
				setting.Codex.RotateIntervalUnit,
				setting.Codex.RotateIntervalValue,
			)
			changed = true
		} else if now >= setting.Codex.NextRotateAt {
			// 执行轮换
			oldID := setting.Codex.InstallationID
			newID := uuid.NewString()
			setting.Codex.InstallationID = newID
			setting.Codex.NextRotateAt = model.CalculateNextRotateTime(
				setting.Codex.RotateIntervalUnit,
				setting.Codex.RotateIntervalValue,
			)
			changed = true
			logRotation("codex", oldID, newID, setting.Codex.NextRotateAt)
		}
	}

	// 检查 Claude 轮换
	if setting.Claude.Enabled && setting.Claude.RotateEnabled {
		if setting.Claude.NextRotateAt == 0 {
			setting.Claude.NextRotateAt = model.CalculateNextRotateTime(
				setting.Claude.RotateIntervalUnit,
				setting.Claude.RotateIntervalValue,
			)
			changed = true
		} else if now >= setting.Claude.NextRotateAt {
			oldID := setting.Claude.DeviceID
			newID := uuid.NewString()
			setting.Claude.DeviceID = newID
			if setting.Claude.SessionIDMode == model.SessionIDModeForceGlobal {
				setting.Claude.FixedSessionID = uuid.NewString()
			}
			setting.Claude.NextRotateAt = model.CalculateNextRotateTime(
				setting.Claude.RotateIntervalUnit,
				setting.Claude.RotateIntervalValue,
			)
			changed = true
			logRotation("claude", oldID, newID, setting.Claude.NextRotateAt)
		}
	}

	// 检查通用厂商轮换
	for i := range setting.Generic {
		if setting.Generic[i].Enabled && setting.Generic[i].RotateEnabled {
			if setting.Generic[i].NextRotateAt == 0 {
				setting.Generic[i].NextRotateAt = model.CalculateNextRotateTime(
					setting.Generic[i].RotateIntervalUnit,
					setting.Generic[i].RotateIntervalValue,
				)
				changed = true
			} else if now >= setting.Generic[i].NextRotateAt {
				oldID := setting.Generic[i].FieldValue
				newID := uuid.NewString()
				setting.Generic[i].FieldValue = newID
				setting.Generic[i].NextRotateAt = model.CalculateNextRotateTime(
					setting.Generic[i].RotateIntervalUnit,
					setting.Generic[i].RotateIntervalValue,
				)
				changed = true
				logRotation("generic:"+setting.Generic[i].Name, oldID, newID, setting.Generic[i].NextRotateAt)
			}
		}
	}

	// 保存配置
	if changed {
		if err := model.UpdateClientIdentitySetting(setting); err != nil {
			common.SysLog("client identity rotation: failed to save setting: " + err.Error())
		}
	}
}

// logRotation 记录轮换操作
func logRotation(provider, oldID, newID string, nextRotateAt int64) {
	oldFP := idFingerprint(oldID)
	newFP := idFingerprint(newID)
	common.SysLog(fmt.Sprintf(
		"client identity rotated: provider=%s old_fp=%s new_fp=%s next_rotate_at=%d",
		provider, oldFP, newFP, nextRotateAt,
	))
}

// idFingerprint 生成 ID 指纹
func idFingerprint(id string) string {
	if id == "" {
		return "empty"
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])[:8]
}

// CalculateNextRotateTime 暴露给 model 包使用
func CalculateNextRotateTime(unit string, value int) int64 {
	return model.CalculateNextRotateTime(unit, value)
}
