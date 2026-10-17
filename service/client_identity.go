package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

// ApplyClientIdentityToRequest 应用客户端标识符到请求
// 返回修改后的 body、是否修改标志和错误
func ApplyClientIdentityToRequest(path string, headers http.Header, body []byte) ([]byte, bool, error) {
	return ApplyClientIdentityToRequestWithOptions(path, headers, body, ClientIdentityOptions{})
}

type ClientIdentityOptions struct {
	ForceCodexIdentity bool
}

// ApplyClientIdentityToRequestWithOptions applies client identifiers to the
// request. ForceCodexIdentity is used after channel selection for Codex-like
// upstreams that require Codex client metadata even when the incoming endpoint
// is /v1/chat/completions.
func ApplyClientIdentityToRequestWithOptions(path string, headers http.Header, body []byte, options ClientIdentityOptions) ([]byte, bool, error) {
	setting := model.GetClientIdentitySetting()
	if !setting.Enabled {
		return body, false, nil
	}

	// 检查 Content-Type
	contentType := headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return body, false, nil
	}

	// 尝试解析 JSON
	var payload map[string]interface{}
	if err := common.Unmarshal(body, &payload); err != nil {
		// 无效 JSON，不处理
		return body, false, nil
	}

	changed := false

	// 检测并应用 Codex 标识符
	if (options.ForceCodexIdentity && isOpenAICodexIdentityEligibleRequest(path, payload)) ||
		isCodexLikeRequest(path, headers, payload, setting.ApplyToAllOpenAIResponses) {
		if applyCodexIdentity(headers, payload, setting.Codex) {
			changed = true
		}
	}

	// 检测并应用 Claude 标识符
	if isClaudeCodeLikeRequest(path, headers, payload, setting.ApplyToAllClaudeMessages) {
		if applyClaudeIdentity(headers, payload, setting.Claude) {
			changed = true
		}
	}

	// 应用通用厂商标识符
	for _, genericCfg := range setting.Generic {
		if applyGenericIdentity(headers, payload, genericCfg) {
			changed = true
		}
	}

	if !changed {
		return body, false, nil
	}

	// 重新序列化 JSON
	modifiedBody, err := common.Marshal(payload)
	if err != nil {
		common.SysLog("client identity: failed to marshal modified body: " + err.Error())
		return body, false, err
	}

	return modifiedBody, true, nil
}

func isOpenAICodexIdentityEligibleRequest(path string, body map[string]interface{}) bool {
	normalizedPath := strings.ToLower(path)
	if strings.Contains(normalizedPath, "/v1/messages") ||
		strings.Contains(normalizedPath, "/anthropic/v1/messages") {
		return false
	}
	if strings.Contains(normalizedPath, "/v1/responses") ||
		strings.Contains(normalizedPath, "/v1/chat/completions") ||
		strings.Contains(normalizedPath, "/v1/completions") {
		return true
	}

	if _, ok := body["messages"]; ok {
		return true
	}
	if _, ok := body["input"]; ok {
		return true
	}
	if _, ok := body["prompt"]; ok {
		return true
	}

	return false
}

// isCodexLikeRequest 检测是否为 Codex 类请求
func isCodexLikeRequest(path string, headers http.Header, body map[string]interface{}, applyToAll bool) bool {
	if !strings.Contains(path, "/v1/responses") {
		return false
	}

	if applyToAll {
		return true
	}

	ua := strings.ToLower(headers.Get("User-Agent"))
	if strings.Contains(ua, "codex") {
		return true
	}

	originator := strings.ToLower(headers.Get("Originator"))
	if strings.Contains(originator, "codex") {
		return true
	}

	if headers.Get("X-Codex-Turn-Metadata") != "" {
		return true
	}

	if _, ok := body["client_metadata"]; ok {
		return true
	}

	return false
}

// isClaudeCodeLikeRequest 检测是否为 Claude Code 类请求
func isClaudeCodeLikeRequest(path string, headers http.Header, body map[string]interface{}, applyToAll bool) bool {
	if !strings.Contains(path, "/v1/messages") {
		return false
	}

	if applyToAll {
		return true
	}

	ua := strings.ToLower(headers.Get("User-Agent"))
	if strings.Contains(ua, "claude-cli") || strings.Contains(ua, "claude-code") || strings.Contains(ua, "claude-vscode") {
		return true
	}

	if headers.Get("X-Claude-Code-Session-Id") != "" {
		return true
	}

	if strings.EqualFold(headers.Get("X-App"), "cli") {
		return true
	}

	if headers.Get("Anthropic-Version") != "" && headers.Get("Anthropic-Beta") != "" {
		if _, ok := body["metadata"]; ok {
			return true
		}
	}

	return false
}

