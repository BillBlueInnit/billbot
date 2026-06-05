// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestBuildArgsIncludesQuietPromptModelAndSandboxTools(t *testing.T) {
	got := BuildArgs("hello", Options{
		Model:                 "test-model",
		SessionID:             "session-1",
		SecurityMode:          "sandbox",
		DisableToolsInSandbox: true,
	})
	want := []string{"chat", "-Q", "-q", "hello", "-m", "test-model", "--resume", "session-1", "-t", ""}
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

func TestCleanOutputDropsSessionID(t *testing.T) {
	got, sessionID := cleanOutput("OK\n\nsession_id: 20260605_x\n")
	if got != "OK" {
		t.Fatalf("cleanOutput = %q, want OK", got)
	}
	if sessionID != "20260605_x" {
		t.Fatalf("sessionID = %q, want 20260605_x", sessionID)
	}
}

func TestHelperHungHermesOutput(t *testing.T) {
	if os.Getenv("BILLBOT_HELPER_HUNG_HERMES") != "1" {
		return
	}
	fmt.Println("OK")
	time.Sleep(30 * time.Second)
}
