package helper

import (
	"fmt"
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
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
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}
	if isResponsesCompact && info.ModelMappingRoute.ChannelId == info.ChannelId && info.ModelMappingRoute.Source != "" {
		mappingModelName = info.ModelMappingRoute.Source
	}

	// A mapping value may be a legacy string or an ordered array. RelayInfo
	// retains the chosen index so the retry controller can advance within the
	// same channel before falling back to another channel.
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap, err := appcommon.ParseModelMapping(modelMapping)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}
		candidates, err := appcommon.ResolveModelMappingCandidates(modelMap, mappingModelName)
		if err != nil {
			return fmt.Errorf("model_mapping_contains_cycle")
		}
		channelId := info.ChannelId
		if info.ModelMappingRoute.ChannelId != channelId || info.ModelMappingRoute.Source != mappingModelName {
			info.ModelMappingRoute = common.ModelMappingRouteCursor{
				ChannelId:  channelId,
				Source:     mappingModelName,
				Candidates: append([]string(nil), candidates...),
			}
		}
		if len(info.ModelMappingRoute.Candidates) > 0 {
			if info.ModelMappingRoute.Index < 0 || info.ModelMappingRoute.Index >= len(info.ModelMappingRoute.Candidates) {
				info.ModelMappingRoute.Index = 0
			}
			mappedModel := info.ModelMappingRoute.Candidates[info.ModelMappingRoute.Index]
			info.IsModelMapped = mappedModel != mappingModelName
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
