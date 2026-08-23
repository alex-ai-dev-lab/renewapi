package system_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestThemeDefaultsToRedesignedFrontend(t *testing.T) {
	originalSettings := *GetThemeSettings()
	originalCommonTheme := common.GetTheme()
	t.Cleanup(func() {
		*GetThemeSettings() = originalSettings
		common.SetTheme(originalCommonTheme)
	})

	if got := GetThemeSettings().Frontend; got != "default" {
		t.Fatalf("expected authoritative frontend default to be default, got %q", got)
	}

	UpdateAndSyncTheme()
	if got := common.GetTheme(); got != "default" {
		t.Fatalf("expected common frontend theme to sync to default, got %q", got)
	}
}
