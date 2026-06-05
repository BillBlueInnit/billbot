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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"billbot/internal/bridge"
	"billbot/internal/config"
	"billbot/internal/diagnostics"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

func main() {
	defaultConfigPath := defaultConfigPath()
	port := flag.Int("port", 0, "deprecated compatibility flag; no HTTP listener is started")
	configPath := flag.String("config", defaultConfigPath, "config path")
	cliMode := flag.Bool("cli", false, "deprecated compatibility flag; interactive CLI is the default")
	flag.Parse()

	configExists := fileExists(*configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if *port > 0 {
		cfg.Server.Port = *port
	}
	cfg.Normalize()
	if !configExists {
		cfg = generatedConfigDefaults(*configPath, cfg)
		if err := config.Save(*configPath, cfg); err != nil {
			log.Fatalf("create default config %s: %v", *configPath, err)
		}
		fmt.Printf("created default config: %s\n", *configPath)
	}

	for _, dir := range []string{cfg.Runtime.DataDir, filepath.Dir(cfg.Runtime.LogFile), cfg.Runtime.OutboxDir, cfg.Runtime.TmpDir, cfg.Runtime.SandboxDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create runtime dir %s: %v", dir, err)
		}
	}
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	setupLogging(cfg.Runtime.LogFile, interactive)
	log.Printf("billbot starting config=%s interactive=%t cli_flag=%t", *configPath, interactive, *cliMode)

	bridgeSvc := bridge.NewService(cfg)
	bridgeSvc.SetConfigPath(*configPath)
	runCLI(context.Background(), cfg, *configPath, bridgeSvc, interactive)
}

func defaultConfigPath() string {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		dir := filepath.Dir(exe)
		tomlPath := filepath.Join(dir, "config.toml")
		if fileExists(tomlPath) {
			return tomlPath
		}
		yamlPath := filepath.Join(dir, "config.yaml")
		if fileExists(yamlPath) {
			return yamlPath
		}
		return tomlPath
	}
	if fileExists("config.toml") {
		return "config.toml"
	}
	if fileExists("config.yaml") {
		return "config.yaml"
	}
	return "config.toml"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func generatedConfigDefaults(path string, cfg config.Config) config.Config {
	base := filepath.Dir(path)
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	runtimeDir := filepath.Join(base, "runtime")
	cfg.Runtime.DataDir = filepath.Join(runtimeDir, "data")
	cfg.Runtime.LogFile = filepath.Join(runtimeDir, "logs", "billbot.log")
	cfg.Runtime.OutboxDir = filepath.Join(runtimeDir, "outbox")
	cfg.Runtime.TmpDir = filepath.Join(runtimeDir, "tmp")
	cfg.Runtime.SandboxDir = filepath.Join(runtimeDir, "sandbox")
	return cfg
}

func setupLogging(path string, interactive bool) {
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
	if interactive {
		log.SetOutput(file)
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

type cliState struct {
	mu         sync.Mutex
	cfg        config.Config
	configPath string
	bridgeSvc  *bridge.Service
}

func newCLIState(cfg config.Config, configPath string, bridgeSvc *bridge.Service) *cliState {
	return &cliState{cfg: cfg, configPath: configPath, bridgeSvc: bridgeSvc}
}

func runCLI(ctx context.Context, cfg config.Config, configPath string, bridgeSvc *bridge.Service, interactive bool) {
	state := newCLIState(cfg, configPath, bridgeSvc)
	if interactive {
		runInteractiveCLI(ctx, state)
		return
	}
	runREPL(ctx, state)
}

func runREPL(ctx context.Context, state *cliState) {
	fmt.Println("BillBot CLI")
	fmt.Printf("config: %s\n", state.configPath)
	fmt.Println(strings.TrimSpace(helpText()))
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("billbot> ")
		if !scanner.Scan() {
			break
		}
		out, quit, clear := state.Execute(ctx, scanner.Text())
		if clear {
			clearScreen()
			continue
		}
		if strings.TrimSpace(out) != "" {
			fmt.Println(out)
		}
		if quit {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("cli input: %v", err)
	}
	_ = state.bridgeSvc.Stop()
}

func runInteractiveCLI(ctx context.Context, state *cliState) {
	historyFile := filepath.Join(filepath.Dir(state.configPath), "runtime", "billbot.history")
	if err := os.MkdirAll(filepath.Dir(historyFile), 0755); err != nil {
		log.Printf("create history dir: %v", err)
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "billbot> ",
		HistoryFile:     historyFile,
		InterruptPrompt: "^C",
		EOFPrompt:       "quit",
	})
	if err != nil {
		log.Printf("readline unavailable: %v", err)
		runREPL(ctx, state)
		return
	}
	defer rl.Close()

	fmt.Println("BillBot CLI")
	fmt.Printf("config: %s\n", state.configPath)
	fmt.Println(strings.TrimSpace(helpText()))
	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			fmt.Println("use quit to exit")
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("cli input: %v", err)
			break
		}
		out, quit, clear := state.Execute(ctx, line)
		if clear {
			clearScreen()
			continue
		}
		if strings.TrimSpace(out) != "" {
			fmt.Println(out)
		}
		if quit {
			return
		}
	}
	_ = state.bridgeSvc.Stop()
}

