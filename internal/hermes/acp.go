// SPDX-License-Identifier: LGPL-3.0-only

package hermes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var acpPool = struct {
	sync.Mutex
	clients map[string]*acpClient
}{clients: map[string]*acpClient{}}

func askPersistentACP(ctx context.Context, command string, prompt string, opts Options) (string, string, error) {
	if command == "" {
		command = "hermes"
	}
	key := acpPoolKey(command, opts)
	client, err := acpClientFor(ctx, key, command, opts)
	if err != nil {
		return "", "", err
	}
	reply, sessionID, err := client.ask(ctx, prompt, opts)
	if err != nil {
		dropACPClient(key, client)
		client.stop()
	}
	return reply, sessionID, err
}

func acpPoolKey(command string, opts Options) string {
	return strings.Join([]string{
		command,
		opts.SandboxDir,
		opts.SecurityMode,
		opts.SandboxBackend,
		strings.Join(opts.SandboxCommand, "\x01"),
		opts.SandboxDockerImage,
		strings.Join(opts.SandboxDockerArgs, "\x01"),
		opts.Provider,
		opts.Model,
	}, "\x00")
}

func acpClientFor(ctx context.Context, key string, command string, opts Options) (*acpClient, error) {
	acpPool.Lock()
	client := acpPool.clients[key]
	if client != nil && client.isRunning() {
		acpPool.Unlock()
		return client, nil
	}
	client = newACPClient(command)
	acpPool.clients[key] = client
	acpPool.Unlock()
	if err := client.start(ctx, opts); err != nil {
		acpPool.Lock()
		if acpPool.clients[key] == client {
			delete(acpPool.clients, key)
		}
		acpPool.Unlock()
		return nil, err
	}
	return client, nil
}

func dropACPClient(key string, client *acpClient) {
	acpPool.Lock()
	if acpPool.clients[key] == client {
		delete(acpPool.clients, key)
	}
	acpPool.Unlock()
}

func ClosePersistentACP() {
	acpPool.Lock()
	clients := make([]*acpClient, 0, len(acpPool.clients))
	for key, client := range acpPool.clients {
		clients = append(clients, client)
		delete(acpPool.clients, key)
	}
	acpPool.Unlock()
	for _, client := range clients {
		client.stop()
	}
}

type acpClient struct {
	command string
	cmd     *exec.Cmd
	stdin   io.WriteCloser

	nextID  atomic.Int64
	running atomic.Bool

	writeMu sync.Mutex
	askMu   sync.Mutex
	pending map[int64]chan acpResponse

	currentMu     sync.Mutex
	currentPrompt int64
	currentText   strings.Builder
	currentTouch  chan struct{}
}

type acpResponse struct {
	Result json.RawMessage
	Error  *acpError
}

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newACPClient(command string) *acpClient {
	c := &acpClient{command: command, pending: map[int64]chan acpResponse{}}
	return c
}

func (c *acpClient) isRunning() bool {
	return c.running.Load()
}

func (c *acpClient) stop() {
	c.running.Store(false)
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

func (c *acpClient) start(ctx context.Context, opts Options) error {
	argv := hermesArgv(c.command, []string{"acp", "--accept-hooks"})
	argv = applySandboxBackendArgv(argv, opts)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	applyHermesRuntime(cmd, opts)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.stdin = stdin
	c.running.Store(true)
	go c.readStdout(stdout)
	go c.readStderr(stderr)
	go func() {
		err := cmd.Wait()
		c.running.Store(false)
		if err != nil {
			log.Printf("hermes acp exited: %v", err)
		}
	}()
	_, err = c.call(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{"readTextFile": false, "writeTextFile": false},
		},
		"clientInfo": map[string]any{
			"name":    "billbot",
			"title":   "BillBot",
			"version": "0.1.0",
		},
	})
	return err
}

func (c *acpClient) ask(ctx context.Context, prompt string, opts Options) (string, string, error) {
	c.askMu.Lock()
	defer c.askMu.Unlock()

	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		newSession, err := c.newSession(ctx, opts)
		if err != nil {
			return "", "", err
		}
		sessionID = newSession
	}

	promptID := c.nextID.Add(1)
	c.currentMu.Lock()
	c.currentPrompt = promptID
	c.currentText.Reset()
	c.currentTouch = make(chan struct{}, 1)
	touch := c.currentTouch
	c.currentMu.Unlock()

	raw, err := c.callWithIDOrText(ctx, promptID, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    acpPromptContent(prompt, opts.Attachments),
	}, touch, 1200*time.Millisecond)
	c.currentMu.Lock()
	text := strings.TrimSpace(c.currentText.String())
	c.currentPrompt = 0
	c.currentText.Reset()
	c.currentTouch = nil
	c.currentMu.Unlock()
	if err != nil {
		return text, sessionID, err
	}
	if text == "" {
		text = strings.TrimSpace(extractACPTextFromJSON(raw))
	}
	if text == "" {
		return "", sessionID, fmt.Errorf("hermes acp returned no final text")
	}
	return text, sessionID, nil
}

