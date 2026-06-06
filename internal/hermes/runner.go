// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	Persistent            bool
	RequirePersistent     bool
	DisableToolsInSandbox bool
	SecurityMode          string
	SandboxDir            string
	SandboxBackend        string
	SandboxCommand        []string
	SandboxDockerImage    string
	SandboxDockerArgs     []string
	ProfileDir            string
	Attachments           []Attachment
}

type Attachment struct {
	Type string
	URL  string
	File string
	Name string
}

func OptionsFromConfig(cfg config.Config) Options {
	model := cfg.Models.DefaultModel
	return Options{
		Model:                 model,
		Provider:              cfg.Models.DefaultProvider,
		Persistent:            cfg.Hermes.Persistent,
		RequirePersistent:     cfg.Hermes.RequirePersistent,
		DisableToolsInSandbox: cfg.Hermes.DisableToolsInSandbox,
		SecurityMode:          cfg.Security.Mode,
		SandboxDir:            cfg.Runtime.SandboxDir,
		SandboxBackend:        cfg.Security.SandboxBackend,
		SandboxCommand:        cfg.Security.SandboxCommand,
		SandboxDockerImage:    cfg.Security.SandboxDockerImage,
		SandboxDockerArgs:     cfg.Security.SandboxDockerArgs,
		ProfileDir:            cfg.Hermes.ProfileDir,
	}
}

func (r Runner) AskWithOptions(ctx context.Context, prompt string, opts Options) (string, error) {
	text, _, err := r.AskWithSession(ctx, prompt, opts)
	return text, err
}

func (r Runner) AskWithSession(ctx context.Context, prompt string, opts Options) (string, string, error) {
	if opts.Persistent {
		reply, sessionID, err := askPersistentACP(ctx, r.Command, prompt, opts)
		if err == nil {
			return reply, sessionID, nil
		}
		if opts.RequirePersistent {
			return reply, sessionID, err
		}
	}
	args := BuildArgs(prompt, opts)
	argv := hermesArgv(r.Command, args)
	argv = applySandboxBackendArgv(argv, opts)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	applyHermesRuntime(cmd, opts)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = nil
	}
	return runWithIdleOutput(ctx, cmd, 2*time.Second)
}

func hermesArgv(command string, args []string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, command)
	out = append(out, args...)
	return out
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

func applyHermesRuntime(cmd *exec.Cmd, opts Options) {
	if opts.SecurityMode == "sandbox" && strings.TrimSpace(opts.SandboxDir) != "" {
		_ = os.MkdirAll(opts.SandboxDir, 0755)
		cmd.Dir = opts.SandboxDir
	}
	env := os.Environ()
	if strings.TrimSpace(opts.ProfileDir) != "" {
		env = hermesProfileEnv(env, opts.ProfileDir)
	}
	if opts.SecurityMode == "sandbox" && strings.TrimSpace(opts.SandboxDir) != "" {
		env = append(env,
			"BILLBOT_SECURITY_MODE=sandbox",
			"BILLBOT_SANDBOX_DIR="+opts.SandboxDir,
		)
	}
	cmd.Env = env
}

func hermesProfileEnv(base []string, profileDir string) []string {
	profileDir = strings.TrimSpace(profileDir)
	if profileDir == "" {
		return base
	}
	_ = os.MkdirAll(profileDir, 0755)
	return appendWithoutEnvKeys(base,
		"HOME="+profileDir,
		"USERPROFILE="+profileDir,
		"XDG_CONFIG_HOME="+filepath.Join(profileDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(profileDir, "data"),
		"XDG_CACHE_HOME="+filepath.Join(profileDir, "cache"),
		"HERMES_HOME="+profileDir,
		"BILLBOT_HERMES_PROFILE_DIR="+profileDir,
	)
}

func appendWithoutEnvKeys(base []string, values ...string) []string {
	keys := map[string]bool{}
	for _, value := range values {
		key, _, ok := strings.Cut(value, "=")
		if ok {
			keys[strings.ToUpper(key)] = true
		}
	}
	out := make([]string, 0, len(base)+len(values))
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if ok && keys[strings.ToUpper(key)] {
			continue
		}
		out = append(out, item)
	}
	return append(out, values...)
}

func dockerProfileArgs(profileDir string) []string {
	profileDir = strings.TrimSpace(profileDir)
	if profileDir == "" {
		return nil
	}
	_ = os.MkdirAll(profileDir, 0755)
	return []string{
		"-v", profileDir + ":/hermes-profile",
		"-e", "HOME=/hermes-profile",
		"-e", "USERPROFILE=/hermes-profile",
		"-e", "XDG_CONFIG_HOME=/hermes-profile/config",
		"-e", "XDG_DATA_HOME=/hermes-profile/data",
		"-e", "XDG_CACHE_HOME=/hermes-profile/cache",
		"-e", "HERMES_HOME=/hermes-profile",
		"-e", "BILLBOT_HERMES_PROFILE_DIR=/hermes-profile",
	}
}

func dockerSandboxArgs(sandboxDir string) []string {
	if strings.TrimSpace(sandboxDir) == "" {
		return nil
	}
	return []string{
		"-v", sandboxDir + ":/workspace",
		"-w", "/workspace",
	}
}

func sandboxEnvArgs(opts Options) []string {
	if opts.SecurityMode != "sandbox" || strings.TrimSpace(opts.SandboxDir) == "" {
		return nil
	}
	return []string{
		"-e", "BILLBOT_SECURITY_MODE=sandbox",
		"-e", "BILLBOT_SANDBOX_DIR=/workspace",
	}
}

func localSandboxEnv(opts Options) []string {
	if opts.SecurityMode != "sandbox" || strings.TrimSpace(opts.SandboxDir) == "" {
		return nil
	}
	return []string{
		"BILLBOT_SECURITY_MODE=sandbox",
		"BILLBOT_SANDBOX_DIR="+opts.SandboxDir,
	}
}

func applySandboxBackendArgv(argv []string, opts Options) []string {
	if opts.SecurityMode != "sandbox" || len(argv) == 0 {
		return argv
	}
	switch strings.ToLower(strings.TrimSpace(opts.SandboxBackend)) {
	case "", "workdir":
		return argv
	case "command":
		if len(opts.SandboxCommand) == 0 || strings.TrimSpace(opts.SandboxCommand[0]) == "" {
			return argv
		}
		out := append([]string{}, opts.SandboxCommand...)
		return append(out, argv...)
	case "docker":
		image := strings.TrimSpace(opts.SandboxDockerImage)
		if image == "" {
			return argv
		}
		out := []string{"docker", "run", "--rm", "-i"}
		out = append(out, opts.SandboxDockerArgs...)
		out = append(out, dockerProfileArgs(opts.ProfileDir)...)
		out = append(out, dockerSandboxArgs(opts.SandboxDir)...)
		out = append(out, sandboxEnvArgs(opts)...)
		out = append(out, image)
		return append(out, argv...)
	default:
		return argv
	}
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
				text, sessionID := cleanOutput(outputString(&mu, &out))
				if text != "" {
					killProcess(cmd)
					err := <-done
					if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
						err = nil
					}
					return text, sessionID, nil
				}
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
		if isHermesStatusLine(trimmed) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), sessionID
}

func isHermesStatusLine(line string) bool {
	return strings.Contains(line, "Resumed session ") ||
		strings.HasPrefix(line, "Resumed session ")
}