func (s *cliState) Execute(ctx context.Context, rawLine string) (output string, quit bool, clear bool) {
	line := normalizeCLIInput(rawLine)
	lower := strings.ToLower(line)
	if lower == "" || lower == "help" || lower == "?" {
		return helpText(), false, false
	}
	if lower == "clear" || lower == "cls" {
		return "", false, true
	}
	if lower == "quit" || lower == "exit" {
		_ = s.bridgeSvc.Stop()
		return "bye", true, false
	}

	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	switch lower {
	case "status":
		return formatJSON(map[string]any{
			"bridge":  s.bridgeSvc.Status(),
			"routing": routingSummary(cfg),
		}), false, false
	case "diag":
		report := diagnostics.Run(ctx, cfg)
		log.Printf("diagnostics napcat_http=%t napcat_ws=%t hermes_found=%t hermes_status=%t hermes_chat=%t", report.NapCat.HTTPReachable, report.NapCat.WSReachable, report.Hermes.CommandFound, report.Hermes.StatusOK, report.Hermes.ChatOK)
		return formatJSON(map[string]any{"diagnostics": report}), false, false
	case "route":
		return formatJSON(map[string]any{"routing": routingSummary(cfg)}), false, false
	case "route off", "routing off", "model default", "models default":
		return s.disableRouting(), false, false
	case "start":
		if err := s.bridgeSvc.Start(); err != nil {
			return "start failed: " + err.Error(), false, false
		}
		return formatJSON(map[string]any{"ok": true, "bridge": s.bridgeSvc.Status()}), false, false
	case "stop":
		if err := s.bridgeSvc.Stop(); err != nil {
			return "stop failed: " + err.Error(), false, false
		}
		return formatJSON(map[string]any{"ok": true, "bridge": s.bridgeSvc.Status()}), false, false
	case "logs":
		text, err := readLogTail(cfg.Runtime.LogFile, 65536)
		if err != nil {
			return "logs failed: " + err.Error(), false, false
		}
		return text, false, false
	}

	if strings.HasPrefix(lower, "set ") {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return "usage: set KEY VALUE", false, false
		}
		return s.setValue(parts[1], parts[2]), false, false
	}
	if key, value, ok := simpleSet(line); ok {
		return s.setValue(key, value), false, false
	}
	return "unknown command; type help", false, false
}

func simpleSet(line string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	switch strings.ToLower(parts[0]) {
	case "qq", "self_id", "admin", "owner", "token", "http_token", "ws_token", "http", "ws", "hermes":
		return parts[0], parts[1], true
	default:
		return "", "", false
	}
}

func (s *cliState) setValue(key string, value string) string {
	s.mu.Lock()
	next := s.cfg
	s.mu.Unlock()
	if err := setConfigValue(&next, key, value); err != nil {
		return "set failed: " + err.Error()
	}
	if err := config.Save(s.configPath, next); err != nil {
		return "save failed: " + err.Error()
	}
	s.mu.Lock()
	s.cfg = next
	s.mu.Unlock()
	s.bridgeSvc.UpdateConfig(next)
	return "saved " + canonicalConfigKey(key)
}

func (s *cliState) disableRouting() string {
	s.mu.Lock()
	next := s.cfg
	s.mu.Unlock()
	next.Models.DefaultProvider = ""
	next.Models.DefaultModel = ""
	next.Models.BaseProvider = ""
	next.Models.BaseModel = ""
	next.Models.StrongProvider = ""
	next.Models.StrongModel = ""
	next.Models.SpecialModel = ""
	if err := config.Save(s.configPath, next); err != nil {
		return "save failed: " + err.Error()
	}
	s.mu.Lock()
	s.cfg = next
	s.mu.Unlock()
	s.bridgeSvc.UpdateConfig(next)
	return "routing disabled; Hermes default model will be used"
}

