package mobilebridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestStartAgent_RendersConfigAndUpdatesStatus(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "reality.json"), []byte(`{"outbounds":[{"type":"direct","tag":"proxy"}]}`), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	store, err := profile.NewStore(filepath.Join(stateDir, "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	p := profile.Profile{
		ID:      "p1",
		Family:  profile.FamilyReality,
		Enabled: true,
		Endpoint: profile.Endpoint{
			Host: "127.0.0.1",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Source: profile.SourceInfo{
			Source:     "test",
			TrustLevel: 80,
			ImportedAt: time.Now().Unix(),
		},
	}
	if err := store.Save([]profile.Profile{p}); err != nil {
		t.Fatalf("save store: %v", err)
	}

	cfg := AgentConfig{
		StateDir:        stateDir,
		MasterKey:       "0123456789abcdef0123456789abcdef",
		TemplatesDir:    templatesDir,
		PollIntervalSec: 10,
	}
	raw, _ := json.Marshal(cfg)
	StartAgent(string(raw))
	defer StopAgent()

	time.Sleep(250 * time.Millisecond)
	var st AgentStatus
	if err := json.Unmarshal([]byte(GetStatus()), &st); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if st.CurrentProfile != "p1" {
		t.Fatalf("expected current profile p1, got %q (status=%+v)", st.CurrentProfile, st)
	}
	if st.LastError != "" {
		t.Fatalf("unexpected status error: %s", st.LastError)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "runtime", "config.json")); err != nil {
		t.Fatalf("expected config path: %v", err)
	}
}

func TestStartAgent_InvalidConfig(t *testing.T) {
	StartAgent(`{"state_dir":"","master_key":"short","templates_dir":""}`)
	defer StopAgent()
	var st AgentStatus
	if err := json.Unmarshal([]byte(GetStatus()), &st); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if st.LastError == "" {
		t.Fatalf("expected error status")
	}
}
