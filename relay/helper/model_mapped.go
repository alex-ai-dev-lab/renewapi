package helper

import (
	"fmt"
	"strings"

	basecommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	info.RoutingModelName = originModelName
	info.MappedModelName = originModelName
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}
	if isResponsesCompact && info.ModelMappingRoute.ChannelId == info.ChannelId && info.ModelMappingRoute.Source != "" {
		mappingModelName = info.ModelMappingRoute.Source
	}

	// Reused RelayInfo must not leak the previous channel's mapped target.
	info.IsModelMapped = false
	info.UpstreamModelName = mappingModelName
	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap, err := basecommon.ParseModelMappingConfig(modelMapping)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}
		mappingStartModel := mappingModelName
		if isResponsesCompact {
			for _, source := range modelMap.Sources() {
				if source == originModelName {
					mappingStartModel = originModelName
					break
				}
			}
		}
		resolved, err := modelMap.Candidates(mappingStartModel)
		if err != nil {
			return fmt.Errorf("model_mapping_contains_cycle: %w", err)
		}
		candidates := make([]string, 0, len(resolved))
		ruleIDs := make([]string, 0, len(resolved))
		for _, candidate := range resolved {
			candidates = append(candidates, candidate.Model)
			ruleIDs = append(ruleIDs, candidate.RuleID)
		}
		channelId := info.ChannelId
		if info.ModelMappingRoute.ChannelId != channelId || info.ModelMappingRoute.Source != mappingStartModel {
			info.ModelMappingRoute = common.ModelMappingRouteCursor{
				ChannelId:  channelId,
				Source:     mappingStartModel,
				Candidates: append([]string(nil), candidates...),
				RuleIDs:    ruleIDs,
			}
		}
		if len(info.ModelMappingRoute.Candidates) > 0 {
			if info.ModelMappingRoute.Index < 0 || info.ModelMappingRoute.Index >= len(info.ModelMappingRoute.Candidates) {
				info.ModelMappingRoute.Index = 0
			}
			mappedModel := info.ModelMappingRoute.Candidates[info.ModelMappingRoute.Index]
			info.IsModelMapped = mappedModel != mappingModelName
			info.MappedModelName = mappedModel
			info.UpstreamModelName = mappedModel
		}
	} else if info.ModelMappingRoute.ChannelId != info.ChannelId {
		info.ModelMappingRoute = common.ModelMappingRouteCursor{ChannelId: info.ChannelId, Source: mappingModelName}
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.MappedModelName = finalUpstreamModelName
	}
	if info.Failover != nil && len(info.Failover.AttemptRecords) > 0 {
		attempt := &info.Failover.AttemptRecords[len(info.Failover.AttemptRecords)-1]
		attempt.UpstreamModel = info.UpstreamModelName
		cursor := info.ModelMappingRoute
		if cursor.Index >= 0 && cursor.Index < len(cursor.RuleIDs) {
			attempt.MappingRuleID = cursor.RuleIDs[cursor.Index]
		}
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}

// AdvanceModelMappingFallback advances to the next ordered upstream model for
// the current channel without performing cross-channel selection.
func AdvanceModelMappingFallback(info *common.RelayInfo) (string, string, bool) {
	if info == nil || info.ChannelMeta == nil || info.ModelMappingRoute.ChannelId != info.ChannelId {
		return "", "", false
	}
	next := info.ModelMappingRoute.Index + 1
	if next >= len(info.ModelMappingRoute.Candidates) {
		return "", "", false
	}
	previous := info.ModelMappingRoute.Candidates[info.ModelMappingRoute.Index]
	info.ModelMappingRoute.Index = next
	return previous, info.ModelMappingRoute.Candidates[next], true
}
