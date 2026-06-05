// SPDX-License-Identifier: LGPL-3.0-only

package napcat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	resp, err := client.Get(c.cfg.HTTP + "/get_status")
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

	ws, _, err := websocket.DefaultDialer.Dial(c.cfg.WS, nil)
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
	resp, err := client.Post(c.cfg.HTTP+path, "application/json", bytes.NewReader(body))
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
	text := event.RawMessage
	if text == "" {
		text = messageText(event.Message)
	}
	if strings.TrimSpace(text) == "" {
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
		Platform: connector.PlatformQQ,
		BotID:    rawID(event.SelfID),
		ChatID:   chatID,
		UserID:   userID,
		GroupID:  groupID,
		Private:  private,
		Text:     text,
		Raw:      append([]byte(nil), raw...),
	}, true
}

func messageText(v any) string {
	switch msg := v.(type) {
	case string:
		return msg
	case []any:
		var b strings.Builder
		for _, item := range msg {
			part, ok := item.(map[string]any)
			if !ok || part["type"] != "text" {
				continue
			}
			data, _ := part["data"].(map[string]any)
			text, _ := data["text"].(string)
			b.WriteString(text)
		}
		return b.String()
	default:
		return ""
	}
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
