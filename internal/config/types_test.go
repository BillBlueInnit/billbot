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
	if cfg.Security.SandboxBackend != "docker" {
		t.Fatalf("sandbox backend = %q, want docker", cfg.Security.SandboxBackend)
	}
	if cfg.Security.SandboxDockerImage != "billbot-hermes:latest" {
		t.Fatalf("sandbox docker image = %q, want billbot-hermes:latest", cfg.Security.SandboxDockerImage)
	}
}

func TestNapCatEffectiveTokens(t *testing.T) {
	cfg := NapCatConfig{AccessToken: "legacy"}
	if cfg.EffectiveHTTPToken() != "legacy" || cfg.EffectiveWSToken() != "legacy" {
		t.Fatalf("legacy token fallback failed: %#v", cfg)
	}
	cfg.HTTPToken = "http-secret"
	cfg.WSToken = "ws-secret"
	if cfg.EffectiveHTTPToken() != "http-secret" || cfg.EffectiveWSToken() != "ws-secret" {
		t.Fatalf("split token preference failed: %#v", cfg)
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

func TestLoadAndSaveTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := []byte(`name = "toml-test"
owners = [12345]

[napcat]
http = "http://127.0.0.1:3000"
ws = "ws://127.0.0.1:3001"

[bridge]
enabled = true
respond_to_group_mentions_only = true
self_id = 67890

[models]
base_provider = "cheap"
base_model = "fast-model"
strong_provider = "strong"
strong_model = "reasoning-model"
`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load toml: %v", err)
	}
	if cfg.Name != "toml-test" || !cfg.Bridge.Enabled || cfg.Models.StrongModel != "reasoning-model" {
		t.Fatalf("loaded cfg = %#v", cfg)
	}
	cfg.Models.RoutingTimeoutSec = 12
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save toml: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved toml: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		`name = "toml-test"`,
		`routing_timeout_sec = 12`,
		`[napcat]`,
		`[models]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved toml missing %q:\n%s", want, text)
		}
	}
}
