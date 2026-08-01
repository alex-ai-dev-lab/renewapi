package system_setting

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PasskeySettings struct {
	Enabled              bool   `json:"enabled"`
	RPDisplayName        string `json:"rp_display_name"`
	RPID                 string `json:"rp_id"`
	Origins              string `json:"origins"`
	AllowInsecureOrigin  bool   `json:"allow_insecure_origin"`
	UserVerification     string `json:"user_verification"`
	AttachmentPreference string `json:"attachment_preference"`
}

var defaultPasskeySettings = PasskeySettings{
	Enabled:              false,
	RPDisplayName:        common.SystemName,
	RPID:                 "",
	Origins:              "",
	AllowInsecureOrigin:  false,
	UserVerification:     "preferred",
	AttachmentPreference: "",
}

func init() {
	config.GlobalConfig.Register("passkey", &defaultPasskeySettings)
}

func GetPasskeySettings() *PasskeySettings {
	applyPasskeyDefaults(&defaultPasskeySettings)
	return &defaultPasskeySettings
}

// GetPasskeySettingsSnapshot returns the effective settings without mutating
// the registered config. Validation paths use this form so merely checking a
// proposed option cannot silently persist derived RP ID/origin values.
func GetPasskeySettingsSnapshot() PasskeySettings {
	settings := defaultPasskeySettings
	applyPasskeyDefaults(&settings)
	return settings
}

func applyPasskeyDefaults(settings *PasskeySettings) {
	if settings.RPID == "" && ServerAddress != "" {
		// 从ServerAddress提取域名作为RPID
		// ServerAddress可能是 "https://newapi.pro" 这种格式
		serverAddr := strings.TrimSpace(ServerAddress)
		if parsed, err := url.Parse(serverAddr); err == nil && parsed.Host != "" {
			settings.RPID = strings.ToLower(parsed.Hostname())
		} else {
			settings.RPID = serverAddr
		}
	}
	if settings.Origins == "" || settings.Origins == "[]" {
		serverAddr := strings.TrimSpace(ServerAddress)
		if parsed, err := url.Parse(serverAddr); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			settings.Origins = parsed.Scheme + "://" + parsed.Host
		} else {
			settings.Origins = strings.TrimRight(serverAddr, "/")
		}
	}
}
