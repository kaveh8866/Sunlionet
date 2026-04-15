package comms

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveLoad_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "comms.enc")

	store, err := NewStore(dbPath, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	state := NewState(RoleInside)
	state.InstalledApps = []InstalledApp{
		{App: AppSession, Installed: true, Enabled: true},
		{App: AppBriar, Installed: true, Enabled: true},
	}
	state.Contacts = []TrustedContact{
		{
			ID:    "c1",
			Alias: "helper-one",
			Trust: TrustTrusted,
			Channels: []ContactChannel{
				{App: AppSession, IdentifierHint: "05ab.."},
			},
		},
	}
	state.BundleHistory = []BundleImportRecord{
		{
			BundleID:   "b1",
			SourceApp:  AppSession,
			ReceivedAt: time.Now().Unix(),
			Status:     "accepted",
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.Load(RoleInside)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Role != RoleInside {
		t.Fatalf("unexpected role: %s", loaded.Role)
	}
	if len(loaded.Contacts) != 1 || loaded.Contacts[0].Alias != "helper-one" {
		t.Fatalf("unexpected contacts: %#v", loaded.Contacts)
	}
}

func TestStore_Save_DoesNotLeakPlaintext(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "comms.enc")

	store, err := NewStore(dbPath, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	state := NewState(RoleOutside)
	state.Contacts = []TrustedContact{
		{
			ID:    "outside-1",
			Alias: "sensitive-helper",
			Trust: TrustTrusted,
			Channels: []ContactChannel{
				{App: AppSimpleX, IdentifierHint: "simplex://invitation-very-secret"},
			},
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	for _, needle := range [][]byte{
		[]byte("sensitive-helper"),
		[]byte("simplex://invitation-very-secret"),
	} {
		if bytes.Contains(raw, needle) {
			t.Fatalf("encrypted comms store leaked plaintext token: %q", needle)
		}
	}
}

func TestState_PruneHistory(t *testing.T) {
	s := NewState(RoleInside)
	now := time.Now()
	for i := 0; i < MaxBundleHistory+20; i++ {
		receivedAt := now.Unix() - int64(i*60)
		s.BundleHistory = append(s.BundleHistory, BundleImportRecord{
			BundleID:   "b",
			SourceApp:  AppSession,
			ReceivedAt: receivedAt,
			Status:     "accepted",
		})
	}
	s.BundleHistory = append(s.BundleHistory, BundleImportRecord{
		BundleID:   "old",
		SourceApp:  AppBriar,
		ReceivedAt: now.Add(-(BundleHistoryRetentionDays + 10) * 24 * time.Hour).Unix(),
		Status:     "accepted",
	})

	s.Prune(now)
	if len(s.BundleHistory) > MaxBundleHistory {
		t.Fatalf("expected <= %d records, got %d", MaxBundleHistory, len(s.BundleHistory))
	}
	for _, r := range s.BundleHistory {
		if r.BundleID == "old" {
			t.Fatalf("expected old record to be pruned")
		}
	}
}

func TestState_Validate_RejectsDuplicateContactID(t *testing.T) {
	s := NewState(RoleInside)
	s.Contacts = []TrustedContact{
		{
			ID:       "same",
			Alias:    "a",
			Trust:    TrustTrusted,
			Channels: []ContactChannel{{App: AppSession}},
		},
		{
			ID:       "same",
			Alias:    "b",
			Trust:    TrustTrusted,
			Channels: []ContactChannel{{App: AppBriar}},
		},
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("expected duplicate contact id validation error")
	}
}
