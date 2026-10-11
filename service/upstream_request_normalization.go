package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UsesCodexRequestProfile(info *relaycommon.RelayInfo) bool {
	return info != nil && info.ChannelMeta != nil &&
		(info.ChannelType == constant.ChannelTypeCodex || info.ChannelSetting.ResponsesRequestProfile == "codex")
}

func RecordRequestNormalization(info *relaycommon.RelayInfo, action string) {
	if info == nil {
		return
	}
	for _, existing := range info.RequestNormalization {
		if existing == action {
			return
		}
	}
	info.RequestNormalization = append(info.RequestNormalization, action)
}

func generatedCodexCacheKey(c *gin.Context, info *relaycommon.RelayInfo) string {
	if info.GeneratedRequestCacheKeys == nil {
		info.GeneratedRequestCacheKeys = make(map[int]string)
	}
	if key := info.GeneratedRequestCacheKeys[info.ChannelId]; key != "" {
		return key
	}
	session := ""
	if c != nil && c.Request != nil {
		for _, name := range []string{"Session-Id", "Session_id", "X-Session-Id", "X-Claude-Code-Session-Id"} {
			if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
				session = value
				break
			}
		}
	}
	// A missing session is request-scoped, not a fake global conversation.
	if session == "" {
		session = uuid.NewString()
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("renewapi-codex\x00%d\x00%d\x00%s", info.UserId, info.ChannelId, session)))
	// Responses providers cap prompt_cache_key at 64 characters. The hash
	// already namespaces the user/channel/session; a textual prefix exceeds it.
	key := hex.EncodeToString(hash[:])
	info.GeneratedRequestCacheKeys[info.ChannelId] = key
	return key
}

// FinalizeUpstreamJSON is shared by HTTP, native WS (via DoApiRequest), tests
// and all three text entry protocols. Run after adapter/legacy overrides and
// before identity injection. No DTO round-trip may follow this function.
func FinalizeUpstreamJSON(c *gin.Context, info *relaycommon.RelayInfo, path string, data []byte) ([]byte, error) {
	if info == nil || info.ChannelMeta == nil {
		return data, nil
	}
	if info.ResponsesRequestKind != dto.ResponsesNormal || strings.HasSuffix(path, "/compact") {
		return data, nil
	}
	path = strings.TrimRight(path, "/")
	if !strings.HasSuffix(path, "/responses") && !strings.HasSuffix(path, "/chat/completions") && !strings.HasSuffix(path, "/messages") {
		return data, nil
	}
	defer func() {
		if info.Failover != nil && len(info.Failover.AttemptRecords) > 0 {
			info.Failover.AttemptRecords[len(info.Failover.AttemptRecords)-1].RequestNormalization = append([]string(nil), info.RequestNormalization...)
		}
		if c != nil {
			c.Set("request_normalization", append([]string(nil), info.RequestNormalization...))
		}
	}()
	var err error
	data, err = relaycommon.ApplyUpstreamParamOverrideWithRelayInfo(data, info)
	if err != nil {
		if rejection, ok := relaycommon.AsParamOverrideReturnError(err); ok {
			return nil, relaycommon.NewAPIErrorFromParamOverride(rejection)
		}
		return nil, err
	}
	data, err = relaycommon.RemoveDisabledFields(data, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.TrimRight(path, "/"), "/responses") || !UsesCodexRequestProfile(info) {
		return data, nil
	}
	if info.ChannelOtherSettings.DisableStore {
		return nil, normalizationError("Codex request profile requires store=false; disable_store cannot remove it")
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, normalizationError("Codex request must be a JSON object")
	}
	var modelName string
	if common.Unmarshal(fields["model"], &modelName) != nil || strings.TrimSpace(modelName) == "" {
		return nil, normalizationError("Codex model must be a non-empty string")
	}
	if raw := fields["conversation"]; len(raw) > 0 && string(raw) != "null" {
		return nil, normalizationError("Codex does not support conversation storage")
	}
	if raw := fields["previous_response_id"]; len(raw) > 0 && string(raw) != "null" && string(raw) != `""` {
		return nil, normalizationError("Codex store=false cannot resolve previous_response_id; send complete input history")
	}
	if raw := fields["store"]; len(raw) > 0 && string(raw) != "null" && string(raw) != "false" && string(raw) != "true" {
		return nil, normalizationError("store must be a boolean")
	}
	if err := normalizeCodexInput(fields, info); err != nil {
		return nil, err
	}
	set := func(name string, value any, action string) {
		fields[name], _ = common.Marshal(value)
		RecordRequestNormalization(info, action+" "+name)
	}
	var include []string
	if raw := fields["include"]; len(raw) > 0 && string(raw) != "null" {
		if err := common.Unmarshal(raw, &include); err != nil {
			return nil, normalizationError("include must be an array of strings")
		}
	}
	merged := make([]string, 0, len(include)+1)
	seen := make(map[string]bool)
	for _, item := range append(include, "reasoning.encrypted_content") {
		if !seen[item] {
			merged = append(merged, item)
			seen[item] = true
		}
	}
	hadEncrypted := false
	for _, item := range include {
		if item == "reasoning.encrypted_content" {
			hadEncrypted = true
		}
	}
	if len(include) != len(merged) || !hadEncrypted {
		set("include", merged, "merged")
	}
	var cacheKey string
	if raw := fields["prompt_cache_key"]; len(raw) > 0 && string(raw) != "null" {
		if err := common.Unmarshal(raw, &cacheKey); err != nil {
			return nil, normalizationError("prompt_cache_key must be a string")
		}
	}
	if strings.TrimSpace(cacheKey) == "" {
		set("prompt_cache_key", generatedCodexCacheKey(c, info), "generated")
	}
	if raw := fields["instructions"]; len(raw) == 0 || string(raw) == "null" {
		set("instructions", "", "added")
	} else {
		var instructions string
		if common.Unmarshal(raw, &instructions) != nil {
			return nil, normalizationError("Codex instructions must be a string")
		}
	}
	if raw := fields["store"]; string(raw) != "false" {
		if string(raw) == "true" {
			return nil, normalizationError("Codex store=false cannot preserve a request that requires server-side conversation storage")
		}
		set("store", false, "set")
	}
	if string(fields["stream"]) != "true" {
		set("stream", true, "set")
	}
	if c != nil {
		c.Set("responses_upstream_stream", true)
	}
	for _, name := range []string{"temperature", "top_p"} {
		if _, exists := fields[name]; exists {
			delete(fields, name)
			RecordRequestNormalization(info, "removed unsupported "+name)
		}
	}
	supportsLimit := info.ChannelType != constant.ChannelTypeCodex
	if configured := info.ChannelSetting.ResponsesSupportsMaxOutputTokens; configured != nil {
		supportsLimit = *configured
	}
	if !supportsLimit {
		if _, exists := fields["max_output_tokens"]; exists {
			return nil, normalizationError("This channel does not support max_output_tokens; the requested output limit cannot be preserved")
		}
	}
	RecordRequestNormalization(info, "profile codex")
	return common.Marshal(fields)
}

