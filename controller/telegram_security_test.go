package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func signTelegramParams(params url.Values, token string) {
	data := make([]string, 0, len(params))
	for key, values := range params {
		if key != "hash" {
			data = append(data, key+"="+values[0])
		}
	}
	sort.Strings(data)
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(strings.Join(data, "\n")))
	params.Set("hash", hex.EncodeToString(mac.Sum(nil)))
}

func cloneTelegramParams(params url.Values) url.Values {
	cloned := make(url.Values, len(params))
	for key, values := range params {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func TestVerifyTelegramAuthorization(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token := "telegram-secret"
	valid := url.Values{"id": {"42"}, "auth_date": {"1800000000"}, "first_name": {"Ada"}}
	signTelegramParams(valid, token)
	id, err := verifyTelegramAuthorization(valid, token, now)
	require.NoError(t, err)
	require.Equal(t, "42", id)

	for name, mutate := range map[string]func(url.Values){
		"expired":  func(v url.Values) { v.Set("auth_date", "1799999600"); signTelegramParams(v, token) },
		"future":   func(v url.Values) { v.Set("auth_date", "1800000121"); signTelegramParams(v, token) },
		"bad hash": func(v url.Values) { v.Set("hash", "not-hex") },
		"duplicate": func(v url.Values) {
			v["id"] = []string{"42", "43"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneTelegramParams(valid)
			mutate(candidate)
			_, err := verifyTelegramAuthorization(candidate, token, now)
			require.Error(t, err)
		})
	}
}
