package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutsideMVP_Plaintext_GenerateAndVerify(t *testing.T) {
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
		mkRealityProfile("r1", "example.com", 443),
		mkHysteria2Profile("h1", "example.com", 9443),
	})

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside",
		"--profiles", profilesPath,
		"--out", distDir,
		"--templates-dir", filepath.Join(root, "templates"),
		"--signing-key", edPriv,
		"--allow-plaintext",
		"--bundle-ttl-sec", "3600",
		"--max-profiles", "10",
		"--max-per-family", "10",
	)

	mustExist(t, filepath.Join(distDir, "bundle.snb.json"))
	mustExist(t, filepath.Join(distDir, "bundle.uri.txt"))
	mustExist(t, filepath.Join(distDir, "manifest.json"))
	mustExist(t, filepath.Join(distDir, "issuer_pub.b64url"))
	mustExist(t, filepath.Join(distDir, "bundle.sig.b64url"))

	manifest := readJSONFile(t, filepath.Join(distDir, "manifest.json"))
	hdr := manifest["header"].(map[string]any)
	if hdr["cipher"].(string) != "none" {
		t.Fatalf("expected cipher=none, got %v", hdr["cipher"])
	}
	if hdr["recipient_key_id"].(string) != "none" {
		t.Fatalf("expected recipient_key_id=none, got %v", hdr["recipient_key_id"])
	}

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub,
	)
	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub,
		"--json",
	)

	uriText, err := os.ReadFile(filepath.Join(distDir, "bundle.uri.txt"))
	if err != nil {
		t.Fatalf("read uri: %v", err)
	}
	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "verify",
		"--uri", strings.TrimSpace(string(uriText)),
		"--signer-pub", edPub,
	)
}

func TestOutsideMVP_Encrypted_GenerateAndVerify(t *testing.T) {
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

	manifest := readJSONFile(t, filepath.Join(distDir, "manifest.json"))
	hdr := manifest["header"].(map[string]any)
	if hdr["cipher"].(string) != "age-x25519" {
		t.Fatalf("expected cipher=age-x25519, got %v", hdr["cipher"])
	}
	if !strings.HasPrefix(hdr["recipient_key_id"].(string), "age-x25519:") {
		t.Fatalf("expected recipient_key_id age-x25519:..., got %v", hdr["recipient_key_id"])
	}

	runGoOutside(t, root, "run", "-tags", "outside", "./cmd/outside", "verify",
		"--bundle", filepath.Join(distDir, "bundle.snb.json"),
		"--signer-pub", edPub,
		"--age-identity", ageID,
		"--require-decrypt",
	)
}

func runGoOutside(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: go %s\n%v\n%s", strings.Join(args, " "), err, buf.String())
	}
	return buf.String()
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %s (%v)", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	return v
}

func writeProfilesFile(t *testing.T, path string, profiles []map[string]any) {
	t.Helper()
	raw, err := json.Marshal(profiles)
	if err != nil {
		t.Fatalf("marshal profiles: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write profiles: %v", err)
	}
}

func mkRealityProfile(id string, host string, port int) map[string]any {
	return map[string]any{
		"id":           id,
		"family":       "reality",
		"created_at":   0,
		"expires_at":   0,
		"priority":     0,
		"template_ref": "",
		"endpoint": map[string]any{
			"host":       host,
			"port":       port,
			"ip_version": "dual",
		},
		"capabilities": map[string]any{
			"transport":           "tcp",
			"dpi_resistance_tags": []string{"utls"},
		},
		"credentials": map[string]any{
			"uuid":             "00000000-0000-0000-0000-000000000001",
			"public_key":       "pk",
			"short_id":         "sid",
			"sni":              "sni.example.com",
			"utls_fingerprint": "chrome",
		},
		"source": map[string]any{
			"source":      "test",
			"trust_level": 80,
		},
		"notes": "",
	}
}

func mkHysteria2Profile(id string, host string, port int) map[string]any {
	return map[string]any{
		"id":           id,
		"family":       "hysteria2",
		"created_at":   0,
		"expires_at":   0,
		"priority":     0,
		"template_ref": "",
		"endpoint": map[string]any{
			"host":       host,
			"port":       port,
			"ip_version": "dual",
		},
		"capabilities": map[string]any{
			"transport":           "udp",
			"dpi_resistance_tags": []string{"quic"},
		},
		"credentials": map[string]any{
			"password":      "pw",
			"obfs_password": "obfs",
			"sni":           "sni.example.com",
		},
		"source": map[string]any{
			"source":      "test",
			"trust_level": 80,
		},
		"notes": "",
	}
}
