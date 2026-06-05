// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"context"
	"os/exec"
	"runtime"

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
	DisableToolsInSandbox bool
	SecurityMode          string
}

func OptionsFromConfig(cfg config.Config) Options {
	model := cfg.Models.DefaultModel
	return Options{
		Model:                 model,
		DisableToolsInSandbox: cfg.Hermes.DisableToolsInSandbox,
		SecurityMode:          cfg.Security.Mode,
	}
}

func (r Runner) AskWithOptions(ctx context.Context, prompt string, opts Options) (string, error) {
	args := BuildArgs(prompt, opts)
	cmd := exec.CommandContext(ctx, r.Command, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = nil
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func BuildArgs(prompt string, opts Options) []string {
	args := []string{"chat", "-Q", "-q", prompt}
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}
	if opts.SecurityMode == "sandbox" && opts.DisableToolsInSandbox {
		args = append(args, "-t", "")
	}
	return args
}
