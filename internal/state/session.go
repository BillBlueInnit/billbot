// SPDX-License-Identifier: LGPL-3.0-only

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Session struct {
	ID    string `json:"id"`
	Turns int    `json:"turns"`
}

type Store struct {
	mu       sync.Mutex
	path     string
	maxTurns int
	items    map[string]Session
}

func NewStore(path string, maxTurns int) *Store {
	if maxTurns <= 0 {
		maxTurns = 120
	}
	return &Store{
		path:     path,
		maxTurns: maxTurns,
		items:    map[string]Session{},
	}
}

func Key(platform, chatID, userID string) string {
	return platform + ":" + chatID + ":" + userID
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var items map[string]Session
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	if items == nil {
		items = map[string]Session{}
	}
	s.items = items
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) Get(key string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	return item, ok
}

func (s *Store) Put(key string, item Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.Turns >= s.maxTurns {
		item.ID = ""
		item.Turns = 0
	}
	s.items[key] = item
	return s.saveLocked()
}

func (s *Store) Increment(key string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.items[key]
	item.Turns++
	if item.Turns >= s.maxTurns {
		item.ID = ""
		item.Turns = 0
	}
	s.items[key] = item
	return item, s.saveLocked()
}

func (s *Store) Reset(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return s.saveLocked()
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = map[string]Session{}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0600)
}
