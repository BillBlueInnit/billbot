// SPDX-License-Identifier: LGPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeFillsV01Defaults(t *testing.T) {
	var cfg Config
	cfg.Normalize()

	if cfg.Server.Port != 2006 {
		t.Fatalf("server port = %d, want 2006", cfg.Server.Port)
	}
	if cfg.Connector.Mode != "external" || cfg.Connector.Name != "napcat" {
		t.Fatalf("connector defaults = %#v", cfg.Connector)
	}
	if cfg.NapCat.HTTP == "" || cfg.NapCat.WS == "" {
		t.Fatalf("napcat endpoints were not defaulted: %#v", cfg.NapCat)
	}
	if cfg.Hermes.Command != "hermes" {
		t.Fatalf("hermes command = %q, want hermes", cfg.Hermes.Command)
	}
	if cfg.Security.Mode != "sandbox" {
		t.Fatalf("security mode = %q, want sandbox", cfg.Security.Mode)
	}
	if cfg.Autostart.Name != "BillBot" {
		t.Fatalf("autostart name = %q, want BillBot", cfg.Autostart.Name)
	}
}

func TestSavePreservesUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte(`name: old
future:
  enabled: true
runtime:
  data_dir: custom-data
  future_runtime_field: keep-me
`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Name = "new"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"name: new",
		"future:",
		"enabled: true",
		"future_runtime_field: keep-me",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved config missing %q:\n%s", want, text)
		}
	}
}
