// SPDX-License-Identifier: LGPL-3.0-only

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"billbot/internal/bridge"
	"billbot/internal/config"
	"billbot/internal/connector/napcat"
	"billbot/internal/diagnostics"
	"billbot/internal/loginqr"
)

func main() {
	defaultConfigPath := filepath.Join(config.AppDir(), "config.yaml")
	port := flag.Int("port", 0, "dashboard port")
	configPath := flag.String("config", defaultConfigPath, "config path")
	webDir := flag.String("web", defaultWebDir(), "dashboard static file directory")
	cliMode := flag.Bool("cli", false, "run terminal control mode")
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
	setupLogging(cfg.Runtime.LogFile)

	bridgeSvc := bridge.NewService(cfg)
	if *cliMode {
		runCLI(context.Background(), cfg, *configPath, bridgeSvc)
		return
	}

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
	mux.HandleFunc("/api/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, map[string]any{"diagnostics": diagnostics.Run(r.Context(), cfg)})
	})
	mux.HandleFunc("/api/login/qr", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		out, err := loginqr.Fetch(r.Context(), cfg.Login)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"qr": out})
	})
	mux.HandleFunc("/api/login/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		out, err := loginqr.Status(r.Context(), cfg.Login)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"login": out})
	})
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		text, err := readLogTail(cfg.Runtime.LogFile, 65536)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"log": text})
	})
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("billbot listening on %s", addr)
	log.Printf("dashboard web dir %s", *webDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func setupLogging(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Printf("create log dir: %v", err)
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("open log file: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))
}

func runCLI(ctx context.Context, cfg config.Config, configPath string, bridgeSvc *bridge.Service) {
	fmt.Println("BillBot CLI")
	fmt.Printf("config: %s\n", configPath)
	fmt.Printf("dashboard port: %d\n", cfg.Server.Port)
	fmt.Println("commands: status, diag, start, stop, qr, login, logs, set, help, quit")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("billbot> ")
		if !scanner.Scan() {
			break
		}
		line := normalizeCLIInput(scanner.Text())
		switch strings.ToLower(line) {
		case "", "help":
			fmt.Println("status  show bridge status")
			fmt.Println("diag    run NapCat and Hermes diagnostics")
			fmt.Println("start   start bridge and configured NapCat process")
			fmt.Println("stop    stop bridge")
			fmt.Println("qr      show QQ login QR availability")
			fmt.Println("login   show QQ login status")
			fmt.Println("logs    show recent runtime logs")
			fmt.Println("set KEY VALUE  update a supported config field and save")
			fmt.Println("quit    exit CLI")
		case "status":
			printJSON(map[string]any{
				"bridge":      bridgeSvc.Status(),
				"dashboard":   fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port),
				"diagnostics": diagnostics.Run(ctx, cfg),
			})
		case "diag":
			printJSON(map[string]any{"diagnostics": diagnostics.Run(ctx, cfg)})
		case "start":
			if err := bridgeSvc.Start(); err != nil {
				fmt.Printf("start failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "bridge": bridgeSvc.Status()})
		case "stop":
			if err := bridgeSvc.Stop(); err != nil {
				fmt.Printf("stop failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "bridge": bridgeSvc.Status()})
		case "qr":
			out, err := loginqr.Fetch(ctx, cfg.Login)
			if err != nil {
				fmt.Printf("qr failed: %v\n", err)
				fmt.Println("Configure login.qr_command with a cross-platform NapCat launcher/API command that prints QR content.")
				continue
			}
			fmt.Println(out.Render)
		case "login":
			out, err := loginqr.Status(ctx, cfg.Login)
			if err != nil {
				fmt.Printf("login status failed: %v\n", err)
				fmt.Println("Configure login.status_command with a cross-platform NapCat launcher/API command.")
				continue
			}
			printJSON(map[string]any{"login": out})
		case "logs":
			text, err := readLogTail(cfg.Runtime.LogFile, 65536)
			if err != nil {
				fmt.Printf("logs failed: %v\n", err)
				continue
			}
			fmt.Print(text)
		case "quit", "exit":
			_ = bridgeSvc.Stop()
			return
		default:
			if strings.HasPrefix(strings.ToLower(line), "set ") {
				parts := strings.SplitN(line, " ", 3)
				if len(parts) != 3 {
					fmt.Println("usage: set KEY VALUE")
					continue
				}
				next := cfg
				if err := setConfigValue(&next, parts[1], parts[2]); err != nil {
					fmt.Printf("set failed: %v\n", err)
					continue
				}
				if err := config.Save(configPath, next); err != nil {
					fmt.Printf("save failed: %v\n", err)
					continue
				}
				cfg = next
				bridgeSvc.UpdateConfig(cfg)
				fmt.Println("saved")
				continue
			}
			fmt.Println("unknown command; type help")
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("cli input: %v", err)
	}
	_ = bridgeSvc.Stop()
}

func normalizeCLIInput(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}

func readLogTail(path string, maxBytes int64) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("runtime.log_file is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	start := int64(0)
	if info.Size() > maxBytes {
		start = info.Size() - maxBytes
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func setConfigValue(cfg *config.Config, key string, value string) error {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "napcat.http":
		cfg.NapCat.HTTP = value
	case "napcat.ws":
		cfg.NapCat.WS = value
	case "hermes.command":
		cfg.Hermes.Command = value
	case "models.default_provider":
		cfg.Models.DefaultProvider = value
	case "models.default_model":
		cfg.Models.DefaultModel = value
	case "models.base_provider":
		cfg.Models.BaseProvider = value
	case "models.base_model":
		cfg.Models.BaseModel = value
	case "models.strong_provider":
		cfg.Models.StrongProvider = value
	case "models.strong_model":
		cfg.Models.StrongModel = value
	case "login.qr_command":
		cfg.Login.QRCommand = strings.Fields(value)
	case "login.status_command":
		cfg.Login.StatusCommand = strings.Fields(value)
	case "processes.napcat.command":
		cfg.Processes.NapCat.Command = value
	case "processes.napcat.auto_start":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Processes.NapCat.AutoStart = v
	case "bridge.self_id":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		cfg.Bridge.SelfID = v
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	cfg.Normalize()
	return nil
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
