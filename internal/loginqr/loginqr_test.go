// SPDX-License-Identifier: LGPL-3.0-only

package loginqr

import (
	"context"
	"os"
	"testing"

	"billbot/internal/config"
)

func TestFetchRequiresCommand(t *testing.T) {
	_, err := Fetch(context.Background(), config.LoginConfig{})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestRenderFallsBackToDataURLWhenTerminalIsSmall(t *testing.T) {
	t.Setenv("COLUMNS", "10")
	t.Setenv("LINES", "5")
	render, dataURL, err := Render("https://example.test/login")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if render == "" || dataURL == "" {
		t.Fatalf("empty render/dataURL: render=%q dataURL=%q", render, dataURL)
	}
	if render != dataURL {
		t.Fatalf("small terminal should return data URL render")
	}
}

func TestFetchUsesConfiguredArgv(t *testing.T) {
	cfg := config.LoginConfig{
		QRCommand:    []string{os.Args[0], "-test.run=TestHelperQRCommand"},
		QRTimeoutSec: 5,
	}
	t.Setenv("BILLBOT_HELPER_QR_COMMAND", "1")
	out, err := Fetch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if out.Content != "https://example.test/qr" {
		t.Fatalf("content = %q", out.Content)
	}
	if out.Render == "" || out.DataURL == "" {
		t.Fatalf("missing render/dataURL: %#v", out)
	}
}

func TestStatusUsesConfiguredArgv(t *testing.T) {
	cfg := config.LoginConfig{
		StatusCommand: []string{os.Args[0], "-test.run=TestHelperStatusCommand"},
		QRTimeoutSec:  5,
	}
	t.Setenv("BILLBOT_HELPER_STATUS_COMMAND", "1")
	out, err := Status(context.Background(), cfg)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if out.Output != "online" {
		t.Fatalf("output = %q", out.Output)
	}
}

func TestHelperQRCommand(t *testing.T) {
	if os.Getenv("BILLBOT_HELPER_QR_COMMAND") != "1" {
		return
	}
	print("https://example.test/qr")
	os.Exit(0)
}

func TestHelperStatusCommand(t *testing.T) {
	if os.Getenv("BILLBOT_HELPER_STATUS_COMMAND") != "1" {
		return
	}
	print("online")
	os.Exit(0)
}
