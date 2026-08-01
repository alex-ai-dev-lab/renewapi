package model

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

// 兑换失败的具体原因。
// 原实现把所有失败统一改写成 ErrRedeemFailed，真实原因只进了 SysError 日志，
// 用户无法区分“没这个码”、“已被使用”、“已过期”。
var (
	ErrRedemptionInvalid = errors.New("无效的兑换码")
	ErrRedemptionUsed    = errors.New("该兑换码已被使用")
	ErrRedemptionExpired = errors.New("该兑换码已过期")
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

// GetAllRedemptions 这里不需要显式事务：两个只读查询不要求原子性，
// 原实现的 DB.Begin() 会在分页期间白白占着一个事务，
// 且 defer 里只 recover() 不重抛，会把 panic 吞掉，让上层拿到零值当正常结果。
func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	if err = DB.Model(&Redemption{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = DB.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error; err != nil {
		return nil, 0, err
	}
	return redemptions, total, nil
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// Keep wildcard characters in the search syntax under control. The shared
	// helper uses ! as the escape character for cross-dialect consistency.
	escapedKeyword := escapeLikeKeyword(keyword)
	buildQuery := func() *gorm.DB {
		if id, convErr := strconv.Atoi(keyword); convErr == nil {
			return DB.Model(&Redemption{}).Where("id = ? OR name LIKE ? ESCAPE '!'", id, escapedKeyword+"%")
		}
		return DB.Model(&Redemption{}).Where("name LIKE ? ESCAPE '!'", escapedKeyword+"%")
	}

	if err = buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = buildQuery().Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error; err != nil {
		return nil, 0, err
	}
	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	var redemption Redemption
	if err := DB.First(&redemption, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &redemption, nil
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return 0, errors.New("无效的 user id")
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}
	// 去掉 common.RandomSleep(): 那是引入 lockForUpdate 之前用来缓解并发兑换的遗留手法。
	// 现在行锁 + `WHERE id = ? AND status = enabled` 的条件更新已经保证了幂等，
	// 再随机 sleep 只是无条件抬高每一次兑换的延迟。
	businessErr := error(nil)
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where(keyCol+" = ?", key).First(redemption).Error; err != nil {
			businessErr = ErrRedemptionInvalid
			return businessErr
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			businessErr = ErrRedemptionUsed
			return businessErr
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			businessErr = ErrRedemptionExpired
			return businessErr
		}
		result := tx.Model(&Redemption{}).
			Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"redeemed_time": common.GetTimestamp(),
				"status":        common.RedemptionCodeStatusUsed,
				"used_user_id":  userId,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			businessErr = ErrRedemptionUsed
			return businessErr
		}
		return tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
	})
	if err != nil {
		if businessErr != nil {
			// 业务失败不属于系统异常，不必写 SysError，也应该把具体原因告知用户。
			return 0, businessErr
		}
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	// 事务内直接用 gorm.Expr 改了 users.quota，绕过了所有缓存同步入口。
	// 不失效的后果: Redis 里 user:%d 的 Quota 仍是旧值，直到 TTL 到期。
	// 用户兑换后看不到新余额，且预扣费等读缓存的链路会按旧余额判断。
	// 失效失败不影响兑换结果本身，因此只记日志。
	if cacheErr := invalidateUserCache(userId); cacheErr != nil {
		common.SysError(fmt.Sprintf("failed to invalidate user cache after redemption, user %d: %s", userId, cacheErr.Error()))
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return redemption.Quota, nil
}

func (redemption *Redemption) Insert() error {
	if redemption.Quota < 0 {
		return errors.New("quota cannot be negative")
	}
	return DB.Create(redemption).Error
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	if redemption.Quota < 0 {
		return errors.New("quota cannot be negative")
	}
	return DB.Model(redemption).Select("name", "status", "quota", "redeemed_time", "expired_time").Updates(redemption).Error
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

// DeleteInvalidRedemptions 软删除所有已使用/已禁用的兑换码，以及已过期的启用码。
// 注意: 条件里的 status IN (Used, Disabled) 不带任何时间限制，所以这个操作会一次性
// 清掉全部兑换历史。因为 Redemption 带 gorm.DeletedAt，这是软删除，数据仍可用
// Unscoped() 找回，但管理面板上看不到。若需要保留近期对账记录，应该给 Used /
// Disabled 加上 redeemed_time 早于 N 天的限定。
func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
