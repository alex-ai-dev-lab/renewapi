package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type CodexCredentialCandidate struct {
	Index           int      `json:"index"`
	SourceType      string   `json:"source_type"`
	Confidence      int      `json:"confidence"`
	Label           string   `json:"label,omitempty"`
	Key             string   `json:"key"`
	AccountID       string   `json:"account_id,omitempty"`
	AccountIDSource string   `json:"account_id_source,omitempty"`
	Email           string   `json:"email,omitempty"`
	ExpiresAt       string   `json:"expires_at,omitempty"`
	LastRefresh     string   `json:"last_refresh,omitempty"`
	HasRefreshToken bool     `json:"has_refresh_token"`
	HasIDToken      bool     `json:"has_id_token"`
	NonRefreshable  bool     `json:"non_refreshable"`
	Warnings        []string `json:"warnings,omitempty"`
}

type CodexCredentialNormalizeResult struct {
	Candidates []CodexCredentialCandidate `json:"candidates"`
}

type codexCredentialDraft struct {
	sourceType      string
	confidence      int
	label           string
	accessToken     string
	refreshToken    string
	idToken         string
	sessionToken    string
	accountID       string
	accountIDSource string
	email           string
	expiresAt       string
	lastRefresh     string
	planType        string
	userID          string
	warnings        []string
}

type normalizedCodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	SessionToken string `json:"session_token,omitempty"`

	AccountID   string `json:"account_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
	PlanType    string `json:"plan_type,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
}

func NormalizeCodexCredential(raw string) (*CodexCredentialNormalizeResult, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return nil, errors.New("empty credential input")
	}

	var doc any
	if err := json.Unmarshal([]byte(input), &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON credential input: %w", err)
	}

	drafts := collectCodexCredentialDrafts(doc)
	candidates := make([]CodexCredentialCandidate, 0, len(drafts))
	seen := map[string]struct{}{}
	for _, draft := range drafts {
		candidate, ok := finalizeCodexCredentialDraft(draft)
		if !ok {
			continue
		}
		fingerprint := candidate.AccountID + "\x00" + hashCredentialKey(candidate.Key)
		if _, exists := seen[fingerprint]; exists {
			continue
		}
		seen[fingerprint] = struct{}{}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence == candidates[j].Confidence {
			return candidates[i].SourceType < candidates[j].SourceType
		}
		return candidates[i].Confidence > candidates[j].Confidence
	})
	for i := range candidates {
		candidates[i].Index = i
	}
	if len(candidates) == 0 {
		return nil, errors.New("no supported Codex credential found; expected access_token and account_id or a supported export format")
	}
	return &CodexCredentialNormalizeResult{Candidates: candidates}, nil
}

func SelectCodexCredentialCandidate(raw string, index int) (*CodexCredentialCandidate, error) {
	result, err := NormalizeCodexCredential(raw)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(result.Candidates) {
		return nil, fmt.Errorf("candidate index %d out of range", index)
	}
	return &result.Candidates[index], nil
}

func collectCodexCredentialDrafts(doc any) []codexCredentialDraft {
	var drafts []codexCredentialDraft
	collectCodexCredentialDraftsInto(&drafts, doc, "")
	return drafts
}

func collectCodexCredentialDraftsInto(out *[]codexCredentialDraft, doc any, path string) {
	switch v := doc.(type) {
	case []any:
		for i, item := range v {
			collectCodexCredentialDraftsInto(out, item, fmt.Sprintf("%s[%d]", path, i))
		}
	case map[string]any:
		if accounts, ok := arrayValue(v, "accounts"); ok {
			for i, account := range accounts {
				if accountMap, ok := objectValueAny(account); ok {
					if draft, ok := draftFromSub2APIAccount(accountMap, i); ok {
						*out = append(*out, draft)
					}
				}
			}
			return
		}
		if _, ok := objectValue(v, "credentials"); ok {
			if draft, ok := draftFromSub2APIAccount(v, 0); ok {
				*out = append(*out, draft)
			}
		}
		matchedStructured := false
		if draft, ok := draftFromTokensObject(v); ok {
			*out = append(*out, draft)
			matchedStructured = true
		}
		if draft, ok := draftFrom9Router(v); ok {
			*out = append(*out, draft)
			matchedStructured = true
		}
		if draft, ok := draftFromChatGPTSession(v); ok {
			*out = append(*out, draft)
			matchedStructured = true
		}
		if !matchedStructured {
			if draft, ok := draftFromFlatObject(v); ok {
				*out = append(*out, draft)
			}
		}
		for _, key := range []string{"items", "records", "credentials", "data"} {
			if nested, ok := arrayValue(v, key); ok {
				for i, item := range nested {
					collectCodexCredentialDraftsInto(out, item, fmt.Sprintf("%s.%s[%d]", path, key, i))
				}
			}
		}
	}
}

