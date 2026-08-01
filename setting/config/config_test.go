package config

import (
	"testing"
)

type testConfigWithMap struct {
	Modes map[string]string `json:"modes"`
	Exprs map[string]string `json:"exprs"`
	Name  string            `json:"name"`
}

type testConfigWithBooleans struct {
	Enabled              bool   `json:"enabled"`
	ResponseProofEnabled bool   `json:"response_proof_enabled"`
	Name                 string `json:"name"`
}

func TestLoadFromDB_LoadsWholeJSONOption(t *testing.T) {
	manager := NewConfigManager()
	cfg := &testConfigWithBooleans{
		Enabled:              true,
		ResponseProofEnabled: true,
		Name:                 "default",
	}
	manager.Register("anti_poison_setting", cfg)

	err := manager.LoadFromDB(map[string]string{
		"anti_poison_setting": `{"enabled":true,"response_proof_enabled":false,"name":"json"}`,
	})
	if err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}

	if !cfg.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if cfg.ResponseProofEnabled {
		t.Fatalf("ResponseProofEnabled = true, want false from whole JSON option")
	}
	if cfg.Name != "json" {
		t.Fatalf("Name = %q, want json", cfg.Name)
	}
}

func TestLoadFromDB_DotOptionOverridesWholeJSONOption(t *testing.T) {
	manager := NewConfigManager()
	cfg := &testConfigWithBooleans{}
	manager.Register("anti_poison_setting", cfg)

	err := manager.LoadFromDB(map[string]string{
		"anti_poison_setting":                        `{"enabled":true,"response_proof_enabled":true,"name":"json"}`,
		"anti_poison_setting.response_proof_enabled": "false",
		"anti_poison_setting.name":                   "dot",
	})
	if err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}

	if !cfg.Enabled {
		t.Fatalf("Enabled = false, want true from whole JSON option")
	}
	if cfg.ResponseProofEnabled {
		t.Fatalf("ResponseProofEnabled = true, want false from dot override")
	}
	if cfg.Name != "dot" {
		t.Fatalf("Name = %q, want dot override", cfg.Name)
	}
}

func TestUpdateConfigFromMap_MapReplacement(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
			"model-b": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
			"model-b": "p * 10 + c * 50",
		},
		Name: "billing",
	}

	// Simulate removing model-a: new value only has model-b
	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{"model-b": "tiered_expr"}`,
		"exprs": `{"model-b": "p * 10 + c * 50"}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if _, ok := cfg.Modes["model-a"]; ok {
		t.Errorf("Modes still contains model-a after it was removed from the update; got %v", cfg.Modes)
	}
	if _, ok := cfg.Exprs["model-a"]; ok {
		t.Errorf("Exprs still contains model-a after it was removed from the update; got %v", cfg.Exprs)
	}

	if cfg.Modes["model-b"] != "tiered_expr" {
		t.Errorf("Modes[model-b] = %q, want %q", cfg.Modes["model-b"], "tiered_expr")
	}
	if cfg.Exprs["model-b"] != "p * 10 + c * 50" {
		t.Errorf("Exprs[model-b] = %q, want %q", cfg.Exprs["model-b"], "p * 10 + c * 50")
	}
}

func TestUpdateConfigFromMap_EmptyMapClearsAll(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
		},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{}`,
		"exprs": `{}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if len(cfg.Modes) != 0 {
		t.Errorf("Modes should be empty after updating with {}, got %v", cfg.Modes)
	}
	if len(cfg.Exprs) != 0 {
		t.Errorf("Exprs should be empty after updating with {}, got %v", cfg.Exprs)
	}
}

func TestUpdateConfigFromMap_ScalarFieldsUnchanged(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{"m": "v"},
		Name:  "old",
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"name": "new",
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if cfg.Name != "new" {
		t.Errorf("Name = %q, want %q", cfg.Name, "new")
	}
	// modes was not in configMap, should remain unchanged
	if cfg.Modes["m"] != "v" {
		t.Errorf("Modes should be unchanged, got %v", cfg.Modes)
	}
}

func TestUpdateConfigFromMap_InvalidFieldDoesNotPartiallyApply(t *testing.T) {
	cfg := &testConfigWithBooleans{
		Enabled: true,
		Name:    "old",
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"enabled": "not-a-bool",
		"name":    "new",
	})
	if err == nil {
		t.Fatal("UpdateConfigFromMap should reject an invalid boolean")
	}

	if !cfg.Enabled {
		t.Fatal("Enabled changed even though the update was rejected")
	}
	if cfg.Name != "old" {
		t.Fatalf("Name = %q, want old after rejected update", cfg.Name)
	}
}
