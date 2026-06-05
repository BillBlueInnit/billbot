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
	if err := setConfigValue(&cfg, "login.qr_command", "printf https://example.test/qr"); err != nil {
		t.Fatalf("set qr command: %v", err)
	}
	if len(cfg.Login.QRCommand) != 2 || cfg.Login.QRCommand[0] != "printf" {
		t.Fatalf("qr command = %#v", cfg.Login.QRCommand)
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
	if err := setConfigValue(&cfg, "autostart.enabled", "true"); err != nil {
		t.Fatalf("set autostart enabled: %v", err)
	}
	if !cfg.Autostart.Enabled {
		t.Fatal("autostart.enabled not set")
	}
	if err := setConfigValue(&cfg, "autostart.name", "BillBot Test"); err != nil {
		t.Fatalf("set autostart name: %v", err)
	}
	if cfg.Autostart.Name != "BillBot Test" {
		t.Fatalf("autostart name = %q", cfg.Autostart.Name)
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

func TestStartConfiguredBridgeHonorsEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Bridge.Enabled = false
	svc := bridge.NewService(cfg)
	if err := startConfiguredBridge(cfg, svc); err != nil {
		t.Fatalf("disabled bridge returned error: %v", err)
	}
}
