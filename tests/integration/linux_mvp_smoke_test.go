package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type agentState struct {
	StateDir           string   `json:"state_dir"`
	ProfilesLoaded     int      `json:"profiles_loaded"`
	SelectedProfileID  string   `json:"selected_profile_id"`
	SelectionReason    string   `json:"selection_reason"`
	FallbackCandidates []string `json:"fallback_candidates"`
	ConfigPath         string   `json:"config_path"`
	SingBoxBinary      string   `json:"sing_box_binary"`
	SingBoxPID         int      `json:"sing_box_pid"`
	UpdatedAtUnix      int64    `json:"updated_at_unix"`
}

func prepareBundleForInside(t *testing.T, root string) (bundlePath string, signerPubB64URL string, ageIdentity string) {
	t.Helper()

	tmp := t.TempDir()
	keysDir := filepath.Join(tmp, "keys")
	distDir := filepath.Join(tmp, "dist")
	profilesPath := filepath.Join(tmp, "profiles.json")

	edPriv := filepath.Join(keysDir, "outside.ed25519")
	edPub := filepath.Join(keysDir, "outside.ed25519.pub")
	ageID := filepath.Join(keysDir, "inside.agekey")
	ageRecipient := filepath.Join(keysDir, "inside.agepub")

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "keygen",
		"--ed25519-priv", edPriv,
		"--ed25519-pub", edPub,
		"--age-identity", ageID,
		"--age-recipient", ageRecipient,
	)

	writeProfilesFile(t, profilesPath, []map[string]any{
		mkRealityProfile("r1", "example.com", 443),
		mkHysteria2Profile("h1", "example.com", 9443),
	})

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside",
		"--profiles", profilesPath,
		"--out", distDir,
		"--templates-dir", filepath.Join(root, "templates"),
		"--signing-key", edPriv,
		"--recipient-pub", ageRecipient,
		"--bundle-ttl-sec", "3600",
		"--max-profiles", "10",
		"--max-per-family", "10",
	)

	pubRaw, err := os.ReadFile(edPub)
	if err != nil {
		t.Fatalf("read signer pub: %v", err)
	}
	ageRaw, err := os.ReadFile(ageID)
	if err != nil {
		t.Fatalf("read age identity: %v", err)
	}

	return filepath.Join(distDir, "bundle.snb.json"), strings.TrimSpace(string(pubRaw)), strings.TrimSpace(string(ageRaw))
}

func buildFakeSingBox(t *testing.T, dir string) string {
	t.Helper()

	src := `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing subcommand")
		os.Exit(2)
	}

	sub := os.Args[1]
	var cfgPath string
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "-c" && i+1 < len(os.Args) {
			cfgPath = os.Args[i+1]
		}
	}
	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "missing -c config path")
		os.Exit(2)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if !json.Valid(raw) {
		fmt.Fprintln(os.Stderr, "invalid json")
		os.Exit(1)
	}

	switch sub {
	case "check":
		os.Exit(0)
	case "run":
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		select {
		case <-sig:
			os.Exit(0)
		case <-time.After(10 * time.Second):
			os.Exit(0)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand")
		os.Exit(2)
	}
}
`

	srcPath := filepath.Join(dir, "fake_singbox.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		t.Fatalf("write fake sing-box src: %v", err)
	}

	outPath := filepath.Join(dir, "sing-box")
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outPath, srcPath)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake sing-box: %v\n%s", err, string(out))
	}
	return outPath
}

func TestInsideLinuxMVP_Smoke_GoRun(t *testing.T) {
	root := repoRoot(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	bundlePath, pubB64, ageID := prepareBundleForInside(t, root)

	fake := buildFakeSingBox(t, t.TempDir())

	masterKey := "0123456789abcdef0123456789abcdef"
	args := []string{
		"run", "./cmd/inside",
		"--state-dir", stateDir,
		"--import", bundlePath,
		"--master-key", masterKey,
		"--trusted-signer-pub-b64url", pubB64,
		"--age-identity", ageID,
		"--sing-box-bin", fake,
		"--verbose",
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("inside failed: %v\n%s", err, outBuf.String())
	}

	rawState, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var st agentState
	if err := json.Unmarshal(rawState, &st); err != nil {
		t.Fatalf("parse state.json: %v", err)
	}

	if st.ProfilesLoaded < 1 {
		t.Fatalf("expected profiles_loaded >= 1, got %d", st.ProfilesLoaded)
	}
	if st.SelectedProfileID == "" {
		t.Fatalf("expected selected_profile_id to be set")
	}
	if st.FallbackCandidates == nil {
		t.Fatalf("expected fallback_candidates to be present")
	}
	if st.ConfigPath == "" {
		t.Fatalf("expected config_path to be set")
	}
	if _, err := os.Stat(st.ConfigPath); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	cfgBytes, err := os.ReadFile(st.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !json.Valid(cfgBytes) {
		t.Fatalf("rendered config is not valid json")
	}
	if st.SingBoxPID <= 0 {
		t.Fatalf("expected sing_box_pid > 0, got %d", st.SingBoxPID)
	}

	proc, err := os.FindProcess(st.SingBoxPID)
	if err == nil {
		_ = proc.Kill()
	}
}

func TestInsideLinuxMVP_MissingSingBox_FailsCleanly(t *testing.T) {
	root := repoRoot(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	bundlePath, pubB64, ageID := prepareBundleForInside(t, root)

	masterKey := "0123456789abcdef0123456789abcdef"
	args := []string{
		"run", "./cmd/inside",
		"--state-dir", stateDir,
		"--import", bundlePath,
		"--master-key", masterKey,
		"--trusted-signer-pub-b64url", pubB64,
		"--age-identity", ageID,
		"--sing-box-bin", filepath.Join(stateDir, "missing-sing-box"),
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), "sing-box binary not found") {
		t.Fatalf("expected actionable sing-box error, got:\n%s", string(out))
	}
	if !strings.Contains(string(out), "set --sing-box-bin") {
		t.Fatalf("expected actionable hint, got:\n%s", string(out))
	}
}
