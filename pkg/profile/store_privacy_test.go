package profile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_Save_DoesNotLeakPlaintextSecrets(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "store.enc")
	masterKey := []byte("0123456789abcdef0123456789abcdef")

	store, err := NewStore(dbPath, masterKey)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	p := Profile{
		ID:      "sensitive-profile-01",
		Family:  FamilyReality,
		Enabled: true,
		Endpoint: Endpoint{
			Host: "secret.example.com",
			Port: 443,
		},
	}
	if err := store.Save([]Profile{p}); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	for _, needle := range [][]byte{
		[]byte("secret.example.com"),
		[]byte("sensitive-profile-01"),
	} {
		if bytes.Contains(raw, needle) {
			t.Fatalf("encrypted store leaked plaintext token: %q", needle)
		}
	}
}
