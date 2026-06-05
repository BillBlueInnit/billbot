// SPDX-License-Identifier: LGPL-3.0-only

package process

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"billbot/internal/config"
)

type Manager struct {
	mu        sync.Mutex
	processes map[string]*exec.Cmd
	client    http.Client
}

func NewManager() *Manager {
	return &Manager{
		processes: map[string]*exec.Cmd{},
		client:    http.Client{Timeout: 2 * time.Second},
	}
}

func (m *Manager) StartNapCat(ctx context.Context, cfg config.ManagedProcessConfig) error {
	if cfg.WaitHTTP != "" && m.httpReady(ctx, cfg.WaitHTTP) {
		return nil
	}
	if !cfg.AutoStart {
		return nil
	}
	if cfg.Command == "" {
		return errors.New("napcat auto_start is enabled but command is empty")
	}

	m.mu.Lock()
	if cmd, ok := m.processes["napcat"]; ok && cmd.Process != nil {
		m.mu.Unlock()
		return m.waitHTTP(ctx, cfg)
	}
	cmd := exec.CommandContext(context.Background(), cfg.Command, cfg.Args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start napcat: %w", err)
	}
	m.processes["napcat"] = cmd
	m.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.processes["napcat"] == cmd {
			delete(m.processes, "napcat")
		}
		m.mu.Unlock()
	}()

	return m.waitHTTP(ctx, cfg)
}

func (m *Manager) StopNapCat() error {
	m.mu.Lock()
	cmd := m.processes["napcat"]
	delete(m.processes, "napcat")
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func (m *Manager) waitHTTP(parent context.Context, cfg config.ManagedProcessConfig) error {
	if cfg.WaitHTTP == "" {
		return nil
	}
	timeout := time.Duration(cfg.WaitTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if m.httpReady(ctx, cfg.WaitHTTP) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("napcat did not become ready at %s within %s", cfg.WaitHTTP, timeout)
		case <-ticker.C:
		}
	}
}

func (m *Manager) httpReady(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}
