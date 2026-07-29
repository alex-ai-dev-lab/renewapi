package service

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// RelayModelRoles names the models used at each boundary of a relay attempt.
// Keeping these roles explicit prevents an upstream mapping or capability-only
// route from silently changing client authorization or billing ownership.
type RelayModelRoles struct {
	ClientModel            string
	RoutingModel           string
	BillingModel           string
	RequiredModel          string
	Group                  string
	PreferredChannelId     int
	StrictPreferredChannel bool
	RetryBudget            int
}

type RelayRoutePlan struct {
	routes []RelayModelRoles
	index  int
}

// ResponsesRelayRoutePlanParams 里有三个含义相近的渠道 ID，注意区分：
//   - PinnedChannelId: 硬过滤，只保留这一个渠道
//   - InitialChannelId: distributor 已经安装到 context 的渠道，必须仍是计划的第一项
//   - PreferredChannelId: 软偏好，默认只在同优先级内生效；PreferChannelFirst=true 时提到最前
type ResponsesRelayRoutePlanParams struct {
	Group              string
	Groups             []string
	ClientModel        string
	PrimaryModel       string
	RequiredModel      string
	InitialChannelId   int
	PinnedChannelId    int
	PreferredChannelId int
	PreferChannelFirst bool
	Requirement        *ResponsesRoutingRequirement
	Request            dto.Request
	ProviderPolicy     *ProviderRoutingPolicy
	TokenModelAllowed  func(string) bool
	ChannelAllowed     func(*model.Channel) bool
}

// relayRoutePlanLimits 缓存一次请求内的环境变量取值。
// 这些值原先是在每分组、甚至每渠道的循环体内反复读取的。
type relayRoutePlanLimits struct {
	maxCandidates       int
	maxModelsPerChannel int
	allowCrossFamily    bool
}

func loadRelayRoutePlanLimits() relayRoutePlanLimits {
	return relayRoutePlanLimits{
		maxCandidates:       common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_ROUTE_CANDIDATES", 12),
		maxModelsPerChannel: common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_MODELS_PER_CHANNEL", 3),
		allowCrossFamily:    common.GetEnvOrDefaultBool("RESPONSES_COMPACTION_CROSS_FAMILY_FALLBACK", false),
	}
}

var relayRouteDottedVersionRE = regexp.MustCompile(`\d+(?:\.\d+)+`)
var relayRouteDatedSuffixRE = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

// relayRouteModelFamily 把模型名归一到“家族”，用于禁止跨家族回退。
// 局限性（默认 RESPONSES_COMPACTION_CROSS_FAMILY_FALLBACK=false 时影响很大）：
//   - 只识别点号版本号，claude-3-5-sonnet 这种连字符版本号不会被归一，
//     与 claude-3.5-sonnet 会被判成两个不同家族；
//   - 只剥离 -YYYY-MM-DD 形式的日期后缀，-20240620 / -latest / -preview 都留在名字里，
//     于是 xxx-20240620 与 xxx 也是两个家族。
//
// 结果是不少本该允许的同家族回退被静默丢弃。彻底修需要一份显式的家族映射表。
func relayRouteModelFamily(modelName string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	normalized = relayRouteDatedSuffixRE.ReplaceAllString(normalized, "")
	return relayRouteDottedVersionRE.ReplaceAllString(normalized, "{version}")
}

func relayRouteVersion(modelName string) []int {
	raw := relayRouteDottedVersionRE.FindString(strings.ToLower(modelName))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ".")
	version := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			// 正则已保证全是数字，这里只防御超长数字溢出。
			return nil
		}
		version = append(version, value)
	}
	return version
}

// compareRelayRouteVersionDesc 版本号降序比较。
// 注意: 完全不含点号版本号的模型名 version 为 nil，会被当成全 0，即排在最后。
func compareRelayRouteVersionDesc(left, right string) int {
	a := relayRouteVersion(left)
	b := relayRouteVersion(right)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av > bv {
			return -1
		}
		if av < bv {
			return 1
		}
	}
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func channelContainsGroup(channel *model.Channel, group string) bool {
	for _, candidate := range channel.GetGroups() {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(group)) {
			return true
		}
	}
	return false
}

