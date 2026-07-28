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
}

var relayRouteDottedVersionRE = regexp.MustCompile(`\d+(?:\.\d+)+`)
var relayRouteDatedSuffixRE = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

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
			return nil
		}
		version = append(version, value)
	}
	return version
}

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
	routes := make([]RelayModelRoles, 0, 16)
	for _, group := range groups {
		groupRoutes, err := buildResponsesRelayRoutesForGroup(params, group)
		if err != nil {
			return nil, err
		}
		routes = append(routes, groupRoutes...)
	}
	maxCandidates := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_ROUTE_CANDIDATES", 12)
	if maxCandidates > 0 && len(routes) > maxCandidates {
		routes = routes[:maxCandidates]
	}
	if len(routes) == 0 {
		return nil, errors.New("no channel/model candidate satisfies responses compaction requirements")
	}
	// Plans built after the distributor installed a context must retain the
	// installed pair. Distributor-built plans leave InitialChannelId at zero and
	// therefore select the actual highest-priority (channel, model) pair.
	if params.InitialChannelId > 0 &&
		(routes[0].PreferredChannelId != params.InitialChannelId ||
			!strings.EqualFold(routes[0].RoutingModel, params.PrimaryModel)) {
		return nil, errors.New("initial channel/model no longer satisfies responses compaction requirements")
	}
	return &RelayRoutePlan{routes: routes}, nil
}

func buildResponsesRelayRoutesForGroup(params ResponsesRelayRoutePlanParams, group string) ([]RelayModelRoles, error) {
	channels := model.CacheGetAllChannels()
	if len(channels) == 0 {
		var err error
		channels, err = model.GetAllChannels(0, 0, true, false)
		if err != nil {
			return nil, err
		}
	}
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

	allowCrossFamily := common.GetEnvOrDefaultBool("RESPONSES_COMPACTION_CROSS_FAMILY_FALLBACK", false)
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
			if !allowCrossFamily && !strings.EqualFold(relayRouteModelFamily(modelName), primaryFamily) {
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
		maxModelsPerChannel := common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_MODELS_PER_CHANNEL", 3)
		if maxModelsPerChannel > 0 && len(candidates) > maxModelsPerChannel {
			candidates = candidates[:maxModelsPerChannel]
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
	return routes, nil
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
