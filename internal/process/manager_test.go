// SPDX-License-Identifier: LGPL-3.0-only

package process

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"billbot/internal/config"
)

func TestNapCatStatusReportsReadyExternalProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.ManagedProcessConfig{
		WaitHTTP:   server.URL,
		AutoStart:  true,
		StopOnExit: true,
	}
	got := NewManager().NapCatStatus(context.Background(), cfg)

	if got.Managed || got.Running {
		t.Fatalf("external process should not be reported as managed/running: %#v", got)
	}
	if !got.Ready {
		t.Fatalf("ready endpoint was not reported ready: %#v", got)
	}
	if got.WaitHTTP != server.URL || !got.AutoStart || !got.StopOnExit {
		t.Fatalf("config fields not reflected in status: %#v", got)
	}
}

func TestStartNapCatNoopWhenExternalReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.ManagedProcessConfig{
		WaitHTTP:  server.URL,
		AutoStart: true,
		Command:   "definitely-not-a-real-napcat-command",
	}
	if err := NewManager().StartNapCat(context.Background(), cfg); err != nil {
		t.Fatalf("ready external napcat should not require launching command: %v", err)
	}
}
