package common

import (
	"net/url"
	"regexp"
	"strings"
)

var errorCredentialPattern = regexp.MustCompile(`(?i)(\b(?:authorization|key|api[-_]?key|access[-_]?token|refresh[-_]?token|password|cookie)\b["']?\s*[:=]\s*["']?)(?:Bearer\s+)?[^"'\s,;&]+`)
var errorAPIKeyPattern = regexp.MustCompile(`\bsk-[a-zA-Z0-9_-]{8,}`)
var errorURLPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)

// SafeLogURL retains the endpoint while excluding credentials and arbitrary
// query values. It never changes the URL used by the actual HTTP request.
func SafeLogURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid URL>"
	}
	u.User, u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = nil, "", "", "", false
	return u.String()
}

// RedactErrorCredentials 保留管理员定位故障所需的 hostname，同时去除凭证。
func RedactErrorCredentials(message string, secrets ...string) string {
	message = errorURLPattern.ReplaceAllStringFunc(message, SafeLogURL)
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "***")
			message = strings.ReplaceAll(message, url.QueryEscape(secret), "***")
			message = strings.ReplaceAll(message, url.PathEscape(secret), "***")
		}
	}
	message = errorCredentialPattern.ReplaceAllString(message, "${1}***")
	return errorAPIKeyPattern.ReplaceAllString(message, "***")
}
