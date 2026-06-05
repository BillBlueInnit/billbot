// SPDX-License-Identifier: LGPL-3.0-only

package connector

type Platform string

const (
	PlatformQQ Platform = "qq"
)

type Message struct {
	Platform Platform `json:"platform"`
	BotID    string   `json:"bot_id"`
	ChatID   string   `json:"chat_id"`
	UserID   string   `json:"user_id"`
	GroupID  string   `json:"group_id,omitempty"`
	Private  bool     `json:"private"`
	Text     string   `json:"text"`
	Raw      []byte   `json:"raw,omitempty"`
}

type Status struct {
	Name      string   `json:"name"`
	Platform  Platform `json:"platform"`
	Connected bool     `json:"connected"`
	LoginURL  string   `json:"login_url,omitempty"`
	QRCode    string   `json:"qr_code,omitempty"`
	Message   string   `json:"message,omitempty"`
}

type Connector interface {
	Name() string
	Platform() Platform
	Status() (Status, error)
	Start(onMessage func(Message)) error
	Stop() error
	Send(chatID string, text string) error
}