// routeRetryBudget 用 ChannelInfo.MultiKeySize 推算多 Key 重试预算。
// 注意: service/channel_select.go 走的是 MultiKeyStatusList / GetEnabledKeyByIndex，
// 两处口径不同——MultiKeySize 是写库时算好的计数，Key 列表变更后若未同步就会偏大。
func routeRetryBudget(channel *model.Channel) int {
	if channel == nil || !channel.ChannelInfo.IsMultiKey || channel.ChannelInfo.MultiKeySize <= 1 {
		return 0
	}
	budget := channel.ChannelInfo.MultiKeySize - 1
	if budget > common.RetryTimes {
		budget = common.RetryTimes
	}
	return budget
}

// resolveRelayRouteChannels 加载一次渠道列表。
// 原先这段在每个分组的循环体里各执行一次，内存缓存未命中时相当于
// 一个请求做 len(groups) 次 GetAllChannels 全表加载。
func resolveRelayRouteChannels() ([]*model.Channel, error) {
	channels := model.CacheGetAllChannels()
	if len(channels) > 0 {
		return channels, nil
	}
	return model.GetAllChannels(0, 0, true, false)
}

func BuildResponsesRelayRoutePlan(params ResponsesRelayRoutePlanParams) (*RelayRoutePlan, error) {
	params.Group = strings.TrimSpace(params.Group)
	params.ClientModel = strings.TrimSpace(params.ClientModel)
	params.PrimaryModel = strings.TrimSpace(params.PrimaryModel)
	params.RequiredModel = strings.TrimSpace(params.RequiredModel)
	groups := make([]string, 0, len(params.Groups)+1)
	seenGroups := make(map[string]struct{}, len(params.Groups)+1)
	for _, group := range append([]string{params.Group}, params.Groups...) {
		group = strings.TrimSpace(group)
		key := strings.ToLower(group)
		if group == "" || key == "auto" {
			continue
		}
		if _, exists := seenGroups[key]; exists {
			continue
		}
		seenGroups[key] = struct{}{}
		groups = append(groups, group)
	}
	if len(groups) == 0 || params.PrimaryModel == "" || params.Requirement == nil {
		return nil, errors.New("responses route plan requires group, primary model, and capability requirement")
	}
	channels, err := resolveRelayRouteChannels()
	if err != nil {
		return nil, err
	}
	limits := loadRelayRoutePlanLimits()
	groupRoutes := make([][]RelayModelRoles, 0, len(groups))
	for _, group := range groups {
		groupRoutes = append(groupRoutes, buildResponsesRelayRoutesForGroup(params, group, channels, limits))
	}
	firstRoute, hasFirstRoute := firstRelayRoute(groupRoutes)
	if !hasFirstRoute {
		return nil, errors.New("no channel/model candidate satisfies responses compaction requirements")
	}
	// Plans built after the distributor installed a context must retain the
	// installed pair. Distributor-built plans leave InitialChannelId at zero and
	// therefore select the actual highest-priority (channel, model) pair.
	//
	// 这个校验必须在按预算截断之前做：否则一个完全合法的已安装渠道只要被
	// maxCandidates 截掉，就会误报 "initial channel/model no longer satisfies"。
	if params.InitialChannelId > 0 &&
		(firstRoute.PreferredChannelId != params.InitialChannelId ||
			!strings.EqualFold(firstRoute.RoutingModel, params.PrimaryModel)) {
		return nil, errors.New("initial channel/model no longer satisfies responses compaction requirements")
	}
	routes := flattenRelayRoutesWithBudget(groupRoutes, limits.maxCandidates)
	if len(routes) == 0 {
		return nil, errors.New("no channel/model candidate satisfies responses compaction requirements")
	}
	return &RelayRoutePlan{routes: routes}, nil
}

func firstRelayRoute(groupRoutes [][]RelayModelRoles) (RelayModelRoles, bool) {
	for _, routes := range groupRoutes {
		if len(routes) > 0 {
			return routes[0], true
		}
	}
	return RelayModelRoles{}, false
}

