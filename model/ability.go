package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// maxRoutingRank 作为“排序权重未知”的最差 rank。不能用零值，否则 map 缺失的
// 渠道会拿到 rank 0（最优）。
const maxRoutingRank = int(^uint(0) >> 1)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func GetChannel(group string, model string, retry int) (*Channel, error) {
	return GetChannelExcluding(group, model, retry, nil)
}

func GetChannelExcluding(group string, model string, retry int, excluded map[int]bool) (*Channel, error) {
	return GetChannelExcludingWithPolicy(group, model, retry, excluded, nil)
}

func GetChannelExcludingWithPolicy(group string, model string, retry int, excluded map[int]bool, policy ChannelRoutingPolicy) (*Channel, error) {
	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC, weight DESC").
		Find(&abilities).Error; err != nil {
		return nil, err
	}

	if len(excluded) > 0 {
		filtered := abilities[:0]
		for _, ability := range abilities {
			if excluded[ability.ChannelId] {
				continue
			}
			filtered = append(filtered, ability)
		}
		abilities = filtered
		// Mirror the in-memory cache path (GetRandomSatisfiedChannelExcludingWithPolicy):
		// once we are explicitly excluding channels (e.g. retrying after a failure),
		// restart priority selection from the highest remaining priority bucket
		// instead of stepping down to a lower-priority bucket by retry index.
		retry = 0
	}
	abilities = filterChannelModelStatusAbilities(abilities)
	abilities = filterRandomSelectableAbilities(abilities)
	// 注意: filterProviderRoutingAbilities 会按 provider order rank 重排，破坏上面 SQL 的
	// priority DESC 顺序。selectAbilitiesByRetryPriority 已改为自己按优先级降序，
	// 不再依赖传入顺序。
	abilities = filterProviderRoutingAbilities(abilities, policy)
	if len(abilities) == 0 {
		return nil, nil
	}
	abilities = selectAbilitiesByRetryPriority(abilities, retry)
	abilities = keepBestProviderRoutingAbilityOrderRank(abilities, policy)
	if len(abilities) == 0 {
		return nil, nil
	}

	channel := Channel{}
	weightSum := uint(0)
	for _, ability_ := range abilities {
		weightSum += ability_.Weight + 10
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, ability_ := range abilities {
		weight -= int(ability_.Weight) + 10
		if weight <= 0 {
			channel.Id = ability_.ChannelId
			break
		}
	}
	if channel.Id == 0 {
		channel.Id = abilities[0].ChannelId
	}
	err := DB.First(&channel, "id = ?", channel.Id).Error
	return &channel, err
}

// selectAbilitiesByRetryPriority 只保留“第 retry 高”的优先级桶。
// retry=0 取最高优先级，每重试一次降一档，超出档数后停在最低档。
func selectAbilitiesByRetryPriority(abilities []Ability, retry int) []Ability {
	if len(abilities) == 0 {
		return abilities
	}
	priorities := make([]int64, 0, 4)
	seen := make(map[int64]struct{}, 4)
	for _, ability := range abilities {
		priority := abilityPriority(ability)
		if _, ok := seen[priority]; ok {
			continue
		}
		seen[priority] = struct{}{}
		priorities = append(priorities, priority)
	}
	// 以前这里直接沿用传入切片的遇见顺序，隐含假设“已经按 priority DESC 排好”。
	// 但 filterProviderRoutingAbilities 会按 provider order rank 重排，一旦请求带了
	// provider order 策略，retry → 优先级的映射就变成了任意顺序。这里显式降序。
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	idx := retry
	if idx < 0 {
		idx = 0
	}
	if idx >= len(priorities) {
		idx = len(priorities) - 1
	}
	targetPriority := priorities[idx]
	filtered := abilities[:0]
	for _, ability := range abilities {
		if abilityPriority(ability) == targetPriority {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func filterChannelModelStatusAbilities(abilities []Ability) []Ability {
	if len(abilities) == 0 {
		return abilities
	}
	channelIDsByGroupModel := make(map[string][]int)
	for _, ability := range abilities {
		key := channelModelStatusKey(ability.Group, ability.Model)
		channelIDsByGroupModel[key] = append(channelIDsByGroupModel[key], ability.ChannelId)
	}
	disabled := make(map[string]struct{})
	for key, channelIDs := range channelIDsByGroupModel {
		parts := strings.SplitN(key, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		filtered := FilterChannelIDsByModelStatus(channelIDs, parts[0], parts[1])
		allowed := make(map[int]struct{}, len(filtered))
		for _, id := range filtered {
			allowed[id] = struct{}{}
		}
		for _, id := range channelIDs {
			if _, ok := allowed[id]; !ok {
				disabled[fmt.Sprintf("%s\x00%d", key, id)] = struct{}{}
			}
		}
	}
	filtered := abilities[:0]
	for _, ability := range abilities {
		key := fmt.Sprintf("%s\x00%d", channelModelStatusKey(ability.Group, ability.Model), ability.ChannelId)
		if _, ok := disabled[key]; ok {
			continue
		}
		filtered = append(filtered, ability)
	}
	return filtered
}

func filterRandomSelectableAbilities(abilities []Ability) []Ability {
	if len(abilities) == 0 {
		return abilities
	}
	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var selectableIDs []int
	if err := DB.Model(&Channel{}).
		Where("id IN ? AND type <> ?", channelIDs, constant.ChannelTypeMock).
		Pluck("id", &selectableIDs).Error; err != nil {
		common.SysError(fmt.Sprintf("failed to filter random selectable channels: %v", err))
		return abilities
	}
	selectable := make(map[int]struct{}, len(selectableIDs))
	for _, id := range selectableIDs {
		selectable[id] = struct{}{}
	}
	filtered := abilities[:0]
	for _, ability := range abilities {
		if _, ok := selectable[ability.ChannelId]; ok {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func filterProviderRoutingAbilities(abilities []Ability, policy ChannelRoutingPolicy) []Ability {
	if len(abilities) == 0 || policy == nil || policy.Empty() {
		return abilities
	}
	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		common.SysError(fmt.Sprintf("failed to filter provider routing channels: %v", err))
		return abilities
	}
	matches := make(map[int]struct{}, len(channels))
	order := make(map[int]int, len(channels))
	for i := range channels {
		ch := &channels[i]
		if policy.Matches(ch) {
			matches[ch.Id] = struct{}{}
			order[ch.Id] = policy.OrderRank(ch)
		}
	}
	filtered := abilities[:0]
	for _, ability := range abilities {
		if _, ok := matches[ability.ChannelId]; ok {
			filtered = append(filtered, ability)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return routingRank(order, filtered[i].ChannelId) < routingRank(order, filtered[j].ChannelId)
	})
	return filtered
}

func keepBestProviderRoutingAbilityOrderRank(abilities []Ability, policy ChannelRoutingPolicy) []Ability {
	if len(abilities) < 2 || policy == nil || policy.Empty() {
		return abilities
	}
	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		common.SysError(fmt.Sprintf("failed to apply provider routing order: %v", err))
		return abilities
	}
	order := make(map[int]int, len(channels))
	for i := range channels {
		ch := &channels[i]
		order[ch.Id] = policy.OrderRank(ch)
	}
	// 以前这里直接用 order[id]，map 里没有的渠道（例如两次查询之间渠道被删）
	// 会拿到零值 0，即“最优 rank”，反而把真正匹配策略的渠道挤掉。
	bestRank := routingRank(order, abilities[0].ChannelId)
	for _, ability := range abilities[1:] {
		if rank := routingRank(order, ability.ChannelId); rank < bestRank {
			bestRank = rank
		}
	}
	filtered := abilities[:0]
	for _, ability := range abilities {
		if routingRank(order, ability.ChannelId) == bestRank {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func routingRank(order map[int]int, channelID int) int {
	if rank, ok := order[channelID]; ok {
		return rank
	}
	return maxRoutingRank
}

func abilityPriority(ability Ability) int64 {
	if ability.Priority == nil {
		return 0
	}
	return *ability.Priority
}

// buildAbilities 根据渠道当前的模型与分组展开出 ability 行。
// AddAbilities 和 UpdateAbilities 以前各自抱了一份字字相同的展开逻辑，
// 一边改漏另一边很容易让路由表与渠道配置脉络不一致。
func (channel *Channel) buildAbilities() []Ability {
	models_ := channel.GetRoutingModels()
	groups_ := channel.GetGroups()
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			abilities = append(abilities, Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			})
		}
	}
	return abilities
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	abilities := channel.buildAbilities()
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteAbilities 删除该渠道的全部 ability。
// 注意: 与 AddAbilities(tx) 不对称——这里只走全局 DB，无法带入调用方的事务。
// 在事务中调用它会造成“删除已提交、插入可回滚”的不一致，需要事务语义时
// 请直接用 UpdateAbilities(tx)。
func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				// 以前这里只回滚不重抛，panic 被静默吃掉，调用方会以为更新成功。
				panic(r)
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	abilities := channel.buildAbilities()
	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

// UpdateAbilityByTag 批量修改同一标签下的 ability。
// 以前这里组一个 Ability 结构体丢给 Updates，而 GORM 的 struct 更新会忽略零值：
// priority=0 / weight=0 / newTag="" 都会被静默丢弃，管理员在面板上把权重或优先级
// 改成 0 看上去成功，实际数据库没变。现在用 map 显式指定要写的列。
func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	updates := make(map[string]interface{}, 3)
	if newTag != nil {
		updates["tag"] = *newTag
	}
	if priority != nil {
		updates["priority"] = *priority
	}
	if weight != nil {
		updates["weight"] = *weight
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(updates).Error
}

var fixLock = sync.Mutex{}

// FixAbility 重建整张路由表。
// 注意: 整个过程不在事务里（MySQL 的 TRUNCATE 本身就会隐式提交）。先清空再逐批重建，
// 期间路由表是不完整的，失败批次对应的渠道会一直没有 ability——即“无可用渠道”。
// 因此它只适合作为管理员手动修复入口，不能放到启动流程或定时任务里。
func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	// 表刚被清空，不需要再逐批 Delete 一次（以前那句 chunk 内 Delete 是早期实现的遗留）。
	for _, chunk := range lo.Chunk(channels, 50) {
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