// applyCodexIdentity 应用 Codex 标识符
func applyCodexIdentity(headers http.Header, payload map[string]interface{}, cfg model.CodexIdentityConfig) bool {
	if !cfg.Enabled || cfg.Mode == model.IdentityModeDisabled {
		return false
	}

	id, err := model.EnsureCodexInstallationID()
	if err != nil {
		common.SysLog("client identity: failed to ensure codex installation id: " + err.Error())
		return false
	}

	// 获取或创建 client_metadata
	var metadata map[string]interface{}
	if m, ok := payload["client_metadata"].(map[string]interface{}); ok {
		metadata = m
	} else {
		metadata = make(map[string]interface{})
		payload["client_metadata"] = metadata
	}

	existing, _ := metadata["x-codex-installation-id"].(string)
	existing = strings.TrimSpace(existing)

	if cfg.Mode == model.IdentityModeForceGlobal || existing == "" {
		metadata["x-codex-installation-id"] = id
		logIdentityAction("codex", cfg.Mode, "filled", id)
		return true
	}

	return false
}

// applyClaudeIdentity 应用 Claude 标识符
func applyClaudeIdentity(headers http.Header, payload map[string]interface{}, cfg model.ClaudeIdentityConfig) bool {
	if !cfg.Enabled || cfg.Mode == model.IdentityModeDisabled {
		return false
	}

	deviceID, err := model.EnsureClaudeDeviceID()
	if err != nil {
		common.SysLog("client identity: failed to ensure claude device id: " + err.Error())
		return false
	}

	// 获取或创建 metadata
	var metadata map[string]interface{}
	if m, ok := payload["metadata"].(map[string]interface{}); ok {
		metadata = m
	} else {
		metadata = make(map[string]interface{})
		payload["metadata"] = metadata
	}

	// 解析 user_id
	userObj := make(map[string]interface{})
	rawUserID, hasUserID := metadata["user_id"]

	if hasUserID {
		switch v := rawUserID.(type) {
		case string:
			// 尝试解析 JSON 字符串
			var parsed map[string]interface{}
			if err := common.Unmarshal([]byte(v), &parsed); err == nil && parsed != nil {
				userObj = parsed
			} else {
				// 无效 JSON，保存为 legacy
				userObj["legacy_user_id"] = v
			}
		case map[string]interface{}:
			userObj = v
		default:
			userObj["legacy_user_id"] = fmt.Sprint(v)
		}
	}

	// 应用 device_id
	existingDevice, _ := userObj["device_id"].(string)
	existingDevice = strings.TrimSpace(existingDevice)
	if cfg.Mode == model.IdentityModeForceGlobal || existingDevice == "" {
		userObj["device_id"] = deviceID
		logIdentityAction("claude", cfg.Mode, "filled", deviceID)
	}

	// 处理 session_id
	sessionID := resolveClaudeSessionID(headers, userObj, cfg)
	if sessionID != "" {
		userObj["session_id"] = sessionID
		if cfg.SessionIDMode == model.SessionIDModeForceGlobal || cfg.SyncSessionHeader {
			headers.Set("X-Claude-Code-Session-Id", sessionID)
		}
	}

	// 编码回 JSON 字符串
	encoded, err := common.Marshal(userObj)
	if err != nil {
		common.SysLog("client identity: failed to marshal claude user_id: " + err.Error())
		return false
	}
	metadata["user_id"] = string(encoded)

	return true
}

// resolveClaudeSessionID 解析 Claude session ID
func resolveClaudeSessionID(headers http.Header, userObj map[string]interface{}, cfg model.ClaudeIdentityConfig) string {
	if cfg.SessionIDMode == model.SessionIDModeDisabled {
		existing, _ := userObj["session_id"].(string)
		return existing
	}

	if cfg.SessionIDMode == model.SessionIDModeForceGlobal {
		if fixed := strings.TrimSpace(cfg.FixedSessionID); fixed != "" {
			return fixed
		}
		sessionID, err := model.EnsureClaudeSessionID()
		if err != nil {
			common.SysLog("client identity: failed to ensure claude session id: " + err.Error())
			return ""
		}
		return sessionID
	}

	existing, _ := userObj["session_id"].(string)
	if strings.TrimSpace(existing) != "" {
		return existing
	}

	headerSession := strings.TrimSpace(headers.Get("X-Claude-Code-Session-Id"))
	if headerSession != "" {
		return headerSession
	}

	// 生成请求范围的 session ID
	return uuid.NewString()
}

