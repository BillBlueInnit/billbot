// SPDX-License-Identifier: LGPL-3.0-only

package diagnostics

import (
	"context"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"billbot/internal/config"
	"billbot/internal/connector/napcat"
	"billbot/internal/hermes"
)

type Report struct {
	NapCat NapCatReport `json:"napcat"`
	Hermes HermesReport `json:"hermes"`
}

type NapCatReport struct {
	HTTPReachable bool   `json:"http_reachable"`
	WSReachable   bool   `json:"ws_reachable"`
	StatusMessage string `json:"status_message,omitempty"`
}

type HermesReport struct {
	CommandFound bool   `json:"command_found"`
	StatusOK     bool   `json:"status_ok"`
	ChatOK       bool   `json:"chat_ok"`
	Message      string `json:"message,omitempty"`
}

func Run(ctx context.Context, cfg config.Config) Report {
	return Report{
		NapCat: checkNapCat(ctx, cfg),
		Hermes: checkHermes(ctx, cfg),
	}
}

func checkNapCat(ctx context.Context, cfg config.Config) NapCatReport {
	conn := napcat.New(cfg.NapCat)
	status, _ := conn.Status()
	out := NapCatReport{
		HTTPReachable: status.Connected,
		StatusMessage: status.Message,
	}
	wsURL, err := url.Parse(cfg.NapCat.WS)
	if err == nil && (wsURL.Scheme == "ws" || wsURL.Scheme == "wss") {
		host := wsURL.Host
		if !strings.Contains(host, ":") {
			if wsURL.Scheme == "wss" {
				host += ":443"
			} else {
				host += ":80"
			}
		}
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", host)
		if err == nil {
			out.WSReachable = true
			_ = conn.Close()
		}
	}
	return out
}

func checkHermes(ctx context.Context, cfg config.Config) HermesReport {
	command := cfg.Hermes.Command
	if command == "" {
		command = "hermes"
	}
	if _, err := exec.LookPath(command); err != nil {
		return HermesReport{CommandFound: false, Message: err.Error()}
	}
	out := HermesReport{CommandFound: true}
	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	statusCmd := exec.CommandContext(statusCtx, command, "status")
	statusBytes, statusErr := statusCmd.CombinedOutput()
	out.StatusOK = statusErr == nil
	out.Message = compact(string(statusBytes))

	chatCtx, chatCancel := context.WithTimeout(ctx, 20*time.Second)
	defer chatCancel()
	runner := hermes.NewRunner(command)
	chatText, chatErr := runner.AskWithOptions(chatCtx, "BillBot diagnostic smoke test. Reply with OK.", hermes.OptionsFromConfig(cfg))
	if chatErr == nil {
		out.ChatOK = true
		out.Message = compact(chatText)
		return out
	}
	msg := strings.TrimSpace(chatText)
	if msg == "" {
		msg = "chat failed: " + chatErr.Error()
	}
	out.Message = compact(msg)
	return out
}

func compact(text string) string {
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	text = strings.TrimSpace(text)
	if len(text) > 2000 {
		return text[:2000]
	}
	return text
}
