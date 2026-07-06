package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	PlanType     string `json:"plan_type,omitempty"`
}

func TestNormalizeCodexCredentialFlat(t *testing.T) {
	raw := `{"type":"codex","access_token":"opaque-token","account_id":"acc-123","refresh_token":"refresh-123","email":"user@example.com"}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	require.Equal(t, "cpa", candidate.SourceType)
	require.True(t, candidate.HasRefreshToken)
	require.Equal(t, "acc-123", candidate.AccountID)
	require.Equal(t, "user@example.com", candidate.Email)

	key := decodeTestCodexOAuthKey(t, candidate.Key)
	require.Equal(t, "opaque-token", key.AccessToken)
	require.Equal(t, "refresh-123", key.RefreshToken)
	require.Equal(t, "acc-123", key.AccountID)
}

func TestNormalizeCodexCredentialSub2API(t *testing.T) {
	raw := `{
		"exported_at":"2026-07-06T00:00:00Z",
		"accounts":[{
			"name":"K12",
			"platform":"openai",
			"type":"oauth",
			"credentials":{
				"access_token":"opaque-token",
				"chatgpt_account_id":"acc-sub2api",
				"email":"sub@example.com",
				"expires_at":1785886305
			}
		}]
	}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	require.Equal(t, "sub2api", candidate.SourceType)
	require.Equal(t, "acc-sub2api", candidate.AccountID)
	require.False(t, candidate.HasRefreshToken)
	require.True(t, candidate.NonRefreshable)
	require.NotEmpty(t, candidate.ExpiresAt)
	require.Contains(t, strings.Join(candidate.Warnings, "\n"), "refresh_token is missing")
}

func TestNormalizeCodexCredential9Router(t *testing.T) {
	raw := `{
		"accessToken":"router-access",
		"refreshToken":"router-refresh",
		"expiresAt":"2026-07-06T12:00:00Z",
		"providerSpecificData":{
			"chatgptAccountId":"acc-router",
			"chatgptPlanType":"team"
		}
	}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "9router", result.Candidates[0].SourceType)
	require.Equal(t, "acc-router", result.Candidates[0].AccountID)

	key := decodeTestCodexOAuthKey(t, result.Candidates[0].Key)
	require.Equal(t, "router-refresh", key.RefreshToken)
	require.Equal(t, "team", key.PlanType)
}

func TestNormalizeCodexCredentialNativeAuthJSON(t *testing.T) {
	raw := `{
		"auth_mode":"chatgpt",
		"OPENAI_API_KEY":null,
		"last_refresh":"2026-07-06T10:00:00Z",
		"tokens":{
			"access_token":"native-access",
			"refresh_token":"native-refresh",
			"id_token":"native-id",
			"account_id":"acc-native"
		}
	}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "codex-auth", result.Candidates[0].SourceType)
	require.True(t, result.Candidates[0].HasIDToken)
	require.Equal(t, "acc-native", result.Candidates[0].AccountID)
}

func TestNormalizeCodexCredentialChatGPTSessionInfersAccountFromJWT(t *testing.T) {
	accessToken := unsignedJWT(map[string]any{
		"email": "jwt@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc-jwt",
		},
	})
	raw := `{"accessToken":"` + accessToken + `","sessionToken":"session-token","user":{"email":"session@example.com"}}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)

	candidate := result.Candidates[0]
	require.Equal(t, "chatgpt-session", candidate.SourceType)
	require.Equal(t, "acc-jwt", candidate.AccountID)
	require.Equal(t, "session@example.com", candidate.Email)
	require.NotEmpty(t, candidate.ExpiresAt)

	key := decodeTestCodexOAuthKey(t, candidate.Key)
	require.Equal(t, "session-token", key.SessionToken)
	require.Empty(t, key.RefreshToken)
}

func TestNormalizeCodexCredentialMultiAccount(t *testing.T) {
	raw := `{"accounts":[
		{"name":"one","platform":"openai","type":"oauth","credentials":{"access_token":"token-one","chatgpt_account_id":"acc-one"}},
		{"name":"two","platform":"openai","type":"oauth","credentials":{"access_token":"token-two","chatgpt_account_id":"acc-two","refresh_token":"refresh-two"}}
	]}`

	result, err := NormalizeCodexCredential(raw)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 2)
	require.Equal(t, 0, result.Candidates[0].Index)
	require.Equal(t, 1, result.Candidates[1].Index)
	require.ElementsMatch(t, []string{"acc-one", "acc-two"}, []string{result.Candidates[0].AccountID, result.Candidates[1].AccountID})
}

func TestNormalizeCodexCredentialRejectsMissingAccountID(t *testing.T) {
	_, err := NormalizeCodexCredential(`{"access_token":"opaque-token"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no supported Codex credential")
}

func unsignedJWT(claims map[string]any) string {
	header := map[string]any{"alg": "none", "typ": "JWT"}
	encode := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return encode(header) + "." + encode(claims) + "."
}

func decodeTestCodexOAuthKey(t *testing.T, raw string) testCodexOAuthKey {
	t.Helper()
	var key testCodexOAuthKey
	require.NoError(t, json.Unmarshal([]byte(raw), &key))
	return key
}
