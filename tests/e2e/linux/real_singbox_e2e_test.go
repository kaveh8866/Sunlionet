package linux

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type agentState struct {
	Status string `json:"status"`
	Probe  struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"probe"`
}

func TestInside_RealSingBox_HTTPProbe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only e2e")
	}
	if os.Getenv("SUNLIONET_E2E") != "1" {
		t.Skip("set SUNLIONET_E2E=1 to enable")
	}

	if _, err := exec.LookPath("sing-box"); err != nil {
		t.Skip("sing-box not installed in PATH")
	}

	bundle := os.Getenv("SUNLIONET_E2E_BUNDLE")
	trusted := os.Getenv("SUNLIONET_E2E_TRUSTED_SIGNER_PUB_B64URL")
	ageID := os.Getenv("SUNLIONET_E2E_AGE_IDENTITY")
	masterKey := os.Getenv("SUNLIONET_E2E_MASTER_KEY")
	if bundle == "" || trusted == "" || ageID == "" || masterKey == "" {
		t.Skip("missing SUNLIONET_E2E_* env vars (see docs/testing/e2e-validation.md)")
	}

	root := repoRoot(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	args := []string{
		"run", "./cmd/inside",
		"--mode=real",
		"--state-dir", stateDir,
		"--import", bundle,
		"--master-key", masterKey,
		"--trusted-signer-pub-b64url", trusted,
		"--age-identity", ageID,
		"--probe-url", "https://example.com",
		"--verbose",
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inside failed: %v\n%s", err, string(out))
	}

	raw, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var st agentState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("parse state.json: %v", err)
	}
	if st.Status != "running" {
		t.Fatalf("expected status=running got %q", st.Status)
	}
	if st.Probe.Status != "ok" {
		t.Fatalf("expected probe ok, got status=%q reason=%q", st.Probe.Status, st.Probe.Reason)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		wd = filepath.Dir(wd)
	}
	t.Fatal("repo root not found")
	return ""
}