func acpPromptContent(prompt string, attachments []Attachment) []map[string]any {
	blocks := []map[string]any{{"type": "text", "text": prompt}}
	for _, att := range attachments {
		block, ok := acpAttachmentBlock(att)
		if ok {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func acpAttachmentBlock(att Attachment) (map[string]any, bool) {
	typ := strings.ToLower(strings.TrimSpace(att.Type))
	url := strings.TrimSpace(att.URL)
	file := strings.TrimSpace(att.File)
	name := strings.TrimSpace(att.Name)
	switch typ {
	case "image":
		if url != "" {
			block := map[string]any{"type": "image", "url": url}
			if name != "" {
				block["name"] = name
			}
			return block, true
		}
		if file != "" {
			block := map[string]any{"type": "image", "path": file}
			if name != "" {
				block["name"] = name
			}
			return block, true
		}
	case "file", "record", "video":
		if url != "" {
			block := map[string]any{"type": "file", "url": url}
			if name != "" {
				block["name"] = name
			}
			return block, true
		}
		if file != "" {
			block := map[string]any{"type": "file", "path": file}
			if name != "" {
				block["name"] = name
			}
			return block, true
		}
	}
	return nil, false
}

func (c *acpClient) newSession(ctx context.Context, opts Options) (string, error) {
	params := map[string]any{"mcpServers": []any{}}
	if opts.Model != "" {
		modelID := opts.Model
		if opts.Provider != "" && !strings.Contains(modelID, ":") && !strings.Contains(modelID, "/") {
			modelID = opts.Provider + ":" + opts.Model
		}
		params["modelId"] = modelID
	}
	raw, err := c.call(ctx, "session/new", params)
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("hermes acp session/new returned empty session id")
	}
	return out.SessionID, nil
}

func (c *acpClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.callWithID(ctx, c.nextID.Add(1), method, params)
}

func (c *acpClient) callWithID(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
	return c.callWithIDOrText(ctx, id, method, params, nil, 0)
}

func (c *acpClient) callWithIDOrText(ctx context.Context, id int64, method string, params any, touch <-chan struct{}, idleTimeout time.Duration) (json.RawMessage, error) {
	ch := make(chan acpResponse, 1)
	c.writeMu.Lock()
	c.pending[id] = ch
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	b, err := json.Marshal(msg)
	if err == nil {
		_, err = c.stdin.Write(append(b, '\n'))
	}
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(id)
		return nil, err
	}
	var timer *time.Timer
	var timerC <-chan time.Time
	seenText := false
	if touch != nil && idleTimeout > 0 {
		timer = time.NewTimer(idleTimeout)
		if !timer.Stop() {
			<-timer.C
		}
		defer timer.Stop()
	}
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("hermes acp %s failed: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	case <-touch:
		if timer != nil {
			seenText = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idleTimeout)
			timerC = timer.C
		}
		return c.waitForResponseOrIdle(ctx, method, id, ch, touch, timer, timerC, seenText, idleTimeout)
	case <-ctx.Done():
		c.removePending(id)
		c.stop()
		return nil, ctx.Err()
	}
}

func (c *acpClient) waitForResponseOrIdle(ctx context.Context, method string, id int64, ch <-chan acpResponse, touch <-chan struct{}, timer *time.Timer, timerC <-chan time.Time, seenText bool, idleTimeout time.Duration) (json.RawMessage, error) {
	for {
		select {
		case resp := <-ch:
			if resp.Error != nil {
				return nil, fmt.Errorf("hermes acp %s failed: %s", method, resp.Error.Message)
			}
			return resp.Result, nil
		case <-touch:
			seenText = true
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idleTimeout)
				timerC = timer.C
			}
		case <-timerC:
			if seenText {
				c.removePending(id)
				return nil, nil
			}
		case <-ctx.Done():
			c.removePending(id)
			c.stop()
			return nil, ctx.Err()
		}
	}
}

func (c *acpClient) removePending(id int64) {
	c.writeMu.Lock()
	delete(c.pending, id)
	c.writeMu.Unlock()
}

func (c *acpClient) readStdout(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var msg struct {
			ID     *int64          `json:"id,omitempty"`
			Method string          `json:"method,omitempty"`
			Params json.RawMessage `json:"params,omitempty"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *acpError       `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID != nil {
			c.writeMu.Lock()
			ch := c.pending[*msg.ID]
			delete(c.pending, *msg.ID)
			c.writeMu.Unlock()
			if ch != nil {
				ch <- acpResponse{Result: msg.Result, Error: msg.Error}
			}
			continue
		}
		if msg.Method == "session/update" {
			c.handleSessionUpdate(msg.Params)
		}
	}
}

func (c *acpClient) readStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			log.Printf("hermes acp: %s", line)
		}
	}
}

func (c *acpClient) handleSessionUpdate(raw json.RawMessage) {
	var params struct {
		Update map[string]any `json:"update"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return
	}
	updateType, _ := params.Update["sessionUpdate"].(string)
	text := strings.TrimSpace(extractACPText(params.Update))
	if text == "" {
		return
	}
	if updateType != "" && strings.Contains(strings.ToLower(updateType), "user") {
		return
	}
	if updateType != "" && !strings.Contains(strings.ToLower(updateType), "agent") && !strings.Contains(strings.ToLower(updateType), "message") {
		return
	}
	c.currentMu.Lock()
	if c.currentPrompt != 0 {
		if c.currentText.Len() > 0 {
			c.currentText.WriteString("\n")
		}
		c.currentText.WriteString(text)
		if c.currentTouch != nil {
			select {
			case c.currentTouch <- struct{}{}:
			default:
			}
		}
	}
	c.currentMu.Unlock()
}

func extractACPText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var parts []string
		for _, item := range x {
			if text := extractACPText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content"} {
			if text := extractACPText(x[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractACPTextFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return extractACPText(v)
}