func normalizationError(message string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(fmt.Errorf("%s", message), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

func normalizeCodexInput(fields map[string]common.RawMessage, info *relaycommon.RelayInfo) error {
	raw := fields["input"]
	if len(raw) == 0 || string(raw) == "null" {
		return normalizationError("Codex input is required")
	}
	var items []map[string]common.RawMessage
	if common.GetJsonType(raw) == "string" {
		var content string
		if common.Unmarshal(raw, &content) != nil {
			return normalizationError("invalid Codex input")
		}
		role, _ := common.Marshal("user")
		items = []map[string]common.RawMessage{{"role": role, "content": raw}}
	} else if common.Unmarshal(raw, &items) != nil {
		return normalizationError("Codex input must be a string or array")
	}
	changed := common.GetJsonType(raw) == "string"
	for _, item := range items {
		if common.GetJsonType(item["content"]) != "string" {
			continue
		}
		var content, role string
		_ = common.Unmarshal(item["content"], &content)
		_ = common.Unmarshal(item["role"], &role)
		kind := "input_text"
		if role == "assistant" {
			kind = "output_text"
		}
		item["content"], _ = common.Marshal([]map[string]any{{"type": kind, "text": content}})
		changed = true
	}
	if changed {
		fields["input"], _ = common.Marshal(items)
		RecordRequestNormalization(info, "normalized input content blocks")
	}
	return nil
}

func RecordClaudeResponsesConversion(info *relaycommon.RelayInfo, request *dto.ClaudeRequest) {
	if request == nil {
		return
	}
	if len(request.CacheControl) > 0 {
		RecordRequestNormalization(info, "ignored Claude cache_control: target has no block-cache TTL")
	}
	if request.Thinking != nil {
		RecordRequestNormalization(info, "mapped thinking to reasoning effort (budget approximation)")
	}
	tools, _ := common.Any2Type[[]map[string]any](request.Tools)
	for _, tool := range tools {
		if tool["cache_control"] != nil {
			RecordRequestNormalization(info, "ignored Claude cache_control: target has no block-cache TTL")
		}
	}
	for _, message := range request.Messages {
		blocks, _ := message.ParseContent()
		for _, block := range blocks {
			if len(block.CacheControl) > 0 {
				RecordRequestNormalization(info, "ignored Claude cache_control: target has no block-cache TTL")
			}
			if block.IsError != nil && *block.IsError {
				RecordRequestNormalization(info, "mapped tool_result.is_error into tool output")
			}
		}
	}
	if request.System != nil && !request.IsStringSystem() {
		for _, block := range request.ParseSystem() {
			if len(block.CacheControl) > 0 {
				RecordRequestNormalization(info, "ignored Claude cache_control: target has no block-cache TTL")
			}
		}
	}
}
