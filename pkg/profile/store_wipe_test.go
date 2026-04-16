package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_WipeOnSuspicion_RemovesEncryptedStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "store.enc")
	masterKey := []byte("0123456789abcdef0123456789abcdef")

	store, err := NewStore(dbPath, masterKey)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := store.Save([]Profile{{ID: "p1", Family: FamilyReality, Enabled: true}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected store file to exist: %v", err)
	}

	if err := store.WipeOnSuspicion(); err != nil {
		t.Fatalf("wipe: %v", err)
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected store file to be deleted, got: %v", err)
	}
}
