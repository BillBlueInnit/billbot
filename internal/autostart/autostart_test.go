// SPDX-License-Identifier: LGPL-3.0-only

package autostart

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"billbot/internal/config"
)

func TestWindowsEnableUsesRegistryRunKey(t *testing.T) {
	var calls [][]string
	m := Manager{
		GOOS: "windows",
		RunCmd: func(ctx context.Context, name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	}
	cfg := config.Default()
	cfg.Autostart.Name = "BillBot Test"

	status, err := m.Enable(context.Background(), cfg, Options{
		ExePath:    `C:\Program Files\BillBot\billbot.exe`,
		ConfigPath: `C:\Users\me\billbot.yaml`,
		Port:       2006,
	})
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("status = %#v", status)
	}
	if len(calls) != 1 || calls[0][0] != "reg" || !slices.Contains(calls[0], "add") {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	joined := strings.Join(calls[0], " ")
	for _, want := range []string{
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"BillBot Test",
		`"C:\Program Files\BillBot\billbot.exe"`,
		`"C:\Users\me\billbot.yaml"`,
		"--port",
		"2006",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("registry command missing %q: %#v", want, calls)
		}
	}
}

func TestLinuxEnableWritesSystemdUserService(t *testing.T) {
	var calls [][]string
	home := t.TempDir()
	m := Manager{
		GOOS:    "linux",
		HomeDir: home,
		RunCmd: func(ctx context.Context, name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	}
	cfg := config.Default()
	cfg.Autostart.Name = "BillBot Test"

	status, err := m.Enable(context.Background(), cfg, Options{
		ExePath:    "/opt/billbot/billbot",
		ConfigPath: "/etc/billbot/config.yaml",
		Port:       2006,
	})
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("status = %#v", status)
	}
	servicePath := filepath.Join(home, ".config", "systemd", "user", "billbot-test.service")
	body, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"Description=BillBot Test",
		"ExecStart='/opt/billbot/billbot' '--config' '/etc/billbot/config.yaml' '--port' '2006'",
		"Restart=on-failure",
		"WantedBy=default.target",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("service missing %q:\n%s", want, text)
		}
	}
	if len(calls) != 2 || !slices.Contains(calls[1], "enable") || !slices.Contains(calls[1], "--now") {
		t.Fatalf("unexpected systemctl calls: %#v", calls)
	}
}

func TestUnsupportedPlatformStatus(t *testing.T) {
	status := Manager{GOOS: "darwin"}.Status(config.Default(), Options{ExePath: "/tmp/billbot"})
	if status.Supported {
		t.Fatalf("darwin should not be supported yet: %#v", status)
	}
	if status.Message == "" {
		t.Fatalf("status missing message: %#v", status)
	}
}