func helpText() string {
	return strings.TrimSpace(`Commands:
  start / stop          start or stop bridge
  status                show bridge and routing status
  diag                  test NapCat HTTP/WS and Hermes
  route                 show Hermes routing config
  route off             clear model overrides and use Hermes default model
  logs                  show recent log tail
  clear                 clear the screen
  set qq <bot_qq>       save bridge self QQ number
  set admin <qq>        save admin/owner QQ number
  set token <token>     save the same NapCat token for HTTP and WS
  set http_token <tok>  save only NapCat HTTP token
  set ws_token <tok>    save only NapCat WS token
  set http <url>        save NapCat HTTP endpoint
  set ws <url>          save NapCat WS endpoint
  set hermes <command>  save Hermes command
  set KEY VALUE         save any supported config key
  qq <bot_qq>           shortcut for set qq
  admin <qq>            shortcut for set admin
  token <token>         shortcut for set token
  quit                  exit

Keys: Up/Down history. Use clear to clear screen. Paste with your terminal shortcut, usually Ctrl+Shift+V or right click.`)
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func formatJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func routingSummary(cfg config.Config) map[string]any {
	return map[string]any{
		"hermes_command":       cfg.Hermes.Command,
		"default_provider":     cfg.Models.DefaultProvider,
		"default_model":        cfg.Models.DefaultModel,
		"base_provider":        cfg.Models.BaseProvider,
		"base_model":           cfg.Models.BaseModel,
		"strong_provider":      cfg.Models.StrongProvider,
		"strong_model":         cfg.Models.StrongModel,
		"special_model":        cfg.Models.SpecialModel,
		"routing_timeout_sec":  cfg.Models.RoutingTimeoutSec,
		"flash_timeout_sec":    cfg.Models.FlashTimeoutSec,
		"heavy_timeout_sec":    cfg.Models.HeavyTimeoutSec,
		"routing_enabled":      cfg.Models.BaseProvider != "" || cfg.Models.BaseModel != "",
		"strong_route_enabled": (cfg.Models.BaseProvider != "" || cfg.Models.BaseModel != "") && (cfg.Models.StrongProvider != "" || cfg.Models.StrongModel != ""),
	}
}

func normalizeCLIInput(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
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
	key = canonicalConfigKey(key)
	value = normalizeSetValue(value)
	switch key {
	case "napcat.http":
		cfg.NapCat.HTTP = value
	case "napcat.ws":
		cfg.NapCat.WS = value
	case "napcat.access_token":
		cfg.NapCat.AccessToken = value
	case "napcat.token":
		cfg.NapCat.AccessToken = value
		cfg.NapCat.HTTPToken = value
		cfg.NapCat.WSToken = value
	case "napcat.http_token":
		cfg.NapCat.HTTPToken = value
	case "napcat.ws_token":
		cfg.NapCat.WSToken = value
	case "hermes.command":
		cfg.Hermes.Command = value
	case "hermes.persistent":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Hermes.Persistent = v
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
	case "owners", "admin", "owner":
		owners, err := parseInt64List(value)
		if err != nil {
			return err
		}
		cfg.Owners = owners
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
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	cfg.Normalize()
	return nil
}

func normalizeSetValue(value string) string {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case `""`, "''", "null", "none", "default", "-":
		return ""
	default:
		return value
	}
}

func canonicalConfigKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "qq", "self_id", "bot", "bot_qq":
		return "bridge.self_id"
	case "admin", "owner":
		return "admin"
	case "token", "napcat.token":
		return "napcat.token"
	case "access_token":
		return "napcat.access_token"
	case "http_token", "napcat.http_token":
		return "napcat.http_token"
	case "ws_token", "napcat.ws_token":
		return "napcat.ws_token"
	case "http", "napcat_http":
		return "napcat.http"
	case "ws", "napcat_ws":
		return "napcat.ws"
	case "hermes":
		return "hermes.command"
	default:
		return strings.ToLower(strings.TrimSpace(key))
	}
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

func parseInt64List(value string) ([]int64, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []int64
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		v, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one qq number is required")
	}
	return out, nil
}
