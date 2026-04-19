package aipolicy

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	s, err := NewStore(filepath.Join(dir, "aipolicy.enc"), key)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	p := &Policy{
		Grants: []Grant{
			NewGrant(Scope{Type: ScopeChat, ID: "c1"}, []Action{ActionSummarize}, 10*time.Minute),
		},
	}
	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(got.Grants))
	}
	if got.Grants[0].Scope.ID != "c1" {
		t.Fatalf("scope mismatch")
	}
}