func draftFromSub2APIAccount(account map[string]any, index int) (codexCredentialDraft, bool) {
	credentials, ok := objectValue(account, "credentials")
	if !ok {
		return codexCredentialDraft{}, false
	}
	platform := strings.ToLower(firstString(account, "platform"))
	accountType := strings.ToLower(firstString(account, "type", "account_type"))
	if platform != "" && platform != "openai" && platform != "codex" {
		return codexCredentialDraft{}, false
	}
	if accountType != "" && accountType != "oauth" && accountType != "codex" {
		return codexCredentialDraft{}, false
	}
	extra, _ := objectValue(account, "extra")
	return codexCredentialDraft{
		sourceType:      "sub2api",
		confidence:      96,
		label:           firstString(account, "name", "label"),
		accessToken:     firstString(credentials, "access_token", "accessToken"),
		refreshToken:    firstString(credentials, "refresh_token", "refreshToken"),
		idToken:         firstString(credentials, "id_token", "idToken"),
		accountID:       firstString(credentials, "account_id", "chatgpt_account_id", "chatgptAccountId"),
		accountIDSource: "sub2api.credentials",
		email:           firstNonEmpty(firstString(credentials, "email", "email_address"), firstString(extra, "email", "email_address")),
		expiresAt:       normalizeAnyTimestamp(firstAny(credentials, "expires_at", "expiresAt", "expired", "expires")),
		planType:        firstString(credentials, "plan_type", "chatgpt_plan_type"),
	}, true
}

func draftFromTokensObject(root map[string]any) (codexCredentialDraft, bool) {
	tokens, ok := objectValue(root, "tokens")
	if !ok {
		return codexCredentialDraft{}, false
	}
	meta, _ := objectValue(root, "meta")
	sourceType := "codex-auth"
	confidence := 94
	if len(meta) > 0 {
		sourceType = "codex-manager"
		confidence = 93
	}
	if strings.Contains(strings.ToLower(firstString(root, "auth_mode")), "chatgpt") {
		sourceType = "codex-auth"
		confidence = 96
	}
	return codexCredentialDraft{
		sourceType:      sourceType,
		confidence:      confidence,
		label:           firstNonEmpty(firstString(meta, "label", "name", "workspace_id"), firstString(root, "name", "label")),
		accessToken:     firstString(tokens, "access_token", "accessToken"),
		refreshToken:    cleanMissingPlaceholder(firstString(tokens, "refresh_token", "refreshToken")),
		idToken:         firstString(tokens, "id_token", "idToken"),
		accountID:       firstString(tokens, "account_id", "chatgpt_account_id", "chatgptAccountId"),
		accountIDSource: "tokens",
		email:           firstNonEmpty(firstString(tokens, "email"), firstString(meta, "email")),
		expiresAt:       normalizeAnyTimestamp(firstAny(tokens, "expires_at", "expiresAt", "expired", "expires")),
		lastRefresh:     firstString(root, "last_refresh", "lastRefresh"),
		planType:        firstString(meta, "plan_type", "chatgpt_plan_type"),
	}, true
}

func draftFrom9Router(root map[string]any) (codexCredentialDraft, bool) {
	if firstString(root, "accessToken") == "" {
		return codexCredentialDraft{}, false
	}
	providerData, _ := objectValue(root, "providerSpecificData")
	if len(providerData) == 0 {
		return codexCredentialDraft{}, false
	}
	return codexCredentialDraft{
		sourceType:      "9router",
		confidence:      96,
		label:           firstString(root, "name", "label"),
		accessToken:     firstString(root, "accessToken"),
		refreshToken:    cleanMissingPlaceholder(firstString(root, "refreshToken")),
		idToken:         firstString(root, "idToken"),
		accountID:       firstString(providerData, "chatgptAccountId", "chatgpt_account_id", "account_id"),
		accountIDSource: "9router.providerSpecificData",
		email:           firstNonEmpty(firstString(providerData, "email"), firstString(root, "email")),
		expiresAt:       normalizeAnyTimestamp(firstAny(root, "expiresAt", "expires_at", "expired", "expires")),
		planType:        firstString(providerData, "chatgptPlanType", "planType", "plan_type"),
	}, true
}

