package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SaveAndLoad(t *testing.T) {
	// Setup a temporary directory for the test DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_store.enc")
	masterKey := "0123456789abcdef0123456789abcdef" // 32 bytes

	store, err := NewStore(dbPath, masterKey)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create dummy profiles
	profiles := []Profile{
		{
			ID:      "test-profile-1",
			Family:  FamilyReality,
			Enabled: true,
			Endpoint: Endpoint{
				Host: "192.168.1.100",
				Port: 443,
			},
		},
		{
			ID:      "test-profile-2",
			Family:  FamilyHysteria2,
			Enabled: false,
			Endpoint: Endpoint{
				Host: "10.0.0.5",
				Port: 8443,
			},
		},
	}

	// Save profiles
	err = store.Save(profiles)
	if err != nil {
		t.Fatalf("Failed to save profiles: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file was not created at %s", dbPath)
	}

	// Load profiles
	loadedProfiles, err := store.Load()
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}

	// Verify loaded data
	if len(loadedProfiles) != len(profiles) {
		t.Fatalf("Expected %d profiles, got %d", len(profiles), len(loadedProfiles))
	}

	if loadedProfiles[0].ID != "test-profile-1" {
		t.Errorf("Expected ID 'test-profile-1', got '%s'", loadedProfiles[0].ID)
	}

	if loadedProfiles[1].Family != FamilyHysteria2 {
		t.Errorf("Expected Family '%s', got '%s'", FamilyHysteria2, loadedProfiles[1].Family)
	}
}

func TestStore_InvalidKeyLength(t *testing.T) {
	_, err := NewStore("dummy.enc", "short-key")
	if err == nil {
		t.Fatal("Expected error for invalid key length, got nil")
	}
}

func TestStore_LoadEmpty(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty_store.enc")
	store, _ := NewStore(dbPath, "0123456789abcdef0123456789abcdef")

	profiles, err := store.Load()
	if err != nil {
		t.Fatalf("Expected no error when loading non-existent store, got: %v", err)
	}

	if len(profiles) != 0 {
		t.Fatalf("Expected empty profiles list, got length %d", len(profiles))
	}
}
