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

func TestThemeSyncPreservesClassicOverride(t *testing.T) {
	preserveThemeState(t)

	GetThemeSettings().Frontend = "classic"
	UpdateAndSyncTheme()
	if got := common.GetTheme(); got != "classic" {
		t.Fatalf("expected persisted classic override to remain supported, got %q", got)
	}
}
