// SPDX-License-Identifier: LGPL-3.0-only

package commands

import (
	"context"
	"os"
	"strings"
	"testing"

	"billbot/internal/config"
	"billbot/internal/connector"
)

func TestParseSlashCommand(t *testing.T) {
	name, args, ok := Parse("[CQ:at,qq=12345] /Status hello world")
	if !ok {
		t.Fatal("command was not parsed")
	}
	if name != "status" || args != "hello world" {
		t.Fatalf("name=%q args=%q", name, args)
	}
}

func TestHandleRequiresAt(t *testing.T) {
	cfg := config.Default()
	cfg.Bridge.SelfID = 12345
	cfg.Commands = []config.CommandConfig{{Name: "status", Type: "prompt", RequireAt: true, Prompt: "status prompt"}}

	result, err := Handle(context.Background(), cfg, message("/status", "10001"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !result.Handled || !strings.Contains(result.Reply, "requires @bot") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHandleExecRequiresOwner(t *testing.T) {
	cfg := config.Default()
	cfg.Owners = []int64{10001}
	cfg.Commands = []config.CommandConfig{{Name: "date", Type: "exec", Exec: []string{"echo", "ok"}}}

	result, err := Handle(context.Background(), cfg, message("/date", "20002"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !result.Handled || !strings.Contains(result.Reply, "requires owner") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHandleExecUsesConfiguredArgvOnly(t *testing.T) {
	cfg := config.Default()
	cfg.Owners = []int64{10001}
	cfg.Commands = []config.CommandConfig{{Name: "echo", Type: "exec", Exec: []string{os.Args[0], "-test.run=TestHelperExecCommand"}}}

	result, err := Handle(context.Background(), cfg, message("/echo ; rm -rf /*", "10001"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !result.Handled || !strings.Contains(result.Reply, "PASS") {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Contains(result.Reply, "rm -rf") {
		t.Fatalf("untrusted args leaked into exec output: %q", result.Reply)
	}
}

func TestHelperExecCommand(t *testing.T) {}

func message(text, userID string) connector.Message {
	return connector.Message{
		Platform: connector.PlatformQQ,
		BotID:    "12345",
		ChatID:   "private:" + userID,
		UserID:   userID,
		Private:  true,
		Text:     text,
	}
}