func draftFromChatGPTSession(root map[string]any) (codexCredentialDraft, bool) {
	accessToken := firstString(root, "accessToken")
	if accessToken == "" {
		return codexCredentialDraft{}, false
	}
	user, _ := objectValue(root, "user")
	account, _ := objectValue(root, "account")
	return codexCredentialDraft{
		sourceType:      "chatgpt-session",
		confidence:      82,
		label:           firstString(user, "email", "name"),
		accessToken:     accessToken,
		sessionToken:    firstString(root, "sessionToken", "session_token"),
		accountID:       firstString(account, "id", "account_id", "chatgpt_account_id"),
		accountIDSource: "session.account",
		email:           firstString(user, "email"),
		expiresAt:       normalizeAnyTimestamp(firstAny(root, "expires", "expiresAt", "expires_at")),
		planType:        firstString(account, "planType", "plan_type"),
		userID:          firstString(user, "id"),
		warnings:        []string{"ChatGPT Web session usually cannot refresh automatically after access_token expires"},
	}, true
}

func draftFromFlatObject(root map[string]any) (codexCredentialDraft, bool) {
	accessToken := firstString(root, "access_token", "accessToken")
	if accessToken == "" {
		return codexCredentialDraft{}, false
	}
	sourceType := "newapi"
	confidence := 90
	if strings.EqualFold(firstString(root, "type"), "codex") {
		sourceType = "cpa"
		confidence = 94
	} else if firstString(root, "expired", "expires_at", "expiresAt") != "" || firstString(root, "id_token", "idToken") != "" {
		sourceType = "cockpit"
		confidence = 92
	}
	return codexCredentialDraft{
		sourceType:      sourceType,
		confidence:      confidence,
		label:           firstString(root, "name", "label", "email"),
		accessToken:     accessToken,
		refreshToken:    cleanMissingPlaceholder(firstString(root, "refresh_token", "refreshToken")),
		idToken:         firstString(root, "id_token", "idToken"),
		sessionToken:    firstString(root, "session_token", "sessionToken"),
		accountID:       firstString(root, "account_id", "chatgpt_account_id", "chatgptAccountId"),
		accountIDSource: "flat",
		email:           firstString(root, "email", "email_address"),
		expiresAt:       normalizeAnyTimestamp(firstAny(root, "expired", "expires_at", "expiresAt", "expires")),
		lastRefresh:     firstString(root, "last_refresh", "lastRefresh"),
		planType:        firstString(root, "plan_type", "planType", "chatgpt_plan_type"),
		userID:          firstString(root, "user_id", "userId"),
	}, true
}

