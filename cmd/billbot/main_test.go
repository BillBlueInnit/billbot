// SPDX-License-Identifier: LGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"testing"

	"billbot/internal/bridge"
	"billbot/internal/config"
)

func TestSetConfigValue(t *testing.T) {
	cfg := config.Default()
	if err := setConfigValue(&cfg, "qq", "123456"); err != nil {
		t.Fatalf("set qq alias: %v", err)
	}
	if cfg.Bridge.SelfID != 123456 {
		t.Fatalf("self id = %d", cfg.Bridge.SelfID)
	}
	if err := setConfigValue(&cfg, "token", "secret"); err != nil {
		t.Fatalf("set token alias: %v", err)
	}
	if cfg.NapCat.AccessToken != "secret" || cfg.NapCat.HTTPToken != "secret" || cfg.NapCat.WSToken != "secret" {
		t.Fatalf("tokens = access:%q http:%q ws:%q", cfg.NapCat.AccessToken, cfg.NapCat.HTTPToken, cfg.NapCat.WSToken)
	}
	if err := setConfigValue(&cfg, "http_token", "http-secret"); err != nil {
		t.Fatalf("set http token alias: %v", err)
	}
	if err := setConfigValue(&cfg, "ws_token", "ws-secret"); err != nil {
		t.Fatalf("set ws token alias: %v", err)
	}
	if cfg.NapCat.HTTPToken != "http-secret" || cfg.NapCat.WSToken != "ws-secret" {
		t.Fatalf("split tokens = http:%q ws:%q", cfg.NapCat.HTTPToken, cfg.NapCat.WSToken)
	}
	if err := setConfigValue(&cfg, "admin", "10001,10002"); err != nil {
		t.Fatalf("set admin alias: %v", err)
	}
	if len(cfg.Owners) != 2 || cfg.Owners[0] != 10001 || cfg.Owners[1] != 10002 {
		t.Fatalf("owners = %#v", cfg.Owners)
	}
	if err := setConfigValue(&cfg, "models.strong_model", "strong"); err != nil {
		t.Fatalf("set strong model: %v", err)
	}
	if cfg.Models.StrongModel != "strong" {
		t.Fatalf("strong model = %q", cfg.Models.StrongModel)
	}
	if err := setConfigValue(&cfg, "processes.napcat.auto_start", "true"); err != nil {
		t.Fatalf("set auto start: %v", err)
	}
	if !cfg.Processes.NapCat.AutoStart {
		t.Fatal("auto_start not set")
	}
	if err := setConfigValue(&cfg, "bridge.enabled", "true"); err != nil {
		t.Fatalf("set bridge enabled: %v", err)
	}
	if !cfg.Bridge.Enabled {
		t.Fatal("bridge.enabled not set")
	}
	if err := setConfigValue(&cfg, "models.routing_timeout_sec", "12"); err != nil {
		t.Fatalf("set routing timeout: %v", err)
	}
	if cfg.Models.RoutingTimeoutSec != 12 {
		t.Fatalf("routing timeout = %d", cfg.Models.RoutingTimeoutSec)
	}
	if err := setConfigValue(&cfg, "runtime.progress_interval_sec", "7"); err != nil {
		t.Fatalf("set progress interval: %v", err)
	}
	if cfg.Runtime.ProgressIntervalSec != 7 {
		t.Fatalf("progress interval = %d", cfg.Runtime.ProgressIntervalSec)
	}
	if err := setConfigValue(&cfg, "models.special_model", "special"); err != nil {
		t.Fatalf("set special model: %v", err)
	}
	if cfg.Models.SpecialModel != "special" {
		t.Fatalf("special model = %q", cfg.Models.SpecialModel)
	}
	if err := setConfigValue(&cfg, "models.heavy_timeout_sec", "0"); err == nil {
		t.Fatal("expected invalid positive integer error")
	}
	if err := setConfigValue(&cfg, "models.strong_model", `""`); err != nil {
		t.Fatalf("clear strong model: %v", err)
	}
	if cfg.Models.StrongModel != "" {
		t.Fatalf("strong model was not cleared: %q", cfg.Models.StrongModel)
	}
}

func TestReadLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "billbot.log")
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	text, err := readLogTail(path, 6)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if text != "econd\n" {
		t.Fatalf("tail = %q", text)
	}
}

func TestNormalizeCLIInput(t *testing.T) {
	got := normalizeCLIInput("\ufeffs\x00t\x00a\x00t\x00u\x00s\x00\r\n")
	if got != "status" {
		t.Fatalf("normalized input = %q", got)
	}
}

func TestSimpleSet(t *testing.T) {
	key, value, ok := simpleSet("token abc def")
	if !ok || key != "token" || value != "abc def" {
		t.Fatalf("simple set token = key=%q value=%q ok=%t", key, value, ok)
	}
	key, value, ok = simpleSet("qq 123456")
	if !ok || key != "qq" || value != "123456" {
		t.Fatalf("simple set qq = key=%q value=%q ok=%t", key, value, ok)
	}
	key, value, ok = simpleSet("ws_token ws-secret")
	if !ok || key != "ws_token" || value != "ws-secret" {
		t.Fatalf("simple set ws token = key=%q value=%q ok=%t", key, value, ok)
	}
	key, value, ok = simpleSet("admin 10001")
	if !ok || key != "admin" || value != "10001" {
		t.Fatalf("simple set admin = key=%q value=%q ok=%t", key, value, ok)
	}
}

func TestStartConfiguredBridgeHonorsEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Bridge.Enabled = false
	svc := bridge.NewService(cfg)
	if err := startConfiguredBridge(cfg, svc); err != nil {
		t.Fatalf("disabled bridge returned error: %v", err)
	}
}
