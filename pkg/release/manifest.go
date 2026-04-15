package release

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
)

const SchemaV1 = "shadownet.release-manifest.v1"

type Manifest struct {
	Schema    string      `json:"schema"`
	Version   string      `json:"version"`
	Artifacts []Artifact  `json:"artifacts"`
	Meta      *Meta       `json:"meta,omitempty"`
	Mapping   *AssetRules `json:"mapping,omitempty"`
}

type Meta struct {
	Repo      string `json:"repo,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
	BuiltAt   string `json:"built_at,omitempty"`
}

type AssetRules struct {
	Preferred map[string][]string `json:"preferred,omitempty"`
}

type Artifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256,omitempty"`
	SHA512 string `json:"sha512,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

func Load(r io.Reader) (*Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("manifest: unexpected trailing data")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func LoadFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func (m *Manifest) Validate() error {
	if m.Schema == "" {
		return errors.New("manifest: missing schema")
	}
	if m.Schema != SchemaV1 {
		return fmt.Errorf("manifest: unsupported schema: %q", m.Schema)
	}
	if m.Version == "" {
		return errors.New("manifest: missing version")
	}
	if len(m.Artifacts) == 0 {
		return errors.New("manifest: missing artifacts")
	}

	nameSeen := map[string]struct{}{}
	for i := range m.Artifacts {
		a := m.Artifacts[i]
		if a.Name == "" {
			return fmt.Errorf("manifest: artifacts[%d]: missing name", i)
		}
		if _, ok := nameSeen[a.Name]; ok {
			return fmt.Errorf("manifest: duplicate artifact name: %q", a.Name)
		}
		nameSeen[a.Name] = struct{}{}

		if a.SHA256 != "" && !isHexLen(a.SHA256, sha256.Size) {
			return fmt.Errorf("manifest: %s: invalid sha256", a.Name)
		}
		if a.SHA512 != "" && !isHexLen(a.SHA512, sha512.Size) {
			return fmt.Errorf("manifest: %s: invalid sha512", a.Name)
		}
		if a.Size < 0 {
			return fmt.Errorf("manifest: %s: invalid size", a.Name)
		}
	}

	return nil
}

func (m *Manifest) Find(name string) *Artifact {
	for i := range m.Artifacts {
		if m.Artifacts[i].Name == name {
			return &m.Artifacts[i]
		}
	}
	return nil
}

func (m *Manifest) Names() []string {
	out := make([]string, 0, len(m.Artifacts))
	for i := range m.Artifacts {
		out = append(out, m.Artifacts[i].Name)
	}
	sort.Strings(out)
	return out
}

func (m *Manifest) PreferredFor(key string) []string {
	if m.Mapping == nil || m.Mapping.Preferred == nil {
		return nil
	}
	return append([]string(nil), m.Mapping.Preferred[key]...)
}

func isHexLen(s string, bytes int) bool {
	if len(s) != bytes*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

var osArchKeyRe = regexp.MustCompile(`^[a-z0-9_.-]+$`)

func Key(os, arch string) (string, error) {
	if os == "" || arch == "" {
		return "", errors.New("manifest: os/arch required")
	}
	key := os + "-" + arch
	if !osArchKeyRe.MatchString(key) {
		return "", errors.New("manifest: invalid os/arch")
	}
	return key, nil
}
