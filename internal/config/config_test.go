package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML_StringIcon(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	content := `
provider: openai
display: remaining
interval: 10m
icon: "󴃐"
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := LoadYAML(configFile, &cfg); err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	if cfg.GetIcon("openai") != "󴃐" {
		t.Errorf("expected 󴃐, got %s", cfg.GetIcon("openai"))
	}
	if cfg.GetIcon("agy") != "󴃐" {
		t.Errorf("expected 󴃐, got %s", cfg.GetIcon("agy"))
	}
}

func TestLoadYAML_MapIcon(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	content := `
provider: agy
display: remaining
interval: 15m
icon:
  openai: "󰚩"
  agy: "󰫣"
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := LoadYAML(configFile, &cfg); err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	if cfg.GetIcon("openai") != "󰚩" {
		t.Errorf("expected 󰚩, got %s", cfg.GetIcon("openai"))
	}
	if cfg.GetIcon("agy") != "󰫣" {
		t.Errorf("expected 󰫣, got %s", cfg.GetIcon("agy"))
	}
	if cfg.GetIcon("antigravity") != "󰫣" {
		t.Errorf("expected 󰫣 for antigravity alias, got %s", cfg.GetIcon("antigravity"))
	}
}
