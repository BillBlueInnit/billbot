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

	"billbot/internal/autostart"
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
	log.Printf("billbot starting config=%s port=%d cli=%t", *configPath, cfg.Server.Port, *cliMode)

	bridgeSvc := bridge.NewService(cfg)
	if err := startConfiguredBridge(cfg, bridgeSvc); err != nil {
		log.Printf("auto start bridge failed: %v", err)
	}
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
		report := diagnostics.Run(r.Context(), cfg)
		log.Printf("diagnostics napcat_http=%t napcat_ws=%t hermes_found=%t hermes_status=%t hermes_chat=%t", report.NapCat.HTTPReachable, report.NapCat.WSReachable, report.Hermes.CommandFound, report.Hermes.StatusOK, report.Hermes.ChatOK)
		writeJSON(w, map[string]any{"diagnostics": report})
	})
	mux.HandleFunc("/api/processes/napcat/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, map[string]any{"napcat_process": bridgeSvc.NapCatProcessStatus(r.Context())})
	})
	mux.HandleFunc("/api/processes/napcat/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		if err := bridgeSvc.StartNapCatProcess(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "napcat_process": bridgeSvc.NapCatProcessStatus(r.Context())})
	})
	mux.HandleFunc("/api/processes/napcat/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		if err := bridgeSvc.StopNapCatProcess(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "napcat_process": bridgeSvc.NapCatProcessStatus(r.Context())})
	})
	mux.HandleFunc("/api/autostart/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, map[string]any{"autostart": autostart.NewManager().Status(cfg, autostartOptions(*configPath, cfg))})
	})
	mux.HandleFunc("/api/autostart/enable", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		status, err := autostart.NewManager().Enable(r.Context(), cfg, autostartOptions(*configPath, cfg))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		cfg.Autostart.Enabled = true
		if err := config.Save(*configPath, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		log.Printf("autostart enabled platform=%s target=%s", status.Platform, status.Target)
		writeJSON(w, map[string]any{"ok": true, "autostart": status})
	})
	mux.HandleFunc("/api/autostart/disable", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		status, err := autostart.NewManager().Disable(r.Context(), cfg, autostartOptions(*configPath, cfg))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		cfg.Autostart.Enabled = false
		if err := config.Save(*configPath, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		log.Printf("autostart disabled platform=%s target=%s", status.Platform, status.Target)
		writeJSON(w, map[string]any{"ok": true, "autostart": status})
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

func startConfiguredBridge(cfg config.Config, bridgeSvc *bridge.Service) error {
	if !cfg.Bridge.Enabled {
		return nil
	}
	return bridgeSvc.Start()
}

func autostartOptions(configPath string, cfg config.Config) autostart.Options {
	exe, _ := os.Executable()
	return autostart.Options{
		ExePath:    exe,
		ConfigPath: configPath,
		Port:       cfg.Server.Port,
	}
}

func runCLI(ctx context.Context, cfg config.Config, configPath string, bridgeSvc *bridge.Service) {
	fmt.Println("BillBot CLI")
	fmt.Printf("config: %s\n", configPath)
	fmt.Printf("dashboard port: %d\n", cfg.Server.Port)
	fmt.Println("commands: status, diag, start, stop, napcat, napcat-start, napcat-stop, autostart, autostart-enable, autostart-disable, qr, login, logs, set, help, quit")
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
			fmt.Println("napcat  show managed NapCat process status")
			fmt.Println("napcat-start  start configured NapCat process")
			fmt.Println("napcat-stop   stop managed NapCat process")
			fmt.Println("autostart  show OS autostart status")
			fmt.Println("autostart-enable   enable OS autostart for BillBot")
			fmt.Println("autostart-disable  disable OS autostart for BillBot")
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
			report := diagnostics.Run(ctx, cfg)
			log.Printf("diagnostics napcat_http=%t napcat_ws=%t hermes_found=%t hermes_status=%t hermes_chat=%t", report.NapCat.HTTPReachable, report.NapCat.WSReachable, report.Hermes.CommandFound, report.Hermes.StatusOK, report.Hermes.ChatOK)
			printJSON(map[string]any{"diagnostics": report})
		case "napcat":
			printJSON(map[string]any{"napcat_process": bridgeSvc.NapCatProcessStatus(ctx)})
		case "napcat-start":
			if err := bridgeSvc.StartNapCatProcess(ctx); err != nil {
				fmt.Printf("napcat start failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "napcat_process": bridgeSvc.NapCatProcessStatus(ctx)})
		case "napcat-stop":
			if err := bridgeSvc.StopNapCatProcess(); err != nil {
				fmt.Printf("napcat stop failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "napcat_process": bridgeSvc.NapCatProcessStatus(ctx)})
		case "autostart":
			printJSON(map[string]any{"autostart": autostart.NewManager().Status(cfg, autostartOptions(configPath, cfg))})
		case "autostart-enable":
			status, err := autostart.NewManager().Enable(ctx, cfg, autostartOptions(configPath, cfg))
			if err != nil {
				fmt.Printf("autostart enable failed: %v\n", err)
				continue
			}
			cfg.Autostart.Enabled = true
			if err := config.Save(configPath, cfg); err != nil {
				fmt.Printf("save failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "autostart": status})
		case "autostart-disable":
			status, err := autostart.NewManager().Disable(ctx, cfg, autostartOptions(configPath, cfg))
			if err != nil {
				fmt.Printf("autostart disable failed: %v\n", err)
				continue
			}
			cfg.Autostart.Enabled = false
			if err := config.Save(configPath, cfg); err != nil {
				fmt.Printf("save failed: %v\n", err)
				continue
			}
			printJSON(map[string]any{"ok": true, "autostart": status})
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
	case "models.special_model":
		cfg.Models.SpecialModel = value
	case "models.routing_timeout_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Models.RoutingTimeoutSec = v
	case "models.flash_timeout_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Models.FlashTimeoutSec = v
	case "models.heavy_timeout_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Models.HeavyTimeoutSec = v
	case "runtime.start_notice_delay_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Runtime.StartNoticeDelaySec = v
	case "runtime.progress_interval_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Runtime.ProgressIntervalSec = v
	case "runtime.max_turns":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Runtime.MaxTurns = v
	case "login.qr_command":
		cfg.Login.QRCommand = strings.Fields(value)
	case "login.status_command":
		cfg.Login.StatusCommand = strings.Fields(value)
	case "processes.napcat.command":
		cfg.Processes.NapCat.Command = value
	case "processes.napcat.args":
		cfg.Processes.NapCat.Args = strings.Fields(value)
	case "processes.napcat.work_dir":
		cfg.Processes.NapCat.WorkDir = value
	case "processes.napcat.wait_http":
		cfg.Processes.NapCat.WaitHTTP = value
	case "processes.napcat.wait_timeout_sec":
		v, err := parsePositiveInt(value)
		if err != nil {
			return err
		}
		cfg.Processes.NapCat.WaitTimeout = v
	case "processes.napcat.auto_start":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Processes.NapCat.AutoStart = v
	case "processes.napcat.stop_on_exit":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Processes.NapCat.StopOnExit = v
	case "bridge.enabled":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Bridge.Enabled = v
	case "bridge.respond_to_group_mentions_only":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Bridge.RespondToGroupMentionsOnly = v
	case "bridge.self_id":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		cfg.Bridge.SelfID = v
	case "hermes.disable_tools_in_sandbox":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Hermes.DisableToolsInSandbox = v
	case "security.mode":
		cfg.Security.Mode = value
	case "security.allow_full_for_owners_only":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Security.AllowFullForOwnersOnly = v
	case "security.allow_non_owner_sensitive":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Security.AllowNonOwnerSensitive = v
	case "autostart.enabled":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Autostart.Enabled = v
	case "autostart.name":
		cfg.Autostart.Name = value
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	cfg.Normalize()
	return nil
}

func parsePositiveInt(value string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, fmt.Errorf("value must be positive")
	}
	return v, nil
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
