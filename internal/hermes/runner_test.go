// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"reflect"
	"testing"
)

func TestBuildArgsIncludesQuietPromptModelAndSandboxTools(t *testing.T) {
	got := BuildArgs("hello", Options{
		Model:                 "test-model",
		SecurityMode:          "sandbox",
		DisableToolsInSandbox: true,
	})
	want := []string{"chat", "-Q", "-q", "hello", "-m", "test-model", "-t", ""}
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
