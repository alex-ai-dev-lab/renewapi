package system_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ThemeSettings struct {
	Frontend              string `json:"frontend"`
	CustomizationPreset   string `json:"customization_preset"`
	CustomizationFont     string `json:"customization_font"`
	CustomizationRadius   string `json:"customization_radius"`
	CustomizationScale    string `json:"customization_scale"`
	ContentLayout         string `json:"content_layout"`
	CustomAccentEnabled   bool   `json:"custom_accent_enabled"`
	CustomAccentColor     string `json:"custom_accent_color"`
	CustomPaletteEnabled  bool   `json:"custom_palette_enabled"`
	CustomBackgroundColor string `json:"custom_background_color"`
	CustomSurfaceColor    string `json:"custom_surface_color"`
	CustomSidebarColor    string `json:"custom_sidebar_color"`
	CustomChartColor      string `json:"custom_chart_color"`
}

var themeSettings = ThemeSettings{
	Frontend:              "default",
	CustomizationPreset:   "default",
	CustomizationFont:     "default",
	CustomizationRadius:   "default",
	CustomizationScale:    "default",
	ContentLayout:         "full",
	CustomAccentEnabled:   false,
	CustomAccentColor:     "#2563eb",
	CustomPaletteEnabled:  false,
	CustomBackgroundColor: "#ffffff",
	CustomSurfaceColor:    "#f8fafc",
	CustomSidebarColor:    "#f8fafc",
	CustomChartColor:      "#14b8a6",
}

func init() {
	config.GlobalConfig.Register("theme", &themeSettings)
	syncThemeToCommon()
}

func syncThemeToCommon() {
	common.SetTheme(themeSettings.Frontend)
}

func GetThemeSettings() *ThemeSettings {
	return &themeSettings
}

// UpdateAndSyncTheme syncs the theme config to common after DB load.
func UpdateAndSyncTheme() {
	syncThemeToCommon()
}
