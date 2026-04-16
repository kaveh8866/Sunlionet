package profile

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var tagPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// NormalizeForWire returns a canonical, safe representation for distribution bundles.
// It rejects unsupported families, unsafe template references, invalid endpoints, and malformed credentials.
func NormalizeForWire(p Profile, nowUnix int64) (Profile, error) {
	p.ID = strings.TrimSpace(strings.ToLower(p.ID))
	if p.ID == "" {
		return Profile{}, fmt.Errorf("missing id")
	}
	if !profileIDPattern.MatchString(p.ID) {
		return Profile{}, fmt.Errorf("invalid id")
	}

	fam, err := normalizeFamily(p.Family)
	if err != nil {
		return Profile{}, err
	}
	p.Family = fam

	p.Endpoint.Host = strings.ToLower(strings.TrimSpace(p.Endpoint.Host))
	if p.Endpoint.Host == "" {
		return Profile{}, fmt.Errorf("missing endpoint.host")
	}
	if p.Endpoint.Port <= 0 || p.Endpoint.Port > 65535 {
		return Profile{}, fmt.Errorf("invalid endpoint.port")
	}
	p.Endpoint.IPVersion = strings.ToLower(strings.TrimSpace(p.Endpoint.IPVersion))
	if p.Endpoint.IPVersion == "" {
		p.Endpoint.IPVersion = "dual"
	}
	switch p.Endpoint.IPVersion {
	case "v4", "v6", "dual":
	default:
		return Profile{}, fmt.Errorf("invalid endpoint.ip_version")
	}

	p.TemplateRef = strings.TrimSpace(p.TemplateRef)
	if strings.Contains(p.TemplateRef, "..") {
		return Profile{}, fmt.Errorf("invalid template_ref")
	}
	if strings.ContainsAny(p.TemplateRef, `/\`) {
		return Profile{}, fmt.Errorf("invalid template_ref")
	}

	p.Capabilities.Transport = strings.TrimSpace(strings.ToLower(p.Capabilities.Transport))
	if p.Capabilities.Transport == "" {
		return Profile{}, fmt.Errorf("missing capabilities.transport")
	}
	if err := validateTransport(p.Family, p.Capabilities.Transport); err != nil {
		return Profile{}, err
	}

	p.Capabilities.BandwidthClass = strings.ToLower(strings.TrimSpace(p.Capabilities.BandwidthClass))
	switch p.Capabilities.BandwidthClass {
	case "", "low", "medium", "high":
	default:
		return Profile{}, fmt.Errorf("invalid capabilities.bandwidth_class")
	}

	sort.Strings(p.Capabilities.DPIResistanceTags)
	p.Capabilities.DPIResistanceTags = uniqueSortedTags(p.Capabilities.DPIResistanceTags)
	for _, tag := range p.Capabilities.DPIResistanceTags {
		if !tagPattern.MatchString(tag) {
			return Profile{}, fmt.Errorf("invalid capabilities.dpi_resistance_tags")
		}
	}

	p.Source.Source = strings.TrimSpace(p.Source.Source)
	if p.Source.Source == "" {
		return Profile{}, fmt.Errorf("missing source.source")
	}
	if p.Source.TrustLevel <= 0 || p.Source.TrustLevel > 100 {
		return Profile{}, fmt.Errorf("invalid source.trust_level")
	}
	if p.Source.ImportedAt == 0 {
		p.Source.ImportedAt = nowUnix
	}
	if p.Source.ImportedAt < 0 {
		return Profile{}, fmt.Errorf("invalid source.imported_at")
	}

	if err := validateCredentials(p); err != nil {
		return Profile{}, err
	}

	if p.CreatedAt == 0 {
		p.CreatedAt = nowUnix
	}
	if p.CreatedAt < 0 {
		return Profile{}, fmt.Errorf("invalid created_at")
	}
	if p.ExpiresAt != 0 {
		if p.ExpiresAt <= nowUnix {
			return Profile{}, fmt.Errorf("expired profile")
		}
		if p.ExpiresAt < p.CreatedAt {
			return Profile{}, fmt.Errorf("invalid expires_at")
		}
	}

	if p.Priority < -100 || p.Priority > 100 {
		return Profile{}, fmt.Errorf("invalid priority")
	}

	p.Enabled = true
	p.ManualDisabled = false
	p.Health = Health{}
	p.Notes = strings.TrimSpace(p.Notes)
	return p, nil
}

func normalizeFamily(f Family) (Family, error) {
	s := strings.TrimSpace(strings.ToLower(string(f)))
	switch s {
	case string(FamilyReality):
		return FamilyReality, nil
	case "hy2", string(FamilyHysteria2):
		return FamilyHysteria2, nil
	case string(FamilyTUIC):
		return FamilyTUIC, nil
	case string(FamilyShadowTLS):
		return FamilyShadowTLS, nil
	case string(FamilyDNS):
		return FamilyDNS, nil
	default:
		return "", fmt.Errorf("unsupported family")
	}
}

func validateTransport(fam Family, transport string) error {
	switch fam {
	case FamilyReality, FamilyShadowTLS:
		if transport != "tcp" {
			return fmt.Errorf("invalid transport for family")
		}
	case FamilyHysteria2, FamilyTUIC:
		if transport != "udp" && transport != "quic" {
			return fmt.Errorf("invalid transport for family")
		}
	case FamilyDNS:
		if transport != "udp" && transport != "tcp" {
			return fmt.Errorf("invalid transport for family")
		}
	default:
		return fmt.Errorf("unsupported family")
	}
	return nil
}

func uniqueSortedTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if len(out) == 0 || out[len(out)-1] != t {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validateCredentials(p Profile) error {
	switch p.Family {
	case FamilyReality:
		if strings.TrimSpace(p.Credentials.UUID) == "" {
			return fmt.Errorf("missing credentials.uuid")
		}
		if strings.TrimSpace(p.Credentials.PublicKey) == "" {
			return fmt.Errorf("missing credentials.public_key")
		}
		if strings.TrimSpace(p.Credentials.ShortID) == "" {
			return fmt.Errorf("missing credentials.short_id")
		}
		if strings.TrimSpace(p.Credentials.SNI) == "" {
			return fmt.Errorf("missing credentials.sni")
		}
		if strings.TrimSpace(p.Credentials.UTLSFingerprint) == "" {
			return fmt.Errorf("missing credentials.utls_fingerprint")
		}
	case FamilyHysteria2:
		if strings.TrimSpace(p.Credentials.Password) == "" {
			return fmt.Errorf("missing credentials.password")
		}
		if strings.TrimSpace(p.Credentials.ObfsPassword) == "" {
			return fmt.Errorf("missing credentials.obfs_password")
		}
		if strings.TrimSpace(p.Credentials.SNI) == "" {
			return fmt.Errorf("missing credentials.sni")
		}
	case FamilyTUIC:
		if strings.TrimSpace(p.Credentials.UUID) == "" {
			return fmt.Errorf("missing credentials.uuid")
		}
		if strings.TrimSpace(p.Credentials.Password) == "" {
			return fmt.Errorf("missing credentials.password")
		}
		if strings.TrimSpace(p.Credentials.SNI) == "" {
			return fmt.Errorf("missing credentials.sni")
		}
	case FamilyShadowTLS, FamilyDNS:
		return nil
	default:
		return fmt.Errorf("unsupported family")
	}
	return nil
}