// flattenRelayRoutesWithBudget 把各分组的候选拼成一个计划，同时尊重总名额上限。
//
// 旧实现是先无脑拼接再 routes[:maxCandidates]，于是只要第一个分组的候选数
// 就够 maxCandidates（默认 12，配合每渠道 3 个模型只需 4 个渠道），
// 后面所有后备分组永远拿不到一个名额——跨分组后备形同虚设。
//
// 现在先给每个分组分配 maxCandidates/len(groups) 的基础预算（至少 1），
// 再按分组顺序用剩余名额补齐，保证每个非空分组都至少有一个候选。
func flattenRelayRoutesWithBudget(groupRoutes [][]RelayModelRoles, maxCandidates int) []RelayModelRoles {
	total := 0
	for _, routes := range groupRoutes {
		total += len(routes)
	}
	flat := make([]RelayModelRoles, 0, total)
	if maxCandidates <= 0 || total <= maxCandidates {
		for _, routes := range groupRoutes {
			flat = append(flat, routes...)
		}
		return flat
	}
	perGroup := maxCandidates / len(groupRoutes)
	if perGroup < 1 {
		perGroup = 1
	}
	consumed := make([]int, len(groupRoutes))
	for i, routes := range groupRoutes {
		if len(flat) >= maxCandidates {
			break
		}
		take := perGroup
		if take > len(routes) {
			take = len(routes)
		}
		if len(flat)+take > maxCandidates {
			take = maxCandidates - len(flat)
		}
		flat = append(flat, routes[:take]...)
		consumed[i] = take
	}
	for i, routes := range groupRoutes {
		if len(flat) >= maxCandidates {
			break
		}
		take := len(routes) - consumed[i]
		if take <= 0 {
			continue
		}
		if len(flat)+take > maxCandidates {
			take = maxCandidates - len(flat)
		}
		flat = append(flat, routes[consumed[i]:consumed[i]+take]...)
	}
	return flat
}

func buildResponsesRelayRoutesForGroup(
	params ResponsesRelayRoutePlanParams,
	group string,
	channels []*model.Channel,
	limits relayRoutePlanLimits,
) []RelayModelRoles {
	filtered := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled || !channelContainsGroup(channel, group) {
			continue
		}
		if params.PinnedChannelId > 0 && channel.Id != params.PinnedChannelId {
			continue
		}
		if !ChannelMatchesProviderRoutingPolicy(channel, params.ProviderPolicy) || !ChannelAllowedForProduction(channel) {
			continue
		}
		if params.ChannelAllowed != nil && !params.ChannelAllowed(channel) {
			continue
		}
		filtered = append(filtered, channel)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Id == params.InitialChannelId {
			return true
		}
		if filtered[j].Id == params.InitialChannelId {
			return false
		}
		if params.PreferChannelFirst {
			if filtered[i].Id == params.PreferredChannelId {
				return true
			}
			if filtered[j].Id == params.PreferredChannelId {
				return false
			}
		}
		if filtered[i].GetPriority() != filtered[j].GetPriority() {
			return filtered[i].GetPriority() > filtered[j].GetPriority()
		}
		if filtered[i].Id == params.PreferredChannelId {
			return true
		}
		if filtered[j].Id == params.PreferredChannelId {
			return false
		}
		leftRank := ProviderRoutingOrderRank(filtered[i], params.ProviderPolicy)
		rightRank := ProviderRoutingOrderRank(filtered[j], params.ProviderPolicy)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return filtered[i].Id < filtered[j].Id
	})

	primaryFamily := relayRouteModelFamily(params.PrimaryModel)
	routes := make([]RelayModelRoles, 0, 16)
	for _, channel := range filtered {
		modelNames := append([]string{params.PrimaryModel}, channel.GetRoutingModels()...)
		seen := make(map[string]struct{}, len(modelNames))
		candidates := make([]string, 0, len(modelNames))
		for _, modelName := range modelNames {
			modelName = strings.TrimSpace(modelName)
			key := strings.ToLower(modelName)
			if modelName == "" || strings.ContainsAny(modelName, "*?") {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if params.TokenModelAllowed != nil && !params.TokenModelAllowed(modelName) {
				continue
			}
			requiredModel := relayRouteRequiredModel(params, modelName)
			if requiredModel != "" && params.TokenModelAllowed != nil && !params.TokenModelAllowed(requiredModel) {
				continue
			}
			if !limits.allowCrossFamily && !strings.EqualFold(relayRouteModelFamily(modelName), primaryFamily) {
				continue
			}
			if !model.IsChannelEnabledForGroupModel(group, modelName, channel.Id) ||
				model.IsChannelModelDisabledForGroup(channel.Id, group, modelName) {
				continue
			}
			if requiredModel != "" && !strings.EqualFold(requiredModel, modelName) &&
				(!model.IsChannelEnabledForGroupModel(group, requiredModel, channel.Id) ||
					model.IsChannelModelDisabledForGroup(channel.Id, group, requiredModel)) {
				continue
			}
			requirement := *params.Requirement
			requirement.RequiredContinuationModel = requiredModel
			if !ChannelMatchesResponsesRequirement(channel, modelName, &requirement, params.Request) {
				continue
			}
			candidates = append(candidates, modelName)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			leftExact := strings.EqualFold(candidates[i], params.PrimaryModel)
			rightExact := strings.EqualFold(candidates[j], params.PrimaryModel)
			if leftExact != rightExact {
				return leftExact
			}
			return compareRelayRouteVersionDesc(candidates[i], candidates[j]) < 0
		})
		if limits.maxModelsPerChannel > 0 && len(candidates) > limits.maxModelsPerChannel {
			candidates = candidates[:limits.maxModelsPerChannel]
		}
		for _, modelName := range candidates {
			requiredModel := relayRouteRequiredModel(params, modelName)
			routes = append(routes, RelayModelRoles{
				ClientModel:            params.ClientModel,
				RoutingModel:           modelName,
				BillingModel:           params.ClientModel,
				RequiredModel:          requiredModel,
				Group:                  group,
				PreferredChannelId:     channel.Id,
				StrictPreferredChannel: true,
				RetryBudget:            routeRetryBudget(channel),
			})
		}
	}
	return routes
}

