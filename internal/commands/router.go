// SPDX-License-Identifier: LGPL-3.0-only

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"billbot/internal/config"
	"billbot/internal/connector"
)

var commandPattern = regexp.MustCompile(`^/([A-Za-z][A-Za-z0-9_-]*)(?:\s+(.*))?$`)

type Result struct {
	Handled bool
	Reply   string
	Prompt  string
}

func Handle(ctx context.Context, cfg config.Config, msg connector.Message) (Result, error) {
	name, args, ok := Parse(msg.Text)
	if !ok {
		return Result{}, nil
	}
	cmd, ok := find(cfg.Commands, name)
	if !ok {
		return Result{Handled: true, Reply: "Unknown command: /" + name}, nil
	}
	if cmd.RequireAt && !isAtBot(msg.Text, cfg.Bridge.SelfID, msg.BotID) {
		return Result{Handled: true, Reply: "Command /" + name + " requires @bot."}, nil
	}
	if cmd.OwnerOnly || cmd.Type == "exec" {
		userID, _ := strconv.ParseInt(msg.UserID, 10, 64)
		if !isOwner(cfg.Owners, userID) {
			return Result{Handled: true, Reply: "Command /" + name + " requires owner."}, nil
		}
	}

	switch cmd.Type {
	case "prompt":
		return Result{Handled: true, Prompt: buildPromptCommandPrompt(cmd, args)}, nil
	case "skill":
		return Result{Handled: true, Prompt: buildSkillCommandPrompt(cmd, args)}, nil
	case "exec":
		reply, err := runExec(ctx, cmd)
		return Result{Handled: true, Reply: reply}, err
	default:
		return Result{Handled: true, Reply: "Unsupported command type: " + cmd.Type}, nil
	}
}

func Parse(text string) (name string, args string, ok bool) {
	text = stripLeadingAt(strings.TrimSpace(text))
	match := commandPattern.FindStringSubmatch(text)
	if match == nil {
		return "", "", false
	}
	name = strings.ToLower(match[1])
	if len(match) > 2 {
		args = strings.TrimSpace(match[2])
	}
	return name, args, true
}

func find(items []config.CommandConfig, name string) (config.CommandConfig, bool) {
	for _, item := range items {
		if strings.EqualFold(item.Name, name) {
			return item, true
		}
	}
	return config.CommandConfig{}, false
}

func isOwner(owners []int64, userID int64) bool {
	for _, owner := range owners {
		if owner == userID {
			return true
		}
	}
	return false
}

func isAtBot(text string, cfgSelfID int64, botID string) bool {
	selfID := cfgSelfID
	if selfID == 0 {
		selfID, _ = strconv.ParseInt(botID, 10, 64)
	}
	if selfID == 0 {
		return false
	}
	s := strconv.FormatInt(selfID, 10)
	return strings.Contains(text, "[CQ:at,qq="+s+"]") || strings.Contains(text, "@"+s)
}

func stripLeadingAt(text string) string {
	for {
		trimmed := strings.TrimSpace(text)
		if !strings.HasPrefix(trimmed, "[CQ:at,qq=") {
			return trimmed
		}
		end := strings.Index(trimmed, "]")
		if end < 0 {
			return trimmed
		}
		text = trimmed[end+1:]
	}
}

func buildPromptCommandPrompt(cmd config.CommandConfig, args string) string {
	prompt := strings.TrimSpace(cmd.Prompt)
	if args != "" {
		return prompt + "\n\nCommand args:\n" + args
	}
	return prompt
}

func buildSkillCommandPrompt(cmd config.CommandConfig, args string) string {
	var parts []string
	parts = append(parts, "Use configured skill: "+cmd.Skill)
	if cmd.Prompt != "" {
		parts = append(parts, cmd.Prompt)
	}
	if args != "" {
		parts = append(parts, "Command args:\n"+args)
	}
	return strings.Join(parts, "\n\n")
}

func runExec(ctx context.Context, cmd config.CommandConfig) (string, error) {
	if len(cmd.Exec) == 0 || strings.TrimSpace(cmd.Exec[0]) == "" {
		return "", fmt.Errorf("command /%s has empty exec argv", cmd.Name)
	}
	timeout := time.Duration(cmd.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := exec.CommandContext(runCtx, cmd.Exec[0], cmd.Exec[1:]...)
	out, err := c.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if text == "" && err != nil {
		text = err.Error()
	}
	return text, err
}
