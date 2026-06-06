// SPDX-License-Identifier: LGPL-3.0-only

package state

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store := NewStore(path, 3)
	key := Key("qq", "private:10001", "10001")

	if err := store.Put(key, Session{ID: "abc", Turns: 1}); err != nil {
		t.Fatalf("put failed: %v", err)
	}

	reloaded := NewStore(path, 3)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	session, ok := reloaded.Get(key)
	if !ok {
		t.Fatal("session was not loaded")
	}
	if session.ID != "abc" || session.Turns != 1 {
		t.Fatalf("unexpected session: %#v", session)
	}
}

func TestStoreResetsAtMaxTurns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store := NewStore(path, 2)
	key := Key("qq", "private:10001", "10001")

	if err := store.Put(key, Session{ID: "abc", Turns: 1}); err != nil {
		t.Fatalf("put failed: %v", err)
	}
	session, err := store.Increment(key)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if session.ID != "" || session.Turns != 0 {
		t.Fatalf("session was not reset: %#v", session)
	}
}

func TestStoreClearPersistsEmptySessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store := NewStore(path, 3)
	key := Key("qq", "private:10001", "10001")

	if err := store.Put(key, Session{ID: "abc", Turns: 1}); err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	reloaded := NewStore(path, 3)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if _, ok := reloaded.Get(key); ok {
		t.Fatal("session was not cleared")
	}
}
