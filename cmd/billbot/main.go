// SPDX-License-Identifier: LGPL-3.0-only

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"billbot/internal/bridge"
	"billbot/internal/config"
	"billbot/internal/connector/napcat"
)

func main() {
	defaultConfigPath := filepath.Join(config.AppDir(), "config.yaml")
	port := flag.Int("port", 0, "dashboard port")
	configPath := flag.String("config", defaultConfigPath, "config path")
	webDir := flag.String("web", defaultWebDir(), "dashboard static file directory")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if *port > 0 {
		cfg.Server.Port = *port
	}
	cfg.Normalize()

	for _, dir := range []string{cfg.Runtime.DataDir, filepath.Dir(cfg.Runtime.LogFile), cfg.Runtime.OutboxDir, cfg.Runtime.TmpDir, cfg.Runtime.SandboxDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create runtime dir %s: %v", dir, err)
		}
	}

	bridgeSvc := bridge.NewService(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "service": "billbot", "config_path": *configPath})
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{"config": cfg, "config_path": *configPath})
		case http.MethodPost:
			var in config.Config
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			in.Normalize()
			if *port > 0 {
				in.Server.Port = *port
			}
			if err := config.Save(*configPath, in); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			cfg = in
			bridgeSvc.UpdateConfig(cfg)
			writeJSON(w, map[string]any{"ok": true, "config": cfg, "config_path": *configPath})
		default:
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		}
	})
	mux.HandleFunc("/api/connectors/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		conn := napcat.New(cfg.NapCat)
		status, err := conn.Status()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"connectors": []any{status}})
	})
	mux.HandleFunc("/api/bridge/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, map[string]any{"bridge": bridgeSvc.Status()})
	})
	mux.HandleFunc("/api/bridge/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		if err := bridgeSvc.Start(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "bridge": bridgeSvc.Status()})
	})
	mux.HandleFunc("/api/bridge/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		if err := bridgeSvc.Stop(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "bridge": bridgeSvc.Status()})
	})
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("billbot listening on %s", addr)
	log.Printf("dashboard web dir %s", *webDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
}

func defaultWebDir() string {
	if v := os.Getenv("BILLBOT_WEB_DIR"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "web")
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		dir := filepath.Join(wd, "web")
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
	}
	return "web"
}