func finalizeCodexCredentialDraft(draft codexCredentialDraft) (CodexCredentialCandidate, bool) {
	draft.accessToken = strings.TrimSpace(draft.accessToken)
	if draft.accessToken == "" {
		return CodexCredentialCandidate{}, false
	}
	if strings.TrimSpace(draft.accountID) == "" {
		if accountID, ok := ExtractCodexAccountIDFromJWT(draft.accessToken); ok {
			draft.accountID = accountID
			draft.accountIDSource = "access_token.jwt"
			draft.warnings = append(draft.warnings, "account_id was inferred from access_token JWT claims")
		}
	}
	if strings.TrimSpace(draft.email) == "" {
		if email, ok := ExtractEmailFromJWT(draft.accessToken); ok {
			draft.email = email
		}
	}
	if strings.TrimSpace(draft.expiresAt) == "" {
		if expiresAt, ok := ExtractExpiresAtFromJWT(draft.accessToken); ok {
			draft.expiresAt = expiresAt
		}
	}
	draft.accountID = strings.TrimSpace(draft.accountID)
	if draft.accountID == "" {
		return CodexCredentialCandidate{}, false
	}
	if strings.TrimSpace(draft.sourceType) == "" {
		draft.sourceType = "unknown"
	}
	if strings.TrimSpace(draft.label) == "" {
		draft.label = defaultCodexCredentialLabel(draft.email, draft.accountID)
	}
	hasRefreshToken := strings.TrimSpace(draft.refreshToken) != ""
	if !hasRefreshToken {
		draft.warnings = append(draft.warnings, "refresh_token is missing; this credential cannot be refreshed automatically")
	}
	if expired, ok := isTimestampExpired(draft.expiresAt); ok && expired {
		draft.warnings = append(draft.warnings, "access_token appears to be expired")
	}
	key := normalizedCodexOAuthKey{
		IDToken:      strings.TrimSpace(draft.idToken),
		AccessToken:  draft.accessToken,
		RefreshToken: strings.TrimSpace(draft.refreshToken),
		SessionToken: strings.TrimSpace(draft.sessionToken),
		AccountID:    draft.accountID,
		LastRefresh:  strings.TrimSpace(draft.lastRefresh),
		Email:        strings.TrimSpace(draft.email),
		Type:         "codex",
		Expired:      strings.TrimSpace(draft.expiresAt),
		PlanType:     strings.TrimSpace(draft.planType),
		UserID:       strings.TrimSpace(draft.userID),
		SourceType:   strings.TrimSpace(draft.sourceType),
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		return CodexCredentialCandidate{}, false
	}
	return CodexCredentialCandidate{
		SourceType:      draft.sourceType,
		Confidence:      draft.confidence,
		Label:           draft.label,
		Key:             string(encoded),
		AccountID:       draft.accountID,
		AccountIDSource: draft.accountIDSource,
		Email:           strings.TrimSpace(draft.email),
		ExpiresAt:       strings.TrimSpace(draft.expiresAt),
		LastRefresh:     strings.TrimSpace(draft.lastRefresh),
		HasRefreshToken: hasRefreshToken,
		HasIDToken:      strings.TrimSpace(draft.idToken) != "",
		NonRefreshable:  !hasRefreshToken,
		Warnings:        uniqueStrings(draft.warnings),
	}, true
}

func ExtractExpiresAtFromJWT(token string) (string, bool) {
	claims, ok := decodeJWTClaims(token)
	if !ok {
		return "", false
	}
	return normalizeNumericTimestampClaim(claims["exp"])
}

func normalizeNumericTimestampClaim(value any) (string, bool) {
	switch v := value.(type) {
	case float64:
		return timestampFromFloat(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return "", false
		}
		return timestampFromFloat(f)
	case int64:
		return timestampFromFloat(float64(v))
	case int:
		return timestampFromFloat(float64(v))
	case string:
		return normalizeUnixTimestampString(v)
	default:
		return "", false
	}
}

func normalizeAnyTimestamp(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if out, ok := normalizeUnixTimestampString(s); ok {
			return out
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.Format(time.RFC3339)
		}
		return s
	}
	if out, ok := normalizeNumericTimestampClaim(value); ok {
		return out
	}
	return ""
}

func normalizeUnixTimestampString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return timestampFromInt(i), true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return timestampFromFloat(f)
	}
	return "", false
}

func timestampFromFloat(v float64) (string, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return "", false
	}
	return timestampFromInt(int64(v)), true
}

func timestampFromInt(v int64) string {
	if v > 100000000000 {
		v = v / 1000
	}
	return time.Unix(v, 0).UTC().Format(time.RFC3339)
}

func isTimestampExpired(value string) (bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, false
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return time.Now().After(t), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return time.Now().After(t), true
	}
	return false, false
}

func firstAny(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	if len(m) == 0 {
		return ""
	}
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		if s := anyToString(v); s != "" {
			return s
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func anyToString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func objectValue(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	return objectValueAny(v)
}

func objectValueAny(value any) (map[string]any, bool) {
	if obj, ok := value.(map[string]any); ok {
		return obj, true
	}
	return nil, false
}

func arrayValue(m map[string]any, key string) ([]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	if arr, ok := v.([]any); ok {
		return arr, true
	}
	return nil, false
}

func cleanMissingPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if value == "__missing_refresh_token__" || value == "missing_refresh_token" {
		return ""
	}
	return value
}

func defaultCodexCredentialLabel(email, accountID string) string {
	if strings.TrimSpace(email) != "" {
		return strings.TrimSpace(email)
	}
	if len(accountID) > 8 {
		return "account " + accountID[:8]
	}
	if accountID != "" {
		return "account " + accountID
	}
	return "codex credential"
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func hashCredentialKey(key string) string {
	if len(key) <= 24 {
		return key
	}
	return key[:12] + key[len(key)-12:]
}
