package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestInsideRealMode_RenderOnly_Smoke(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	masterKey := []byte("0123456789abcdef0123456789abcdef")
	storePath := filepath.Join(stateDir, "profiles.enc")
	store, err := profile.NewStore(storePath, masterKey)
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
		Credentials: profile.Credentials{
			UUID:            "00000000-0000-0000-0000-000000000000",
			PublicKey:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ShortID:         "bbbbbbbb",
			SNI:             "www.apple.com",
			UTLSFingerprint: "chrome",
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
	}
	if err := store.Save([]profile.Profile{p}); err != nil {
		t.Fatalf("save profiles: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/inside",
		"--mode=real",
		"--render-only",
		"--state-dir="+stateDir,
		"--master-key="+string(masterKey),
		"--templates-dir="+filepath.Join(".", "templates"),
	)
	cmd.Dir = filepath.Join("..", "..")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run failed: %v\n%s", err, out.String())
	}

	want := filepath.Join(stateDir, "runtime", "config.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected config to be rendered at %s: %v\n%s", want, err, out.String())
	}
}
