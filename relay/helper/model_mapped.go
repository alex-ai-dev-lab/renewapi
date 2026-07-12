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
	if isResponsesCompact && info.ModelMappingFallbackChannelId == info.ChannelId && info.ModelMappingFallbackSource != "" {
		mappingModelName = info.ModelMappingFallbackSource
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
		if info.ModelMappingFallbackChannelId != channelId || info.ModelMappingFallbackSource != mappingModelName {
			info.ModelMappingFallbackChannelId = channelId
			info.ModelMappingFallbackSource = mappingModelName
			info.ModelMappingFallbackCandidates = append([]string(nil), candidates...)
			info.ModelMappingFallbackIndex = 0
		}
		if len(info.ModelMappingFallbackCandidates) > 0 {
			if info.ModelMappingFallbackIndex < 0 || info.ModelMappingFallbackIndex >= len(info.ModelMappingFallbackCandidates) {
				info.ModelMappingFallbackIndex = 0
			}
			mappedModel := info.ModelMappingFallbackCandidates[info.ModelMappingFallbackIndex]
			info.IsModelMapped = mappedModel != mappingModelName
			info.UpstreamModelName = mappedModel
		}
	} else if info.ModelMappingFallbackChannelId != info.ChannelId {
		info.ModelMappingFallbackChannelId = info.ChannelId
		info.ModelMappingFallbackSource = mappingModelName
		info.ModelMappingFallbackCandidates = nil
		info.ModelMappingFallbackIndex = 0
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}

// AdvanceModelMappingFallback advances to the next ordered upstream model for
// the current channel without performing cross-channel selection.
func AdvanceModelMappingFallback(info *common.RelayInfo) (string, string, bool) {
	if info == nil || info.ChannelMeta == nil || info.ModelMappingFallbackChannelId != info.ChannelId {
		return "", "", false
	}
	next := info.ModelMappingFallbackIndex + 1
	if next >= len(info.ModelMappingFallbackCandidates) {
		return "", "", false
	}
	previous := info.ModelMappingFallbackCandidates[info.ModelMappingFallbackIndex]
	info.ModelMappingFallbackIndex = next
	return previous, info.ModelMappingFallbackCandidates[next], true
}
