package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutsideVerify_FailsOnWrongSignerKey(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	keysDir := filepath.Join(tmp, "keys")
	distDir := filepath.Join(tmp, "dist")
	profilesPath := filepath.Join(tmp, "profiles.json")

	edPriv1 := filepath.Join(keysDir, "outside1.ed25519")
	edPub1 := filepath.Join(keysDir, "outside1.ed25519.pub")
	edPriv2 := filepath.Join(keysDir, "outside2.ed25519")
	edPub2 := filepath.Join(keysDir, "outside2.ed25519.pub")

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "keygen",
		"--ed25519-priv", edPriv1,
		"--ed25519-pub", edPub1,
	)
	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "keygen",
		"--ed25519-priv", edPriv2,
		"--ed25519-pub", edPub2,
	)

	writeProfilesFile(t, profilesPath, []map[string]any{
		mkRealityProfile("r1", "example.com", 443),
	})

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside",
		"--profiles", profilesPath,
		"--out", distDir,
		"--templates-dir", filepath.Join(root, "templates"),
		"--signing-key", edPriv1,
		"--allow-plaintext",
		"--bundle-ttl-sec", "3600",
	)

	cmd := exec.Command("go", "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub2,
	)
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected verify failure, got success. output:\n%s", buf.String())
	}
}

func TestOutsideVerify_RequireDecryptFailsWithoutIdentity(t *testing.T) {
	root := repoRoot(t)
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
	})

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside",
		"--profiles", profilesPath,
		"--out", distDir,
		"--templates-dir", filepath.Join(root, "templates"),
		"--signing-key", edPriv,
		"--recipient-pub", ageRecipient,
		"--bundle-ttl-sec", "3600",
	)

	cmd := exec.Command("go", "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub,
		"--require-decrypt",
	)
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected verify failure, got success. output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "decrypt") {
		t.Fatalf("expected decrypt-related failure, got:\n%s", buf.String())
	}
}

func TestOutsideGenerate_ExcludesMalformedProfiles(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	keysDir := filepath.Join(tmp, "keys")
	distDir := filepath.Join(tmp, "dist")
	profilesPath := filepath.Join(tmp, "profiles.json")

	edPriv := filepath.Join(keysDir, "outside.ed25519")
	edPub := filepath.Join(keysDir, "outside.ed25519.pub")
	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "keygen",
		"--ed25519-priv", edPriv,
		"--ed25519-pub", edPub,
	)

	writeProfilesFile(t, profilesPath, []map[string]any{
		{"id": "", "family": "reality"},
		mkRealityProfile("r1", "example.com", 443),
	})

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside",
		"--profiles", profilesPath,
		"--out", distDir,
		"--templates-dir", filepath.Join(root, "templates"),
		"--signing-key", edPriv,
		"--allow-plaintext",
		"--bundle-ttl-sec", "3600",
	)

	manifest := readJSONFile(t, filepath.Join(distDir, "manifest.json"))
	sel := manifest["selection"].(map[string]any)
	excluded := sel["excluded"].([]any)
	if len(excluded) == 0 {
		t.Fatalf("expected exclusions")
	}

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub,
	)

	if _, err := os.Stat(filepath.Join(distDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json: %v", err)
	}
}
