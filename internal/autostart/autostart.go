// SPDX-License-Identifier: LGPL-3.0-only

package autostart

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"billbot/internal/config"
)

type Manager struct {
	GOOS    string
	HomeDir string
	RunCmd  func(context.Context, string, ...string) error
}

type Status struct {
	Supported   bool   `json:"supported"`
	Platform    string `json:"platform"`
	Enabled     bool   `json:"enabled"`
	Name        string `json:"name"`
	Target      string `json:"target,omitempty"`
	Message     string `json:"message,omitempty"`
	ServicePath string `json:"service_path,omitempty"`
}

type Options struct {
	ExePath    string
	ConfigPath string
	Port       int
	CLI        bool
}

func NewManager() Manager {
	return Manager{
		GOOS:    runtime.GOOS,
		HomeDir: homeDir(),
		RunCmd: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
	}
}

func (m Manager) Status(cfg config.Config, opts Options) Status {
	name := autostartName(cfg)
	target := m.target(opts)
	out := Status{Platform: m.goos(), Name: name, Target: target}
	switch m.goos() {
	case "windows", "linux":
		out.Supported = true
		out.Enabled = cfg.Autostart.Enabled
	default:
		out.Message = "autostart is not implemented for this platform"
	}
	if m.goos() == "linux" {
		out.ServicePath = m.servicePath(name)
	}
	return out
}

func (m Manager) Enable(ctx context.Context, cfg config.Config, opts Options) (Status, error) {
	status := m.Status(cfg, opts)
	if !status.Supported {
		return status, fmt.Errorf("autostart is not supported on %s", status.Platform)
	}
	if strings.TrimSpace(status.Target) == "" {
		return status, fmt.Errorf("executable path is empty")
	}
	switch m.goos() {
	case "windows":
		err := m.run(ctx, "reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", status.Name, "/t", "REG_SZ", "/d", windowsCommand(status.Target, opts), "/f")
		if err != nil {
			return status, err
		}
	case "linux":
		service := systemdService(status.Name, status.Target, opts)
		if err := os.MkdirAll(filepath.Dir(status.ServicePath), 0755); err != nil {
			return status, err
		}
		if err := os.WriteFile(status.ServicePath, []byte(service), 0600); err != nil {
			return status, err
		}
		if err := m.run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			return status, err
		}
		if err := m.run(ctx, "systemctl", "--user", "enable", "--now", serviceFileName(status.Name)); err != nil {
			return status, err
		}
	}
	status.Enabled = true
	status.Message = "autostart enabled"
	return status, nil
}

func (m Manager) Disable(ctx context.Context, cfg config.Config, opts Options) (Status, error) {
	status := m.Status(cfg, opts)
	if !status.Supported {
		return status, fmt.Errorf("autostart is not supported on %s", status.Platform)
	}
	switch m.goos() {
	case "windows":
		if err := m.run(ctx, "reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", status.Name, "/f"); err != nil {
			return status, err
		}
	case "linux":
		_ = m.run(ctx, "systemctl", "--user", "disable", "--now", serviceFileName(status.Name))
		_ = os.Remove(status.ServicePath)
		_ = m.run(ctx, "systemctl", "--user", "daemon-reload")
	}
	status.Enabled = false
	status.Message = "autostart disabled"
	return status, nil
}

func (m Manager) target(opts Options) string {
	if opts.ExePath != "" {
		return opts.ExePath
	}
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return ""
}

func (m Manager) goos() string {
	if m.GOOS != "" {
		return m.GOOS
	}
	return runtime.GOOS
}

func (m Manager) servicePath(name string) string {
	base := m.HomeDir
	if base == "" {
		base = homeDir()
	}
	return filepath.Join(base, ".config", "systemd", "user", serviceFileName(name))
}

func (m Manager) run(ctx context.Context, name string, args ...string) error {
	if m.RunCmd != nil {
		return m.RunCmd(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...).Run()
}

func autostartName(cfg config.Config) string {
	name := strings.TrimSpace(cfg.Autostart.Name)
	if name == "" {
		return "BillBot"
	}
	return name
}

func serviceFileName(name string) string {
	safe := strings.ToLower(strings.TrimSpace(name))
	safe = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, safe)
	if safe == "" {
		safe = "billbot"
	}
	return safe + ".service"
}

func windowsCommand(exe string, opts Options) string {
	return quoteWindows(exe) + commandArgs(opts, quoteWindows)
}

func systemdService(name string, exe string, opts Options) string {
	return "[Unit]\n" +
		"Description=" + name + "\n" +
		"After=network-online.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"ExecStart=" + quoteSystemd(exe) + commandArgs(opts, shellQuote) + "\n" +
		"Restart=on-failure\n" +
		"RestartSec=5\n\n" +
		"[Install]\n" +
		"WantedBy=default.target\n"
}

func commandArgs(opts Options, quote func(string) string) string {
	var args []string
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	if opts.Port > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", opts.Port))
	}
	if opts.CLI {
		args = append(args, "--cli")
	}
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(quote(arg))
	}
	return b.String()
}

func quoteWindows(v string) string {
	return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}

func quoteSystemd(v string) string {
	return shellQuote(v)
}

func shellQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}
