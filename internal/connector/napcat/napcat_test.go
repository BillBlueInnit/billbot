// SPDX-License-Identifier: LGPL-3.0-only

package napcat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"billbot/internal/config"
	"billbot/internal/connector"

	"github.com/gorilla/websocket"
)

func TestParsePrivateMessageEvent(t *testing.T) {
	raw := []byte(`{"post_type":"message","message_type":"private","self_id":12345,"user_id":67890,"raw_message":"hello"}`)

	msg, ok := ParseMessageEvent(raw)
	if !ok {
		t.Fatal("event was not parsed")
	}
	if !msg.Private {
		t.Fatalf("Private = false, want true")
	}
	if msg.ChatID != "private:67890" {
		t.Fatalf("ChatID = %q, want private:67890", msg.ChatID)
	}
	if msg.BotID != "12345" || msg.UserID != "67890" || msg.Text != "hello" {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

func TestParseGroupMessageEvent(t *testing.T) {
	raw := []byte(`{"post_type":"message","message_type":"group","self_id":"12345","user_id":"67890","group_id":"222","message":[{"type":"text","data":{"text":"hello group"}}]}`)

	msg, ok := ParseMessageEvent(raw)
	if !ok {
		t.Fatal("event was not parsed")
	}
	if msg.Private {
		t.Fatalf("Private = true, want false")
	}
	if msg.ChatID != "group:222" || msg.GroupID != "222" {
		t.Fatalf("unexpected group routing: %#v", msg)
	}
	if msg.Text != "hello group" {
		t.Fatalf("Text = %q, want hello group", msg.Text)
	}
}

func TestParseIgnoresNonMessageEvent(t *testing.T) {
	if _, ok := ParseMessageEvent([]byte(`{"post_type":"notice"}`)); ok {
		t.Fatal("non-message event was parsed")
	}
}

func TestSendPrivateAndGroupUseOneBotEndpoints(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.Path, body: body})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conn := New(configForTest(server.URL))
	if err := conn.SendPrivate("10001", "hello"); err != nil {
		t.Fatalf("send private: %v", err)
	}
	if err := conn.SendGroup("20002", "group hello"); err != nil {
		t.Fatalf("send group: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	if requests[0].path != "/send_private_msg" || requests[0].body["message"] != "hello" {
		t.Fatalf("unexpected private request: %#v", requests[0])
	}
	if requests[0].body["user_id"] != float64(10001) {
		t.Fatalf("private user_id = %#v", requests[0].body["user_id"])
	}
	if requests[1].path != "/send_group_msg" || requests[1].body["message"] != "group hello" {
		t.Fatalf("unexpected group request: %#v", requests[1])
	}
	if requests[1].body["group_id"] != float64(20002) {
		t.Fatalf("group_id = %#v", requests[1].body["group_id"])
	}
}

func TestStatusReturnsDisconnectedWhenNapCatUnavailable(t *testing.T) {
	conn := New(configForTest("http://127.0.0.1:1"))
	status, err := conn.Status()
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if status.Connected {
		t.Fatalf("Connected = true, want false")
	}
	if status.Message == "" {
		t.Fatal("status message is empty")
	}
}

func TestStartConnectsWebSocketAndReceivesMessage(t *testing.T) {
	upgrader := websocket.Upgrader{}
	received := make(chan connector.Message, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer ws.Close()
		err = ws.WriteMessage(websocket.TextMessage, []byte(`{"post_type":"message","message_type":"private","self_id":12345,"user_id":67890,"raw_message":"hello"}`))
		if err != nil {
			t.Errorf("write message: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	conn := New(config.NapCatConfig{
		HTTP: "http://127.0.0.1:1",
		WS:   "ws" + strings.TrimPrefix(server.URL, "http"),
	})
	if err := conn.Start(func(msg connector.Message) {
		received <- msg
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer conn.Stop()

	select {
	case msg := <-received:
		if msg.ChatID != "private:67890" || msg.Text != "hello" {
			t.Fatalf("unexpected message: %#v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func configForTest(httpURL string) config.NapCatConfig {
	return config.NapCatConfig{HTTP: httpURL, WS: "ws://127.0.0.1:1"}
}
