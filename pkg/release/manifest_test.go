package release

import (
	"strings"
	"testing"
)

func validManifestJSON() string {
	return `{
		"schema":"shadownet.release-manifest.v1",
		"version":"v0.1.0",
		"artifacts":[
			{
				"name":"shadownet-inside-v0.1.0-linux-amd64.tar.gz",
				"sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				"size":1234
			}
		],
		"mapping":{"preferred":{"android-arm64":["shadownet-inside-v0.1.0-android-arm64"]}}
	}`
}

func TestLoad_ValidManifest(t *testing.T) {
	m, err := Load(strings.NewReader(validManifestJSON()))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if m.Version != "v0.1.0" {
		t.Fatalf("unexpected version: %s", m.Version)
	}
	if got := m.Find("shadownet-inside-v0.1.0-linux-amd64.tar.gz"); got == nil {
		t.Fatalf("expected artifact to be found")
	}
}

func TestLoad_RejectsTrailingData(t *testing.T) {
	payload := validManifestJSON() + "\n{}\n"
	_, err := Load(strings.NewReader(payload))
	if err == nil {
		t.Fatalf("expected error for trailing data")
	}
	if !strings.Contains(err.Error(), "unexpected trailing data") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_RejectsDuplicateArtifactName(t *testing.T) {
	m := &Manifest{
		Schema:  SchemaV1,
		Version: "v0.1.0",
		Artifacts: []Artifact{
			{Name: "a", SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
			{Name: "a", SHA256: "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"},
		},
	}
	err := m.Validate()
	if err == nil {
		t.Fatalf("expected duplicate artifact validation error")
	}
}

func TestPreferredFor_DefensiveCopy(t *testing.T) {
	m := &Manifest{
		Mapping: &AssetRules{
			Preferred: map[string][]string{
				"linux-amd64": {"one", "two"},
			},
		},
	}

	got := m.PreferredFor("linux-amd64")
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d", len(got))
	}
	got[0] = "mutated"

	again := m.PreferredFor("linux-amd64")
	if again[0] != "one" {
		t.Fatalf("expected internal slice to stay unchanged, got %q", again[0])
	}
}

func TestKey(t *testing.T) {
	key, err := Key("android", "arm64")
	if err != nil {
		t.Fatalf("unexpected key error: %v", err)
	}
	if key != "android-arm64" {
		t.Fatalf("unexpected key: %s", key)
	}

	if _, err := Key("android", "arm64!"); err == nil {
		t.Fatalf("expected invalid key error")
	}
}