// applyGenericIdentity 应用通用厂商标识符
func applyGenericIdentity(headers http.Header, payload map[string]interface{}, cfg model.GenericIdentityConfig) bool {
	if !cfg.Enabled || cfg.Mode == model.IdentityModeDisabled {
		return false
	}

	value := strings.TrimSpace(cfg.FieldValue)
	if value == "" {
		// 生成 UUID
		value = uuid.NewString()
	}

	if cfg.PathType == model.PathTypeHeader {
		// 应用到 header
		if cfg.Mode == model.IdentityModeForceGlobal || headers.Get(cfg.FieldPath) == "" {
			headers.Set(cfg.FieldPath, value)
			logIdentityAction("generic:"+cfg.Name, cfg.Mode, "header_filled", value)
			return true
		}
		return false
	}

	// 应用到 body JSON
	if cfg.PathType == model.PathTypeBodyJSON {
		return applyGenericBodyField(payload, cfg.FieldPath, value, cfg.Mode, cfg.Name)
	}

	return false
}

// applyGenericBodyField 将值应用到 JSON body 的指定路径
func applyGenericBodyField(payload map[string]interface{}, path, value, mode, name string) bool {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return false
	}

	// 遍历到倒数第二层
	current := payload
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		next, ok := current[key].(map[string]interface{})
		if !ok {
			// 创建中间层
			next = make(map[string]interface{})
			current[key] = next
		}
		current = next
	}

	// 设置最终值
	finalKey := parts[len(parts)-1]
	existing, _ := current[finalKey].(string)
	existing = strings.TrimSpace(existing)

	if mode == model.IdentityModeForceGlobal || existing == "" {
		current[finalKey] = value
		logIdentityAction("generic:"+name, mode, "body_filled", value)
		return true
	}

	return false
}

// logIdentityAction 记录标识符操作（仅记录指纹）
func logIdentityAction(provider, mode, action, value string) {
	fp := identityFingerprint(value)
	common.SysLog(fmt.Sprintf("client identity applied: provider=%s mode=%s action=%s id_fp=%s", provider, mode, action, fp))
}

// identityFingerprint 生成标识符指纹
func identityFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

// ApplyClientIdentityToRequestBody 从 io.Reader 读取并应用标识符，返回新 Reader 与需要应用到上游请求的 header。
func ApplyClientIdentityToRequestBody(path string, headers http.Header, body io.Reader) (io.Reader, int64, http.Header, error) {
	return ApplyClientIdentityToRequestBodyWithOptions(path, headers, body, ClientIdentityOptions{})
}

func ApplyClientIdentityToRequestBodyWithOptions(path string, headers http.Header, body io.Reader, options ClientIdentityOptions) (io.Reader, int64, http.Header, error) {
	if body == nil {
		return body, 0, nil, nil
	}

	// 读取 body
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return body, 0, nil, err
	}

	modifiedBody, headerPatch, err := ApplyClientIdentityToRequestBytesWithOptions(path, headers, bodyBytes, options)
	return bytes.NewReader(modifiedBody), int64(len(modifiedBody)), headerPatch, err
}

func ApplyClientIdentityToRequestBytesWithOptions(path string, headers http.Header, body []byte, options ClientIdentityOptions) ([]byte, http.Header, error) {
	originalHeaders := headers.Clone()
	modified, changed, err := ApplyClientIdentityToRequestWithOptions(path, headers, body, options)
	if err != nil || !changed {
		modified = body
	}
	return modified, changedIdentityHeaders(originalHeaders, headers), err
}

func changedIdentityHeaders(before, after http.Header) http.Header {
	patch := http.Header{}
	for key, values := range after {
		if len(values) == 0 {
			continue
		}
		beforeValues := before.Values(key)
		if len(beforeValues) == len(values) {
			same := true
			for i := range values {
				if beforeValues[i] != values[i] {
					same = false
					break
				}
			}
			if same {
				continue
			}
		}
		patch[key] = append([]string(nil), values...)
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}
