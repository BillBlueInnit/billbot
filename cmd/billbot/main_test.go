// SPDX-License-Identifier: LGPL-3.0-only

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if err := setConfigValue(&cfg, "hermes.require_persistent", "false"); err != nil {
		t.Fatalf("set require persistent: %v", err)
	}
	if cfg.Hermes.RequirePersistent {
		t.Fatal("hermes.require_persistent not set")
	}
	if err := setConfigValue(&cfg, "hermes.profile_dir", "D:\\billbot\\hermes-profile"); err != nil {
		t.Fatalf("set hermes profile dir: %v", err)
	}
	if cfg.Hermes.ProfileDir != "D:\\billbot\\hermes-profile" {
		t.Fatalf("hermes.profile_dir = %q", cfg.Hermes.ProfileDir)
	}
	if err := setConfigValue(&cfg, "hermes.reset_profile_on_start", "false"); err != nil {
		t.Fatalf("set reset profile: %v", err)
	}
	if cfg.Hermes.ResetProfileOnStart {
		t.Fatal("hermes.reset_profile_on_start not set")
	}
	if err := setConfigValue(&cfg, "security.sandbox_backend", "command"); err != nil {
		t.Fatalf("set sandbox backend: %v", err)
	}
	if err := setConfigValue(&cfg, "security.sandbox_command", "vm-run --name billbot --"); err != nil {
		t.Fatalf("set sandbox command: %v", err)
	}
	if cfg.Security.SandboxBackend != "command" || len(cfg.Security.SandboxCommand) != 4 || cfg.Security.SandboxCommand[0] != "vm-run" {
		t.Fatalf("sandbox config = backend:%q command:%#v", cfg.Security.SandboxBackend, cfg.Security.SandboxCommand)
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
	key, value, ok = simpleSet("管理员 10001")
	if !ok || key != "管理员" || value != "10001" {
		t.Fatalf("simple set chinese admin = key=%q value=%q ok=%t", key, value, ok)
	}
}

func TestCLIChineseHelpAndUnknownCommand(t *testing.T) {
	cfg := config.Default()
	state := &cliState{cfg: cfg, bridgeSvc: bridge.NewService(cfg)}

	out, _, _ := state.Execute(context.Background(), "帮助")
	if !strings.Contains(out, "BillBot CLI 指令") || strings.Contains(out, "Commands:") {
		t.Fatalf("unexpected help text: %s", out)
	}

	out, _, _ = state.Execute(context.Background(), "不存在")
	if !strings.Contains(out, "未知指令") {
		t.Fatalf("unexpected unknown command text: %s", out)
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
