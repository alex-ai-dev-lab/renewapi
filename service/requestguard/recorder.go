package requestguard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func loadEndpointSecret(endpointID string) string {
	key := EndpointSecretOptionKey(endpointID)
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	return strings.TrimSpace(common.Interface2String(value))
}

func EndpointSecretOptionKey(endpointID string) string {
	return "request_guard_endpoint_secret." + strings.TrimSpace(endpointID) + ".api_key"
}

func HasEndpointSecret(endpointID string) bool {
	return loadEndpointSecret(endpointID) != ""
}

func recordEvent(meta RequestMeta, snapshot Snapshot, result Result, setting *operation_setting.RequestGuardSetting) {
	if setting == nil || (result.Kind == DecisionAllow && !setting.StorePassEvents) {
		return
	}
	categories, _ := common.Marshal(result.Categories)
	event := &model.RequestGuardEvent{
		RequestID: meta.RequestID, UserID: meta.UserID, TokenID: meta.TokenID,
		Group: meta.Group, Protocol: meta.Protocol, Model: meta.Model,
		Mode: meta.Mode, Decision: string(result.Kind), ReasonCode: result.ReasonCode,
		CategoriesText: string(categories), PromptHMAC: promptHMAC(snapshot),
		PromptRunes: snapshot.RuneCount, Truncated: snapshot.Truncated,
		GuardEndpointID: result.EndpointID, GuardModel: result.Model,
		PolicyVersion: result.PolicyVersion, LatencyMs: result.Latency.Milliseconds(), CreatedAt: time.Now().Unix(),
	}
	if setting.StoreRedactedPreview {
		event.RedactedPreview = redactedPreview(snapshot.Text(), 256)
	}
	if err := model.CreateRequestGuardEvent(event); err != nil {
		recordAuditError()
		common.SysError("requestguard: failed to persist audit event")
	}
}

func promptHMAC(snapshot Snapshot) string {
	key := []byte(common.SessionSecret)
	if len(key) == 0 {
		key = snapshot.Digest[:]
	}
	digest := hmac.New(sha256.New, key)
	for _, segment := range snapshot.Segments {
		_, _ = digest.Write([]byte(segment.Role))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(segment.Text))
		_, _ = digest.Write([]byte{0xff})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func redactedPreview(text string, maxRunes int) string {
	text = common.MaskSensitiveInfo(strings.TrimSpace(text))
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	count := 0
	for index := range text {
		if count == maxRunes {
			return text[:index]
		}
		count++
	}
	return text
}
