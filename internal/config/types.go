// SPDX-License-Identifier: LGPL-3.0-only

package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name      string          `json:"name" yaml:"name"`
	Server    ServerConfig    `json:"server" yaml:"server"`
	Runtime   RuntimeConfig   `json:"runtime" yaml:"runtime"`
	Processes ProcessesConfig `json:"processes" yaml:"processes"`
	Connector ConnectorConfig `json:"connector" yaml:"connector"`
	NapCat    NapCatConfig    `json:"napcat" yaml:"napcat"`
	Bridge    BridgeConfig    `json:"bridge" yaml:"bridge"`
	Hermes    HermesConfig    `json:"hermes" yaml:"hermes"`
	Commands  []CommandConfig `json:"commands" yaml:"commands"`
	Models    ModelsConfig    `json:"models" yaml:"models"`
	Owners    []int64         `json:"owners" yaml:"owners"`
	Prompt    PromptConfig    `json:"prompt" yaml:"prompt"`
	Security  SecurityConfig  `json:"security" yaml:"security"`
}

type ServerConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

type RuntimeConfig struct {
	DataDir             string `json:"data_dir" yaml:"data_dir"`
	LogFile             string `json:"log_file" yaml:"log_file"`
	OutboxDir           string `json:"outbox_dir" yaml:"outbox_dir"`
	TmpDir              string `json:"tmp_dir" yaml:"tmp_dir"`
	SandboxDir          string `json:"sandbox_dir" yaml:"sandbox_dir"`
	SaveIntervalSec     int    `json:"save_interval_sec" yaml:"save_interval_sec"`
	StartNoticeDelaySec int    `json:"start_notice_delay_sec" yaml:"start_notice_delay_sec"`
	ProgressIntervalSec int    `json:"progress_interval_sec" yaml:"progress_interval_sec"`
	MaxTurns            int    `json:"max_turns" yaml:"max_turns"`
}

type ProcessesConfig struct {
	NapCat ManagedProcessConfig `json:"napcat" yaml:"napcat"`
}

type ManagedProcessConfig struct {
	AutoStart   bool     `json:"auto_start" yaml:"auto_start"`
	Command     string   `json:"command" yaml:"command"`
	Args        []string `json:"args" yaml:"args"`
	WorkDir     string   `json:"work_dir" yaml:"work_dir"`
	WaitHTTP    string   `json:"wait_http" yaml:"wait_http"`
	WaitTimeout int      `json:"wait_timeout_sec" yaml:"wait_timeout_sec"`
	StopOnExit  bool     `json:"stop_on_exit" yaml:"stop_on_exit"`
}

type ConnectorConfig struct {
	Mode string `json:"mode" yaml:"mode"`
	Name string `json:"name" yaml:"name"`
}

type NapCatConfig struct {
	HTTP string `json:"http" yaml:"http"`
	WS   string `json:"ws" yaml:"ws"`
}

type BridgeConfig struct {
	Enabled                    bool  `json:"enabled" yaml:"enabled"`
	RespondToGroupMentionsOnly bool  `json:"respond_to_group_mentions_only" yaml:"respond_to_group_mentions_only"`
	SelfID                     int64 `json:"self_id" yaml:"self_id"`
}

type HermesConfig struct {
	Command               string `json:"command" yaml:"command"`
	DisableToolsInSandbox bool   `json:"disable_tools_in_sandbox" yaml:"disable_tools_in_sandbox"`
}

type CommandConfig struct {
	Name       string   `json:"name" yaml:"name"`
	Type       string   `json:"type" yaml:"type"`
	RequireAt  bool     `json:"require_at" yaml:"require_at"`
	OwnerOnly  bool     `json:"owner_only" yaml:"owner_only"`
	Model      string   `json:"model" yaml:"model"`
	Provider   string   `json:"provider" yaml:"provider"`
	Skill      string   `json:"skill" yaml:"skill"`
	Prompt     string   `json:"prompt" yaml:"prompt"`
	Exec       []string `json:"exec" yaml:"exec"`
	TimeoutSec int      `json:"timeout_sec" yaml:"timeout_sec"`
}

type ModelsConfig struct {
	DefaultModel      string `json:"default_model" yaml:"default_model"`
	DefaultProvider   string `json:"default_provider" yaml:"default_provider"`
	BaseModel         string `json:"base_model" yaml:"base_model"`
	BaseProvider      string `json:"base_provider" yaml:"base_provider"`
	StrongModel       string `json:"strong_model" yaml:"strong_model"`
	StrongProvider    string `json:"strong_provider" yaml:"strong_provider"`
	SpecialModel      string `json:"special_model" yaml:"special_model"`
	RoutingTimeoutSec int    `json:"routing_timeout_sec" yaml:"routing_timeout_sec"`
	FlashTimeoutSec   int    `json:"flash_timeout_sec" yaml:"flash_timeout_sec"`
	HeavyTimeoutSec   int    `json:"heavy_timeout_sec" yaml:"heavy_timeout_sec"`
}

type PromptConfig struct {
	Identity string `json:"identity" yaml:"identity"`
	Style    string `json:"style" yaml:"style"`
}

type SecurityConfig struct {
	Mode                   string `json:"mode" yaml:"mode"`
	AllowFullForOwnersOnly bool   `json:"allow_full_for_owners_only" yaml:"allow_full_for_owners_only"`
	AllowNonOwnerSensitive bool   `json:"allow_non_owner_sensitive" yaml:"allow_non_owner_sensitive"`
}

func AppDir() string {
	if v := os.Getenv("BILLBOT_HOME"); v != "" {
		return v
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "billbot")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".billbot")
	}
	return "."
}

