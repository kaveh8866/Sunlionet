package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/llm"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to get caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type errorAdvisor struct{ err error }

func (a *errorAdvisor) ProposeAction(string, profile.Profile, []profile.Profile, []detector.Event) (policy.Action, error) {
	return policy.Action{}, a.err
}

func TestRotationManager_EndToEndFallbackCycle_WritesConfig(t *testing.T) {
	root := repoRoot(t)

	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	seeds := []profile.Profile{
		{
			ID:      "p1",
			Family:  profile.FamilyReality,
			Enabled: true,
			Endpoint: profile.Endpoint{
				Host: "127.0.0.1",
				Port: 443,
			},
			Capabilities: profile.Capabilities{Transport: "tcp"},
			Health:       profile.Health{SuccessEWMA: 0.9},
		},
	}
	if err := store.Save(seeds); err != nil {
		t.Fatalf("save: %v", err)
	}

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	rm := policy.NewRotationManager(&errorAdvisor{err: os.ErrNotExist}, ctrl, gen, store)

	rm.Rotate(context.Background(), []detector.Event{
		{Type: detector.EventSNIBlockSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()},
	})

	configPath := filepath.Join(ctrl.ConfigDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.json to exist, stat error: %v", err)
	}
}

func TestRotationManager_LLMClientInvalidJSON_FallbackStillAppliesConfig(t *testing.T) {
	root := repoRoot(t)

	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save([]profile.Profile{
		{
			ID:      "p1",
			Family:  profile.FamilyReality,
			Enabled: true,
			Endpoint: profile.Endpoint{
				Host: "127.0.0.1",
				Port: 443,
			},
			Capabilities: profile.Capabilities{Transport: "tcp"},
			Health:       profile.Health{SuccessEWMA: 0.9},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":"not-json"}`))
	}))
	defer mockServer.Close()

	llmClient := llm.NewLocalLlamaCPPClient(mockServer.URL, false)

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	rm := policy.NewRotationManager(llmClient, ctrl, gen, store)

	rm.Rotate(context.Background(), []detector.Event{
		{Type: detector.EventActiveResetSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()},
	})

	configPath := filepath.Join(ctrl.ConfigDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.json to exist, stat error: %v", err)
	}
}
