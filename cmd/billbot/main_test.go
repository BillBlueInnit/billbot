// SPDX-License-Identifier: LGPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"testing"

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
	if err := setConfigValue(&cfg, "login.qr_command", "printf https://example.test/qr"); err != nil {
		t.Fatalf("set qr command: %v", err)
	}
	if len(cfg.Login.QRCommand) != 2 || cfg.Login.QRCommand[0] != "printf" {
		t.Fatalf("qr command = %#v", cfg.Login.QRCommand)
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
