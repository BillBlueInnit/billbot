// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"billbot/internal/config"
)

type Runner struct {
	Command string
}

func NewRunner(command string) Runner {
	if command == "" {
		command = "hermes"
	}
	return Runner{Command: command}
}

func (r Runner) Ask(ctx context.Context, prompt string) (string, error) {
	return r.AskWithOptions(ctx, prompt, Options{})
}

type Options struct {
	Model                 string
	Provider              string
	SessionID             string
	DisableToolsInSandbox bool
	SecurityMode          string
}

func OptionsFromConfig(cfg config.Config) Options {
	model := cfg.Models.DefaultModel
	return Options{
		Model:                 model,
		Provider:              cfg.Models.DefaultProvider,
		DisableToolsInSandbox: cfg.Hermes.DisableToolsInSandbox,
		SecurityMode:          cfg.Security.Mode,
	}
}

func (r Runner) AskWithOptions(ctx context.Context, prompt string, opts Options) (string, error) {
	text, _, err := r.AskWithSession(ctx, prompt, opts)
	return text, err
}

func (r Runner) AskWithSession(ctx context.Context, prompt string, opts Options) (string, string, error) {
	args := BuildArgs(prompt, opts)
	cmd := exec.CommandContext(ctx, r.Command, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = nil
	}
	return runWithIdleOutput(ctx, cmd, 2*time.Second)
}

func BuildArgs(prompt string, opts Options) []string {
	args := []string{"chat", "-Q", "-q", prompt}
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}
	if opts.Provider != "" {
		args = append(args, "--provider", opts.Provider)
	}
	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}
	if opts.SecurityMode == "sandbox" && opts.DisableToolsInSandbox {
		args = append(args, "-t", "")
	}
	return args
}

func runWithIdleOutput(ctx context.Context, cmd *exec.Cmd, idleTimeout time.Duration) (string, string, error) {
	var mu sync.Mutex
	var out bytes.Buffer
	touch := make(chan struct{}, 1)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}
	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	copyOutput := func(r io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				mu.Lock()
				_, _ = out.Write(buf[:n])
				mu.Unlock()
				select {
				case touch <- struct{}{}:
				default:
				}
			}
			if readErr != nil {
				return
			}
		}
	}
	go copyOutput(stdout)
	go copyOutput(stderr)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()
	seenOutput := false

	for {
		select {
		case err := <-done:
			text, sessionID := cleanOutput(outputString(&mu, &out))
			return text, sessionID, err
		case <-touch:
			seenOutput = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idleTimeout)
		case <-timer.C:
			if seenOutput {
				killProcess(cmd)
				err := <-done
				if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
					err = nil
				}
				text, sessionID := cleanOutput(outputString(&mu, &out))
				if text != "" {
					return text, sessionID, nil
				}
				return text, sessionID, err
			}
			timer.Reset(idleTimeout)
		case <-ctx.Done():
			killProcess(cmd)
			_ = <-done
			text, sessionID := cleanOutput(outputString(&mu, &out))
			if text != "" {
				return text, sessionID, nil
			}
			return "", "", ctx.Err()
		}
	}
}

func killProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func outputString(mu *sync.Mutex, out *bytes.Buffer) string {
	mu.Lock()
	defer mu.Unlock()
	return out.String()
}

func cleanOutput(text string) (string, string) {
	var lines []string
	var sessionID string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "session_id:") {
			sessionID = strings.TrimSpace(strings.TrimPrefix(trimmed, "session_id:"))
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), sessionID
}