func relayRouteRequiredModel(params ResponsesRelayRoutePlanParams, routingModel string) string {
	requiredModel := strings.TrimSpace(params.RequiredModel)
	if requiredModel == "" && params.Requirement != nil && params.Requirement.Kind == dto.ResponsesCompactionTrigger &&
		params.ClientModel != "" && !strings.EqualFold(params.ClientModel, routingModel) {
		requiredModel = strings.TrimSpace(params.ClientModel)
	}
	return requiredModel
}

func NewRelayRoutePlan(clientModel, primaryModel, requiredModel string, fallbackModels []string) *RelayRoutePlan {
	clientModel = strings.TrimSpace(clientModel)
	primaryModel = strings.TrimSpace(primaryModel)
	if clientModel == "" {
		clientModel = primaryModel
	}
	models := make([]string, 0, len(fallbackModels)+1)
	models = append(models, primaryModel)
	models = append(models, fallbackModels...)
	seen := make(map[string]struct{}, len(models))
	routes := make([]RelayModelRoles, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		key := strings.ToLower(modelName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		routes = append(routes, RelayModelRoles{
			ClientModel:   clientModel,
			RoutingModel:  modelName,
			BillingModel:  clientModel,
			RequiredModel: strings.TrimSpace(requiredModel),
		})
	}
	// 注意: primaryModel 与 fallbackModels 全为空时会返回一个零候选的计划，
	// 调用方只能通过 Current() 返回 false 感知，这里不报错。
	return &RelayRoutePlan{routes: routes}
}

func (p *RelayRoutePlan) Current() (RelayModelRoles, bool) {
	if p == nil || p.index < 0 || p.index >= len(p.routes) {
		return RelayModelRoles{}, false
	}
	return p.routes[p.index], true
}

func (p *RelayRoutePlan) Advance() bool {
	if p == nil || p.index+1 >= len(p.routes) {
		return false
	}
	p.index++
	return true
}

func (p *RelayRoutePlan) HasNext() bool {
	return p != nil && p.index+1 < len(p.routes)
}

func (p *RelayRoutePlan) Position() int {
	if p == nil {
		return -1
	}
	return p.index
}

func (p *RelayRoutePlan) Len() int {
	if p == nil {
		return 0
	}
	return len(p.routes)
}
