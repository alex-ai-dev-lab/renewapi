package requestguard

import (
	"path"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func matchScope(setting *operation_setting.RequestGuardSetting, info *relaycommon.RelayInfo, request dto.Request) (string, bool) {
	if setting == nil || info == nil {
		return "", false
	}
	protocol := requestProtocol(info, request)
	if protocol == "" || !containsFold(setting.Scope.Protocols, protocol) {
		return protocol, false
	}

	group := strings.ToLower(strings.TrimSpace(info.UsingGroup))
	if group == "" {
		group = strings.ToLower(strings.TrimSpace(info.TokenGroup))
	}
	if !setting.Scope.AllGroups && !containsFold(setting.Scope.Groups, group) {
		return protocol, false
	}

	model := strings.ToLower(strings.TrimSpace(info.OriginModelName))
	if len(setting.Scope.Models) == 0 || !matchesAnyPattern(setting.Scope.Models, model) {
		return protocol, false
	}
	return protocol, true
}

func requestProtocol(info *relaycommon.RelayInfo, request dto.Request) string {
	if info != nil {
		switch info.RelayFormat {
		case types.RelayFormatOpenAI:
			return "openai_chat"
		case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
			return "openai_responses"
		case types.RelayFormatClaude:
			return "anthropic"
		case types.RelayFormatGemini:
			return "gemini"
		case types.RelayFormatOpenAIImage:
			return "image"
		case types.RelayFormatOpenAIAudio:
			return "audio"
		case types.RelayFormatEmbedding:
			return "embedding"
		case types.RelayFormatRerank:
			return "rerank"
		}
	}

	switch request.(type) {
	case *dto.GeneralOpenAIRequest:
		return "openai_chat"
	case *dto.OpenAIResponsesRequest, *dto.OpenAIResponsesCompactionRequest:
		return "openai_responses"
	case *dto.ClaudeRequest:
		return "anthropic"
	case *dto.GeminiChatRequest:
		return "gemini"
	case *dto.ImageRequest:
		return "image"
	case *dto.AudioRequest:
		return "audio"
	case *dto.EmbeddingRequest:
		return "embedding"
	case *dto.RerankRequest:
		return "rerank"
	default:
		return ""
	}
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func matchesAnyPattern(patterns []string, value string) bool {
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "*" || pattern == value {
			return true
		}
		if matched, err := path.Match(pattern, value); err == nil && matched {
			return true
		}
	}
	return false
}
