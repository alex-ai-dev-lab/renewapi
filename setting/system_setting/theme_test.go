package system_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func preserveThemeState(t *testing.T) {
	t.Helper()
	originalSettings := *GetThemeSettings()
	originalCommonTheme := common.GetTheme()
	t.Cleanup(func() {
		*GetThemeSettings() = originalSettings
		common.SetTheme(originalCommonTheme)
	})
}

func TestThemeDefaultsToRedesignedFrontend(t *testing.T) {
	preserveThemeState(t)

	if got := GetThemeSettings().Frontend; got != "default" {
		t.Fatalf("expected authoritative frontend default to be default, got %q", got)
	}

	UpdateAndSyncTheme()
	if got := common.GetTheme(); got != "default" {
		t.Fatalf("expected common frontend theme to sync to default, got %q", got)
	}
}

func TestRetiredClassicSettingUsesDefaultFrontend(t *testing.T) {
	preserveThemeState(t)

	common.SetTheme("classic")
	if common.GetTheme() != "default" {
		t.Fatal("retired theme bypassed the default frontend")
	}
	GetThemeSettings().Frontend = "classic"
	UpdateAndSyncTheme()
	if got := common.GetTheme(); got != "default" {
		t.Fatalf("expected retired classic setting to resolve to the available frontend, got %q", got)
	}
}
