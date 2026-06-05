// SPDX-License-Identifier: LGPL-3.0-only

package loginqr

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"billbot/internal/config"

	qrcode "github.com/skip2/go-qrcode"
)

type Result struct {
	Content string `json:"content"`
	Render  string `json:"render"`
	DataURL string `json:"data_url,omitempty"`
}

type StatusResult struct {
	Output string `json:"output"`
}

func Fetch(ctx context.Context, cfg config.LoginConfig) (Result, error) {
	if len(cfg.QRCommand) == 0 || strings.TrimSpace(cfg.QRCommand[0]) == "" {
		return Result{}, fmt.Errorf("login.qr_command is not configured")
	}
	timeout := time.Duration(cfg.QRTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cfg.QRCommand[0], cfg.QRCommand[1:]...)
	out, err := cmd.CombinedOutput()
	content := strings.TrimSpace(string(out))
	if err != nil {
		if content != "" {
			return Result{}, fmt.Errorf("%w: %s", err, content)
		}
		return Result{}, err
	}
	if content == "" {
		return Result{}, fmt.Errorf("login.qr_command returned empty output")
	}
	render, dataURL, err := Render(content)
	if err != nil {
		return Result{}, err
	}
	return Result{Content: content, Render: render, DataURL: dataURL}, nil
}

func Status(ctx context.Context, cfg config.LoginConfig) (StatusResult, error) {
	if len(cfg.StatusCommand) == 0 || strings.TrimSpace(cfg.StatusCommand[0]) == "" {
		return StatusResult{}, fmt.Errorf("login.status_command is not configured")
	}
	timeout := time.Duration(cfg.QRTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cfg.StatusCommand[0], cfg.StatusCommand[1:]...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text != "" {
			return StatusResult{}, fmt.Errorf("%w: %s", err, text)
		}
		return StatusResult{}, err
	}
	return StatusResult{Output: text}, nil
}

func Render(content string) (render string, dataURL string, err error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", "", err
	}
	bits := qr.Bitmap()
	cols, rows := terminalSize()
	neededCols := (len(bits) + 4) * 2
	neededRows := len(bits) + 4
	dataURL, err = dataURLFor(qr)
	if err != nil {
		return "", "", err
	}
	if cols > 0 && rows > 0 && (neededCols > cols || neededRows > rows-6) {
		return dataURL, dataURL, nil
	}

	var b strings.Builder
	for y := -2; y < len(bits)+2; y++ {
		for x := -2; x < len(bits)+2; x++ {
			dark := false
			if y >= 0 && y < len(bits) && x >= 0 && x < len(bits[y]) {
				dark = bits[y][x]
			}
			if dark {
				b.WriteString("##")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), dataURL, nil
}

func dataURLFor(qr *qrcode.QRCode) (string, error) {
	png, err := qr.PNG(512)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func terminalSize() (int, int) {
	cols, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	rows, _ := strconv.Atoi(os.Getenv("LINES"))
	return cols, rows
}
