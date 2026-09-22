package common

import (
	"regexp"
	"strings"
)

var errorCredentialPattern = regexp.MustCompile(`(?i)(\b(?:authorization|api[-_]?key|access[-_]?token|refresh[-_]?token|password|cookie)\b["']?\s*[:=]\s*["']?)(?:Bearer\s+)?[^"'\s,;&]+`)
var errorAPIKeyPattern = regexp.MustCompile(`\bsk-[a-zA-Z0-9_-]{8,}`)

// RedactErrorCredentials 保留管理员定位故障所需的 hostname，同时去除凭证。
func RedactErrorCredentials(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "***")
		}
	}
	message = errorCredentialPattern.ReplaceAllString(message, "${1}***")
	return errorAPIKeyPattern.ReplaceAllString(message, "***")
}
