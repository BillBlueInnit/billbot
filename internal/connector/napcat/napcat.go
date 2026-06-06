// SPDX-License-Identifier: LGPL-3.0-only

package napcat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"billbot/internal/config"
	"billbot/internal/connector"

	"github.com/gorilla/websocket"
)

type Connector struct {
	cfg    config.NapCatConfig
	mu     sync.Mutex
	cancel context.CancelFunc
	conn   *websocket.Conn
}

func New(cfg config.NapCatConfig) *Connector {
	return &Connector{cfg: cfg}
}

func (c *Connector) Name() string                 { return "napcat" }
func (c *Connector) Platform() connector.Platform { return connector.PlatformQQ }

func (c *Connector) Status() (connector.Status, error) {
	status := connector.Status{Name: c.Name(), Platform: c.Platform()}
	client := http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, c.cfg.HTTP+"/get_status", nil)
	if err != nil {
		status.Message = err.Error()
		return status, nil
	}
	authorize(req, c.cfg.EffectiveHTTPToken())
	resp, err := client.Do(req)
	if err != nil {
		status.Message = err.Error()
		return status, nil
	}
	defer resp.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	status.Connected = resp.StatusCode >= 200 && resp.StatusCode < 300
	status.Message = fmt.Sprintf("http %d", resp.StatusCode)
	return status, nil
}

func (c *Connector) Start(onMessage func(connector.Message)) error {
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()

	ws, _, err := websocket.DefaultDialer.Dial(wsURLWithToken(c.cfg.WS, c.cfg.EffectiveWSToken()), nil)
	if err != nil {
		c.mu.Lock()
		if c.conn == nil {
			c.cancel = nil
		}
		c.mu.Unlock()
		cancel()
		return err
	}

	c.mu.Lock()
	c.conn = ws
	c.mu.Unlock()

	go c.readLoop(ctx, ws, onMessage)
	return nil
}

func (c *Connector) Stop() error {
	c.mu.Lock()
	cancel := c.cancel
	ws := c.conn
	c.cancel = nil
	c.conn = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ws != nil {
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		return ws.Close()
	}
	return nil
}

func (c *Connector) Send(chatID string, text string) error {
	if strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("chat id is required")
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if groupID, ok := strings.CutPrefix(chatID, "group:"); ok {
		return c.SendGroup(groupID, text)
	}
	if privateID, ok := strings.CutPrefix(chatID, "private:"); ok {
		return c.SendPrivate(privateID, text)
	}
	return c.SendPrivate(chatID, text)
}

func (c *Connector) SendPrivate(userID string, text string) error {
	return c.postAction("/send_private_msg", map[string]any{
		"user_id": asNumberOrString(userID),
		"message": text,
	})
}

func (c *Connector) SendGroup(groupID string, text string) error {
	return c.postAction("/send_group_msg", map[string]any{
		"group_id": asNumberOrString(groupID),
		"message":  text,
	})
}

func (c *Connector) SendFile(chatID string, filePath string, name string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("file path is required")
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(filePath)
	}
	if groupID, ok := strings.CutPrefix(chatID, "group:"); ok {
		return c.postAction("/upload_group_file", map[string]any{
			"group_id": asNumberOrString(groupID),
			"file":     filePath,
			"name":     name,
		})
	}
	if privateID, ok := strings.CutPrefix(chatID, "private:"); ok {
		return c.postAction("/upload_private_file", map[string]any{
			"user_id": asNumberOrString(privateID),
			"file":    filePath,
			"name":    name,
		})
	}
	return c.postAction("/upload_private_file", map[string]any{
		"user_id": asNumberOrString(chatID),
		"file":    filePath,
		"name":    name,
	})
}

func (c *Connector) readLoop(ctx context.Context, ws *websocket.Conn, onMessage func(connector.Message)) {
	defer func() {
		c.mu.Lock()
		if c.conn == ws {
			c.conn = nil
			c.cancel = nil
		}
		c.mu.Unlock()
		_ = ws.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, raw, err := ws.ReadMessage()
		if err != nil {
			return
		}
		msg, ok := ParseMessageEvent(raw)
		if !ok {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			onMessage(msg)
		}()
	}
}

func (c *Connector) postAction(path string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, c.cfg.HTTP+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	authorize(req, c.cfg.EffectiveHTTPToken())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("napcat %s failed: http %d %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func authorize(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func wsURLWithToken(rawURL string, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := parsed.Query()
	if q.Get("access_token") == "" {
		q.Set("access_token", token)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func ParseMessageEvent(raw []byte) (connector.Message, bool) {
	var event struct {
		PostType    string          `json:"post_type"`
		MessageType string          `json:"message_type"`
		SelfID      json.RawMessage `json:"self_id"`
		UserID      json.RawMessage `json:"user_id"`
		GroupID     json.RawMessage `json:"group_id"`
		Message     any             `json:"message"`
		RawMessage  string          `json:"raw_message"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return connector.Message{}, false
	}
	if event.PostType != "message" {
		return connector.Message{}, false
	}
	text, attachments := messageContent(event.Message)
	if event.RawMessage != "" {
		text = event.RawMessage
	}
	if strings.TrimSpace(text) == "" && len(attachments) == 0 {
		return connector.Message{}, false
	}

	userID := rawID(event.UserID)
	groupID := rawID(event.GroupID)
	private := event.MessageType != "group"
	chatID := "private:" + userID
	if !private {
		chatID = "group:" + groupID
	}
	return connector.Message{
		Platform:    connector.PlatformQQ,
		BotID:       rawID(event.SelfID),
		ChatID:      chatID,
		UserID:      userID,
		GroupID:     groupID,
		Private:     private,
		Text:        text,
		Attachments: attachments,
		Raw:         append([]byte(nil), raw...),
	}, true
}

func messageContent(v any) (string, []connector.Attachment) {
	switch msg := v.(type) {
	case string:
		return msg, nil
	case []any:
		var b strings.Builder
		var attachments []connector.Attachment
		for _, item := range msg {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			typ, _ := part["type"].(string)
			data, _ := part["data"].(map[string]any)
			switch typ {
			case "text":
				text, _ := data["text"].(string)
				b.WriteString(text)
			case "image", "file", "record", "video":
				attachments = append(attachments, attachmentFromSegment(typ, data))
			}
		}
		return b.String(), attachments
	default:
		return "", nil
	}
}

func attachmentFromSegment(typ string, data map[string]any) connector.Attachment {
	att := connector.Attachment{Type: typ}
	for _, key := range []string{"url", "file", "file_id", "name", "summary"} {
		value, _ := data[key].(string)
		if value == "" {
			continue
		}
		switch key {
		case "url":
			att.URL = value
		case "file", "file_id":
			if att.File == "" {
				att.File = value
			}
		case "name":
			att.Name = value
		case "summary":
			att.Summary = value
		}
	}
	if att.Name == "" {
		att.Name, _ = data["filename"].(string)
	}
	return att
}

func messageText(v any) string {
	text, _ := messageContent(v)
	return text
}

func rawID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatInt(n, 10)
	}
	return strings.Trim(string(raw), `"`)
}

func asNumberOrString(v string) any {
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	return v
}
