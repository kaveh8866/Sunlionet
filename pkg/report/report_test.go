package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestGenerate_MissingInputs(t *testing.T) {
	r := Generate("", nil)
	if r.Schema == "" {
		t.Fatalf("expected schema")
	}
	if len(r.Errors) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestGenerate_RedactsProfileIDs(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "profiles.enc")
	key := []byte("0123456789abcdef0123456789abcdef")
	store, err := profile.NewStore(storePath, key)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save([]profile.Profile{
		{ID: "p1", Family: profile.FamilyReality, Enabled: true},
		{ID: "p2", Family: profile.FamilyHysteria2, Enabled: true},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	rawState, _ := json.Marshal(map[string]any{
		"profiles_loaded":     2,
		"selected_profile_id": "p1",
		"selection_reason":    "score=10 source=policy confidence=0.9",
		"sing_box_pid":        0,
		"updated_at_unix":     123,
	})
	if err := os.WriteFile(filepath.Join(tmp, "state.json"), rawState, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	r := Generate(tmp, key)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) == "" {
		t.Fatalf("expected json")
	}
	if strings.Contains(string(b), "p1") || strings.Contains(string(b), "p2") {
		t.Fatalf("report must not contain profile IDs")
	}
	if r.Summary.SelectedFamily != string(profile.FamilyReality) {
		t.Fatalf("expected selected family")
	}
}
