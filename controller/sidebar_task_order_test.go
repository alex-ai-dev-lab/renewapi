package controller

import "testing"

func TestSidebarTaskSectionOrderValidation(t *testing.T) {
	for _, value := range []string{
		"",
		"access,usage,account,models,operations,system",
		"system,models,operations,access,usage,account",
		"access,usage",
		" access , account ",
		"access,access,usage", // Readers normalize duplicates without changing legacy order.
	} {
		t.Run("valid_"+value, func(t *testing.T) {
			if err := validateOptionValues(map[string]string{"SidebarTaskSectionOrder": value}); err != nil {
				t.Fatalf("valid task order %q rejected: %v", value, err)
			}
		})
	}
	for _, value := range []string{"chat", "general", "console", "admin", "access,unknown", "system;account", ",,,", "   "} {
		t.Run("invalid_"+value, func(t *testing.T) {
			if err := validateOptionValues(map[string]string{"SidebarTaskSectionOrder": value}); err == nil {
				t.Fatalf("invalid task order %q accepted", value)
			}
		})
	}
}

func TestSidebarTaskOrderDoesNotReplaceLegacyOrder(t *testing.T) {
	if err := validateOptionValues(map[string]string{
		"SidebarSectionOrder":     "admin,personal,console,chat",
		"SidebarTaskSectionOrder": "access,usage,account,models,operations,system",
	}); err != nil {
		t.Fatalf("independent old/new settings should coexist: %v", err)
	}
	if err := validateOptionValues(map[string]string{"SidebarSectionOrder": "access,usage"}); err == nil {
		t.Fatal("new display categories leaked into the legacy configuration protocol")
	}
}
