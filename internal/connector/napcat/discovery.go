// SPDX-License-Identifier: LGPL-3.0-only

package napcat

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"billbot/internal/config"
)

type Discovery struct {
	Config        config.NapCatConfig `json:"config"`
	Source        string              `json:"source"`
	HTTPReachable bool                `json:"http_reachable"`
	WSReachable   bool                `json:"ws_reachable"`
	Message       string              `json:"message,omitempty"`
}

func Detect(ctx context.Context, configured config.NapCatConfig) Discovery {
	candidates := candidateConfigs(configured)
	var best Discovery
	for i, candidate := range candidates {
		source := "configured"
		if i > 0 {
			source = "default"
		}
		current := Discovery{Config: candidate, Source: source}
		status, _ := New(candidate).Status()
		current.HTTPReachable = status.Connected
		current.Message = status.Message
		current.WSReachable = wsReachable(ctx, candidate.WS)
		if current.HTTPReachable && current.WSReachable {
			return current
		}
		if best.Config.HTTP == "" || score(current) > score(best) {
			best = current
		}
	}
	if best.Config.HTTP == "" {
		best.Config = configured
		best.Source = "configured"
		best.Message = "no napcat candidates configured"
	}
	return best
}

func candidateConfigs(configured config.NapCatConfig) []config.NapCatConfig {
	var out []config.NapCatConfig
	add := func(candidate config.NapCatConfig) {
		if strings.TrimSpace(candidate.HTTP) == "" || strings.TrimSpace(candidate.WS) == "" {
			return
		}
		for _, existing := range out {
			if existing.HTTP == candidate.HTTP && existing.WS == candidate.WS {
				return
			}
		}
		out = append(out, candidate)
	}
	add(configured)
	add(config.NapCatConfig{
		HTTP:        "http://127.0.0.1:3000",
		WS:          "ws://127.0.0.1:3001",
		AccessToken: configured.AccessToken,
		HTTPToken:   configured.HTTPToken,
		WSToken:     configured.WSToken,
	})
	add(config.NapCatConfig{
		HTTP:        "http://localhost:3000",
		WS:          "ws://localhost:3001",
		AccessToken: configured.AccessToken,
		HTTPToken:   configured.HTTPToken,
		WSToken:     configured.WSToken,
	})
	return out
}

func score(d Discovery) int {
	n := 0
	if d.HTTPReachable {
		n++
	}
	if d.WSReachable {
		n++
	}
	return n
}

func wsReachable(ctx context.Context, rawURL string) bool {
	wsURL, err := url.Parse(rawURL)
	if err != nil || (wsURL.Scheme != "ws" && wsURL.Scheme != "wss") {
		return false
	}
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
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
