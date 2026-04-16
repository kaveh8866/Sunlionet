package outsidectl

import (
	"testing"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestNormalizeRejectsTemplateRefTraversalAndSeparators(t *testing.T) {
	now := int64(123)
	for _, ref := range []string{"../x", `..\x`, "a/b", `a\b`} {
		p := profile.Profile{
			ID:          "p1",
			Family:      profile.FamilyReality,
			TemplateRef: ref,
			Endpoint: profile.Endpoint{
				Host: "example.com",
				Port: 443,
			},
			Capabilities: profile.Capabilities{
				Transport: "tcp",
			},
			Credentials: profile.Credentials{
				UUID:            "u",
				PublicKey:       "pk",
				ShortID:         "sid",
				SNI:             "sni",
				UTLSFingerprint: "chrome",
			},
			Source: profile.SourceInfo{
				Source:     "test",
				TrustLevel: 80,
			},
		}
		if _, err := profile.NormalizeForWire(p, now); err == nil {
			t.Fatalf("expected error for template_ref=%q", ref)
		}
	}
}

func TestNormalizeDefaultsAndValidatesEndpointIPVersion(t *testing.T) {
	now := int64(123)
	p := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyHysteria2,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 9443,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Credentials: profile.Credentials{
			Password:     "pw",
			ObfsPassword: "obfs",
			SNI:          "sni",
		},
		Source: profile.SourceInfo{
			Source:     "test",
			TrustLevel: 80,
		},
	}
	np, err := profile.NormalizeForWire(p, now)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if np.Endpoint.IPVersion != "dual" {
		t.Fatalf("expected ip_version to default to dual, got %q", np.Endpoint.IPVersion)
	}

	np.Endpoint.IPVersion = "weird"
	if _, err := profile.NormalizeForWire(np, now); err == nil {
		t.Fatalf("expected invalid ip_version error")
	}
}

func TestNormalizeRejectsMissingFamilyCredentials(t *testing.T) {
	now := int64(123)
	p := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyTUIC,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:     "test",
			TrustLevel: 80,
		},
	}
	if _, err := profile.NormalizeForWire(p, now); err == nil {
		t.Fatalf("expected missing credentials error")
	}
}

func TestSelectForBundleCapsAndDuplicateEndpoint(t *testing.T) {
	now := int64(123)
	candidates := []Candidate{
		{Profile: mkProfile("a", profile.FamilyReality, "h1", 443), Score: 100},
		{Profile: mkProfile("b", profile.FamilyReality, "h1", 443), Score: 90},
		{Profile: mkProfile("c", profile.FamilyTUIC, "h2", 443), Score: 80},
		{Profile: mkProfile("d", profile.FamilyTUIC, "h3", 443), Score: 70},
	}

	res := SelectForBundle(candidates, SelectionParams{MaxProfiles: 2, MaxPerFamily: 1, NowUnix: now})
	if len(res.Included) != 2 {
		t.Fatalf("expected 2 included, got %d", len(res.Included))
	}

	seen := map[string]bool{}
	for _, c := range res.Included {
		ek := endpointKey(c.Profile)
		if seen[ek] {
			t.Fatalf("duplicate endpoint included: %s", ek)
		}
		seen[ek] = true
	}
}

func mkProfile(id string, fam profile.Family, host string, port int) profile.Profile {
	p := profile.Profile{
		ID:     id,
		Family: fam,
		Endpoint: profile.Endpoint{
			Host: host,
			Port: port,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Credentials: profile.Credentials{
			UUID:            "u",
			Password:        "p",
			PublicKey:       "pk",
			ShortID:         "sid",
			SNI:             "sni",
			UTLSFingerprint: "chrome",
			ObfsPassword:    "obfs",
		},
		Source: profile.SourceInfo{
			Source:     "test",
			TrustLevel: 80,
		},
	}

	switch fam {
	case profile.FamilyReality, profile.FamilyShadowTLS:
		p.Capabilities.Transport = "tcp"
	case profile.FamilyHysteria2, profile.FamilyTUIC:
		p.Capabilities.Transport = "udp"
	}

	return p
}
