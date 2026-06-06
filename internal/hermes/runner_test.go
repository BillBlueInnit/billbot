// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"billbot/internal/config"
)

func TestBuildArgsIncludesQuietPromptAndModel(t *testing.T) {
	got := BuildArgs("hello", Options{
		Model:     "test-model",
		SessionID: "session-1",
	})
	want := []string{"chat", "-Q", "-q", "hello", "-m", "test-model", "--resume", "session-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestBuildArgsCanDisableToolsInSandboxWhenConfigured(t *testing.T) {
	got := BuildArgs("hello", Options{
		SecurityMode:          "sandbox",
		DisableToolsInSandbox: true,
	})
	want := []string{"chat", "-Q", "-q", "hello", "-t", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestBuildArgsDoesNotRequireModel(t *testing.T) {
	got := BuildArgs("hello", Options{})
	want := []string{"chat", "-Q", "-q", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestOptionsFromConfigRequiresPersistentByDefault(t *testing.T) {
	cfg := config.Default()
	got := OptionsFromConfig(cfg)
	if !got.Persistent {
		t.Fatal("persistent should be enabled by default")
	}
	if !got.RequirePersistent {
		t.Fatal("require persistent should be enabled by default")
	}
}

func TestApplySandboxBackendArgvUsesCommandWrapper(t *testing.T) {
	got := applySandboxBackendArgv([]string{"hermes", "acp"}, Options{
		SecurityMode:   "sandbox",
		SandboxBackend: "command",
		SandboxCommand: []string{"isolate", "--box", "billbot", "--"},
	})
	want := []string{"isolate", "--box", "billbot", "--", "hermes", "acp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestApplySandboxBackendArgvUsesDocker(t *testing.T) {
	profileDir := t.TempDir()
	got := applySandboxBackendArgv([]string{"hermes", "chat"}, Options{
		SecurityMode:       "sandbox",
		SandboxBackend:     "docker",
		SandboxDir:         "D:\\billbot\\sandbox",
		SandboxDockerImage: "billbot-hermes:latest",
		SandboxDockerArgs:  []string{"--network", "none"},
		ProfileDir:         profileDir,
	})
	want := []string{
		"docker", "run", "--rm", "-i", "--network", "none",
		"-v", profileDir + ":/hermes-profile",
		"-e", "HOME=/hermes-profile",
		"-e", "USERPROFILE=/hermes-profile",
		"-e", "XDG_CONFIG_HOME=/hermes-profile/config",
		"-e", "XDG_DATA_HOME=/hermes-profile/data",
		"-e", "XDG_CACHE_HOME=/hermes-profile/cache",
		"-e", "HERMES_HOME=/hermes-profile",
		"-e", "BILLBOT_HERMES_PROFILE_DIR=/hermes-profile",
		"-v", "D:\\billbot\\sandbox:/workspace", "-w", "/workspace",
		"-e", "BILLBOT_SECURITY_MODE=sandbox",
		"-e", "BILLBOT_SANDBOX_DIR=/workspace",
		"billbot-hermes:latest", "hermes", "chat",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestHermesProfileEnvOverridesUserProfile(t *testing.T) {
	profileDir := t.TempDir()
	got := hermesProfileEnv([]string{"HOME=old", "PATH=test"}, profileDir)
	wantSuffix := []string{
		"PATH=test",
		"HOME=" + profileDir,
		"USERPROFILE=" + profileDir,
		"XDG_CONFIG_HOME=" + filepath.Join(profileDir, "config"),
		"XDG_DATA_HOME=" + filepath.Join(profileDir, "data"),
		"XDG_CACHE_HOME=" + filepath.Join(profileDir, "cache"),
		"HERMES_HOME=" + profileDir,
		"BILLBOT_HERMES_PROFILE_DIR=" + profileDir,
	}
	if !reflect.DeepEqual(got, wantSuffix) {
		t.Fatalf("env = %#v, want %#v", got, wantSuffix)
	}
}

func TestRunWithIdleOutputReturnsBeforeHungProcessExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperHungHermesOutput")
	cmd.Env = append(os.Environ(), "BILLBOT_HELPER_HUNG_HERMES=1")

	start := time.Now()
	out, sessionID, err := runWithIdleOutput(ctx, cmd, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("runWithIdleOutput returned error: %v", err)
	}
	if out != "OK" {
		t.Fatalf("output = %q, want OK", out)
	}
	if sessionID != "" {
		t.Fatalf("sessionID = %q, want empty", sessionID)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("runner waited too long: %s", time.Since(start))
	}
}

func TestRunWithIdleOutputDoesNotReturnStatusOnlyOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperHermesStatusThenReply")
	cmd.Env = append(os.Environ(), "BILLBOT_HELPER_HERMES_STATUS_THEN_REPLY=1")

	out, sessionID, err := runWithIdleOutput(ctx, cmd, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("runWithIdleOutput returned error: %v", err)
	}
	if out != "actual reply" {
		t.Fatalf("output = %q, want actual reply", out)
	}
	if sessionID != "" {
		t.Fatalf("sessionID = %q, want empty", sessionID)
	}
}

func TestCleanOutputDropsSessionID(t *testing.T) {
	got, sessionID := cleanOutput("OK\n\nsession_id: 20260605_x\n")
	if got != "OK" {
		t.Fatalf("cleanOutput = %q, want OK", got)
	}
	if sessionID != "20260605_x" {
		t.Fatalf("sessionID = %q, want 20260605_x", sessionID)
	}
}

func TestCleanOutputDropsHermesStatusLines(t *testing.T) {
	got, sessionID := cleanOutput("↻ Resumed session 20260605_230708_e18670 (1 user message, 4 total messages)\nactual reply\n")
	if got != "actual reply" {
		t.Fatalf("cleanOutput = %q, want actual reply", got)
	}
	if sessionID != "" {
		t.Fatalf("sessionID = %q, want empty", sessionID)
	}
}

func TestACPPromptContentIncludesImageAttachment(t *testing.T) {
	got := acpPromptContent("describe it", []Attachment{{
		Type: "image",
		URL:  "https://example.test/a.png",
		Name: "a.png",
	}})

	if len(got) != 2 {
		t.Fatalf("content blocks = %#v, want 2 blocks", got)
	}
	if got[0]["type"] != "text" || got[0]["text"] != "describe it" {
		t.Fatalf("text block = %#v", got[0])
	}
	if got[1]["type"] != "image" || got[1]["url"] != "https://example.test/a.png" || got[1]["name"] != "a.png" {
		t.Fatalf("image block = %#v", got[1])
	}
}

func TestACPPromptContentIncludesFileAttachment(t *testing.T) {
	got := acpPromptContent("read it", []Attachment{{
		Type: "file",
		File: "C:\\tmp\\answer.txt",
		Name: "answer.txt",
	}})

	if len(got) != 2 {
		t.Fatalf("content blocks = %#v, want 2 blocks", got)
	}
	if got[1]["type"] != "file" || got[1]["path"] != "C:\\tmp\\answer.txt" || got[1]["name"] != "answer.txt" {
		t.Fatalf("file block = %#v", got[1])
	}
}

func TestExtractACPTextFromJSONReadsPromptResult(t *testing.T) {
	got := extractACPTextFromJSON([]byte(`{"content":[{"type":"text","text":"hello from acp"}]}`))
	if got != "hello from acp" {
		t.Fatalf("text = %q, want hello from acp", got)
	}
}

func TestHelperHungHermesOutput(t *testing.T) {
	if os.Getenv("BILLBOT_HELPER_HUNG_HERMES") != "1" {
		return
	}
	fmt.Println("OK")
	time.Sleep(30 * time.Second)
}

func TestHelperHermesStatusThenReply(t *testing.T) {
	if os.Getenv("BILLBOT_HELPER_HERMES_STATUS_THEN_REPLY") != "1" {
		return
	}
	fmt.Println("↻ Resumed session 20260605_230708_e18670 (1 user message, 4 total messages)")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("actual reply")
	time.Sleep(30 * time.Second)
}