func Default() Config {
	app := AppDir()
	return Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 2006},
		Runtime: RuntimeConfig{
			DataDir:             filepath.Join(app, "data"),
			LogFile:             filepath.Join(app, "logs", "billbot.log"),
			OutboxDir:           filepath.Join(app, "outbox"),
			TmpDir:              filepath.Join(app, "tmp"),
			SandboxDir:          filepath.Join(app, "sandbox"),
			SaveIntervalSec:     10,
			StartNoticeDelaySec: 6,
			ProgressIntervalSec: 25,
			MaxTurns:            120,
		},
		Processes: ProcessesConfig{
			NapCat: ManagedProcessConfig{WaitHTTP: "http://127.0.0.1:3000/get_status", WaitTimeout: 30},
		},
		Connector: ConnectorConfig{Mode: "external", Name: "napcat"},
		NapCat:    NapCatConfig{HTTP: "http://127.0.0.1:3000", WS: "ws://127.0.0.1:3001"},
		Bridge:    BridgeConfig{RespondToGroupMentionsOnly: true},
		Hermes:    HermesConfig{Command: "hermes", DisableToolsInSandbox: true},
		Models:    ModelsConfig{RoutingTimeoutSec: 30, FlashTimeoutSec: 90, HeavyTimeoutSec: 300},
		Owners:    []int64{},
		Prompt:    PromptConfig{},
		Security:  SecurityConfig{Mode: "sandbox", AllowFullForOwnersOnly: true},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.Normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := marshalPreservingUnknown(path, cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func marshalPreservingUnknown(path string, cfg Config) ([]byte, error) {
	var next yaml.Node
	if err := next.Encode(cfg); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return yaml.Marshal(cfg)
		}
		return nil, err
	}
	var existing yaml.Node
	if err := yaml.Unmarshal(b, &existing); err != nil {
		return nil, err
	}
	mergeUnknownFields(documentMapping(&existing), documentMapping(&next))
	return yaml.Marshal(&next)
}

func documentMapping(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	if node.Kind == yaml.MappingNode {
		return node
	}
	return nil
}

func mergeUnknownFields(existing, next *yaml.Node) {
	if existing == nil || next == nil || existing.Kind != yaml.MappingNode || next.Kind != yaml.MappingNode {
		return
	}
	index := map[string]int{}
	for i := 0; i+1 < len(next.Content); i += 2 {
		index[next.Content[i].Value] = i
	}
	for i := 0; i+1 < len(existing.Content); i += 2 {
		key := existing.Content[i]
		value := existing.Content[i+1]
		nextIndex, ok := index[key.Value]
		if !ok {
			next.Content = append(next.Content, cloneNode(key), cloneNode(value))
			continue
		}
		mergeUnknownFields(value, next.Content[nextIndex+1])
	}
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	out := *node
	if len(node.Content) > 0 {
		out.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			out.Content[i] = cloneNode(child)
		}
	}
	return &out
}

func (c *Config) Normalize() {
	def := Default()
	if c.Server.Host == "" {
		c.Server.Host = def.Server.Host
	}
	if c.Server.Port == 0 {
		c.Server.Port = def.Server.Port
	}
	if c.Runtime.DataDir == "" {
		c.Runtime.DataDir = def.Runtime.DataDir
	}
	if c.Runtime.LogFile == "" {
		c.Runtime.LogFile = def.Runtime.LogFile
	}
	if c.Runtime.OutboxDir == "" {
		c.Runtime.OutboxDir = def.Runtime.OutboxDir
	}
	if c.Runtime.TmpDir == "" {
		c.Runtime.TmpDir = def.Runtime.TmpDir
	}
	if c.Runtime.SandboxDir == "" {
		c.Runtime.SandboxDir = def.Runtime.SandboxDir
	}
	if c.Runtime.SaveIntervalSec == 0 {
		c.Runtime.SaveIntervalSec = def.Runtime.SaveIntervalSec
	}
	if c.Runtime.StartNoticeDelaySec == 0 {
		c.Runtime.StartNoticeDelaySec = def.Runtime.StartNoticeDelaySec
	}
	if c.Runtime.ProgressIntervalSec == 0 {
		c.Runtime.ProgressIntervalSec = def.Runtime.ProgressIntervalSec
	}
	if c.Runtime.MaxTurns == 0 {
		c.Runtime.MaxTurns = def.Runtime.MaxTurns
	}
	if c.Connector.Mode == "" {
		c.Connector.Mode = def.Connector.Mode
	}
	if c.Processes.NapCat.WaitHTTP == "" {
		c.Processes.NapCat.WaitHTTP = def.Processes.NapCat.WaitHTTP
	}
	if c.Processes.NapCat.WaitTimeout == 0 {
		c.Processes.NapCat.WaitTimeout = def.Processes.NapCat.WaitTimeout
	}
	if c.Connector.Name == "" {
		c.Connector.Name = def.Connector.Name
	}
	if c.NapCat.HTTP == "" {
		c.NapCat.HTTP = def.NapCat.HTTP
	}
	if c.NapCat.WS == "" {
		c.NapCat.WS = def.NapCat.WS
	}
	if c.Hermes.Command == "" {
		c.Hermes.Command = def.Hermes.Command
	}
	if c.Models.FlashTimeoutSec == 0 {
		c.Models.FlashTimeoutSec = def.Models.FlashTimeoutSec
	}
	if c.Models.RoutingTimeoutSec == 0 {
		c.Models.RoutingTimeoutSec = def.Models.RoutingTimeoutSec
	}
	if c.Models.HeavyTimeoutSec == 0 {
		c.Models.HeavyTimeoutSec = def.Models.HeavyTimeoutSec
	}
	if c.Security.Mode == "" {
		c.Security.Mode = def.Security.Mode
	}
}
