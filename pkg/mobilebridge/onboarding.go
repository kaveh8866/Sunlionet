package mobilebridge

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

const (
	OnboardingURIPrefix       = "sunlionet://config/"
	OnboardingShortQRPrefix   = "SL1:"
	OnboardingChunkPrefix     = "SLQR1:"
	MaxOnboardingPayloadChars = 300
	MaxOnboardingURIChars     = len(OnboardingURIPrefix) + MaxOnboardingPayloadChars

	onboardingVersion byte = 1
	onboardingBodyMin      = 4 + 4 + 4 + 1 + 2 + ed25519.PublicKeySize + 1 + 1 + 1 + 1 + 1 + 1
	onboardingBodyMax      = 160
)

var (
	onboardingMagic       = [4]byte{'S', 'L', 'O', onboardingVersion}
	onboardingSignDomain  = []byte("SUNLIONET-ONBOARDING-V1\x00")
	onboardingHostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]{1,96}$`)
	onboardingSNIRegex    = regexp.MustCompile(`^[A-Za-z0-9.*-][A-Za-z0-9.*.-]{0,95}$`)
	onboardingTagRegex    = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

type OnboardingFamily byte

const (
	OnboardingFamilyReality   OnboardingFamily = 1
	OnboardingFamilyHysteria2 OnboardingFamily = 2
	OnboardingFamilyTUIC      OnboardingFamily = 3
	OnboardingFamilyShadowTLS OnboardingFamily = 4
	OnboardingFamilyDNS       OnboardingFamily = 5
)

type OnboardingPayload struct {
	IssuedAtUnix  int64
	ExpiresAtUnix int64
	Family        OnboardingFamily
	Host          string
	Port          int
	SNI           string
	Tag           string
	CredentialA   []byte
	CredentialB   []byte
	CredentialC   []byte
	SignerPub     ed25519.PublicKey
	Signature     []byte
}

func ParseOnboardingQRText(text string, trusted []ed25519.PublicKey, now time.Time) (OnboardingPayload, error) {
	normalized, err := NormalizeOnboardingText(text)
	if err != nil {
		return OnboardingPayload{}, err
	}
	return ParseOnboardingURI(normalized, trusted, now)
}

func NormalizeOnboardingText(text string) (string, error) {
	s := strings.TrimSpace(text)
	if s == "" {
		return "", fmt.Errorf("onboarding: empty link")
	}
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	if strings.HasPrefix(s, OnboardingURIPrefix) {
		if len(s) > MaxOnboardingURIChars {
			return "", fmt.Errorf("onboarding: link too large")
		}
		return s, nil
	}
	if strings.HasPrefix(s, OnboardingShortQRPrefix) {
		payload := strings.TrimPrefix(s, OnboardingShortQRPrefix)
		if len(payload) == 0 || len(payload) > MaxOnboardingPayloadChars {
			return "", fmt.Errorf("onboarding: payload size invalid")
		}
		return OnboardingURIPrefix + payload, nil
	}
	return "", fmt.Errorf("onboarding: unsupported uri")
}

func JoinOnboardingQRChunks(chunks []string) (string, error) {
	if len(chunks) == 0 || len(chunks) > 8 {
		return "", fmt.Errorf("onboarding: invalid chunk count")
	}
	parts := make([]string, len(chunks))
	total := 0
	for _, raw := range chunks {
		s := strings.TrimSpace(raw)
		if !strings.HasPrefix(s, OnboardingChunkPrefix) {
			return "", fmt.Errorf("onboarding: invalid chunk prefix")
		}
		rest := strings.TrimPrefix(s, OnboardingChunkPrefix)
		colon := strings.IndexByte(rest, ':')
		slash := strings.IndexByte(rest, '/')
		if slash <= 0 || colon <= slash+1 {
			return "", fmt.Errorf("onboarding: invalid chunk header")
		}
		var idx, n int
		if _, err := fmt.Sscanf(rest[:colon], "%d/%d", &idx, &n); err != nil {
			return "", fmt.Errorf("onboarding: invalid chunk index")
		}
		if n <= 0 || n > 8 || idx <= 0 || idx > n {
			return "", fmt.Errorf("onboarding: invalid chunk index")
		}
		if total == 0 {
			total = n
			parts = make([]string, n)
		} else if total != n {
			return "", fmt.Errorf("onboarding: inconsistent chunk total")
		}
		payload := rest[colon+1:]
		if payload == "" || len(payload) > MaxOnboardingPayloadChars {
			return "", fmt.Errorf("onboarding: invalid chunk payload")
		}
		if parts[idx-1] != "" {
			return "", fmt.Errorf("onboarding: duplicate chunk")
		}
		parts[idx-1] = payload
	}
	if total == 0 {
		total = len(parts)
	}
	for _, p := range parts[:total] {
		if p == "" {
			return "", fmt.Errorf("onboarding: missing chunk")
		}
	}
	return NormalizeOnboardingText(OnboardingShortQRPrefix + strings.Join(parts[:total], ""))
}

func ParseOnboardingURI(uri string, trusted []ed25519.PublicKey, now time.Time) (OnboardingPayload, error) {
	normalized, err := NormalizeOnboardingText(uri)
	if err != nil {
		return OnboardingPayload{}, err
	}
	token := strings.TrimPrefix(normalized, OnboardingURIPrefix)
	if len(token) == 0 || len(token) > MaxOnboardingPayloadChars {
		return OnboardingPayload{}, fmt.Errorf("onboarding: payload size invalid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: payload encoding invalid")
	}
	if len(raw) < onboardingBodyMin+ed25519.SignatureSize || len(raw) > onboardingBodyMax+ed25519.SignatureSize {
		return OnboardingPayload{}, fmt.Errorf("onboarding: payload length invalid")
	}
	body := raw[:len(raw)-ed25519.SignatureSize]
	sig := raw[len(raw)-ed25519.SignatureSize:]
	p, err := parseOnboardingBody(body)
	if err != nil {
		return OnboardingPayload{}, err
	}
	p.Signature = append([]byte(nil), sig...)
	if !trustedOnboardingSigner(p.SignerPub, trusted) {
		return OnboardingPayload{}, fmt.Errorf("onboarding: untrusted signer")
	}
	if len(p.Signature) != ed25519.SignatureSize || !ed25519.Verify(p.SignerPub, onboardingSignatureMessage(body), p.Signature) {
		return OnboardingPayload{}, fmt.Errorf("onboarding: invalid signature")
	}
	if now.IsZero() {
		now = time.Now()
	}
	nowUnix := now.Unix()
	if p.IssuedAtUnix > nowUnix+300 {
		return OnboardingPayload{}, fmt.Errorf("onboarding: not yet valid")
	}
	if p.ExpiresAtUnix <= nowUnix {
		return OnboardingPayload{}, fmt.Errorf("onboarding: expired")
	}
	if p.ExpiresAtUnix-p.IssuedAtUnix > int64((24 * time.Hour).Seconds()) {
		return OnboardingPayload{}, fmt.Errorf("onboarding: activation window too long")
	}
	if err := validateOnboardingPayload(p); err != nil {
		return OnboardingPayload{}, err
	}
	return p, nil
}

func EncodeOnboardingURI(p OnboardingPayload, signer ed25519.PrivateKey) (string, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("onboarding: invalid private key")
	}
	p.SignerPub = signer.Public().(ed25519.PublicKey)
	if err := validateOnboardingPayload(p); err != nil {
		return "", err
	}
	body, err := marshalOnboardingBody(p)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(signer, onboardingSignatureMessage(body))
	raw := append(append([]byte(nil), body...), sig...)
	token := base64.RawURLEncoding.EncodeToString(raw)
	if len(token) > MaxOnboardingPayloadChars {
		return "", fmt.Errorf("onboarding: encoded payload too large")
	}
	return OnboardingURIPrefix + token, nil
}

func ImportOnboardingURI(uri string) error {
	state.mu.Lock()
	cfg := state.cfg
	state.mu.Unlock()
	return importOnboardingURIWithConfig(uri, cfg)
}

func ImportOnboardingURIWithConfig(uri string, config string) error {
	var cfg AgentConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return importOnboardingURIWithConfig(uri, cfg)
}

func importOnboardingURIWithConfig(uri string, cfg AgentConfig) error {
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	masterKey, err := profile.ParseMasterKey(cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	trustedKeys, err := parseTrustedSignerKeys(cfg.TrustedSignerPubsB64)
	if err != nil {
		return err
	}
	payload, err := ParseOnboardingQRText(uri, trustedKeys, time.Now())
	if err != nil {
		return err
	}
	p, err := payload.ToProfile(time.Now())
	if err != nil {
		return err
	}
	store, err := profile.NewStore(filepath.Join(cfg.StateDir, "profiles.enc"), masterKey)
	if err != nil {
		return err
	}
	profiles, err := store.Load()
	if err != nil {
		return err
	}
	upserted := false
	for i := range profiles {
		if profiles[i].ID == p.ID {
			profiles[i] = p
			upserted = true
			break
		}
	}
	if !upserted {
		profiles = append(profiles, p)
	}
	if err := store.Save(profiles); err != nil {
		return err
	}
	state.mu.Lock()
	state.status.LastAction = "onboarding import"
	state.status.LastError = ""
	state.status.UpdatedAtUnix = time.Now().Unix()
	state.mu.Unlock()
	return nil
}

func (p OnboardingPayload) ToProfile(now time.Time) (profile.Profile, error) {
	if now.IsZero() {
		now = time.Now()
	}
	nowUnix := now.Unix()
	pubB64 := base64.RawURLEncoding.EncodeToString(p.SignerPub)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%d|%d|%s|%d|%x", p.Family, p.ExpiresAtUnix, p.IssuedAtUnix, p.Host, p.Port, p.SignerPub)))
	id := "onb_" + hex.EncodeToString(sum[:6])
	sni := strings.TrimSpace(p.SNI)
	if sni == "" {
		sni = p.Host
	}
	out := profile.Profile{
		ID:             id,
		Family:         p.profileFamily(),
		Enabled:        true,
		CreatedAt:      nowUnix,
		ExpiresAt:      p.ExpiresAtUnix,
		Priority:       20,
		TemplateRef:    string(p.profileFamily()),
		Endpoint:       profile.Endpoint{Host: p.Host, Port: p.Port, IPVersion: "dual"},
		Capabilities:   profile.Capabilities{Transport: p.defaultTransport(), BandwidthClass: "low", DPIResistanceTags: []string{"signed_onboarding"}},
		Source:         profile.SourceInfo{Source: "signed_onboarding_uri", TrustLevel: 80, ImportedAt: nowUnix, PublisherKey: pubB64},
		MutationPolicy: profile.MutationPolicy{AllowedSets: []string{"endpoint", "priority"}, MaxMutationsPerHour: 2},
		Notes:          "signed deep-link onboarding",
	}
	if p.Tag != "" {
		out.Capabilities.DPIResistanceTags = append(out.Capabilities.DPIResistanceTags, p.Tag)
	}
	switch p.Family {
	case OnboardingFamilyReality:
		if len(p.CredentialA) != 16 || len(p.CredentialB) != 32 || len(p.CredentialC) == 0 || len(p.CredentialC) > 8 {
			return profile.Profile{}, fmt.Errorf("onboarding: invalid reality credentials")
		}
		out.Credentials.UUID = uuidString(p.CredentialA)
		out.Credentials.PublicKey = base64.RawURLEncoding.EncodeToString(p.CredentialB)
		out.Credentials.ShortID = hex.EncodeToString(p.CredentialC)
		out.Credentials.SNI = sni
		out.Credentials.UTLSFingerprint = "chrome"
	case OnboardingFamilyHysteria2:
		if len(p.CredentialA) == 0 || len(p.CredentialB) == 0 {
			return profile.Profile{}, fmt.Errorf("onboarding: invalid hysteria2 credentials")
		}
		out.Credentials.Password = string(p.CredentialA)
		out.Credentials.ObfsPassword = string(p.CredentialB)
		out.Credentials.SNI = sni
	case OnboardingFamilyTUIC:
		if len(p.CredentialA) != 16 || len(p.CredentialB) == 0 {
			return profile.Profile{}, fmt.Errorf("onboarding: invalid tuic credentials")
		}
		out.Credentials.UUID = uuidString(p.CredentialA)
		out.Credentials.Password = string(p.CredentialB)
		out.Credentials.SNI = sni
	case OnboardingFamilyShadowTLS, OnboardingFamilyDNS:
	default:
		return profile.Profile{}, fmt.Errorf("onboarding: unsupported family")
	}
	return profile.NormalizeForWire(out, nowUnix)
}

func (p OnboardingPayload) profileFamily() profile.Family {
	switch p.Family {
	case OnboardingFamilyReality:
		return profile.FamilyReality
	case OnboardingFamilyHysteria2:
		return profile.FamilyHysteria2
	case OnboardingFamilyTUIC:
		return profile.FamilyTUIC
	case OnboardingFamilyShadowTLS:
		return profile.FamilyShadowTLS
	case OnboardingFamilyDNS:
		return profile.FamilyDNS
	default:
		return ""
	}
}

func (p OnboardingPayload) defaultTransport() string {
	switch p.Family {
	case OnboardingFamilyHysteria2, OnboardingFamilyTUIC, OnboardingFamilyDNS:
		return "udp"
	default:
		return "tcp"
	}
}

func marshalOnboardingBody(p OnboardingPayload) ([]byte, error) {
	if len(p.SignerPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("onboarding: invalid signer key")
	}
	buf := bytes.NewBuffer(make([]byte, 0, onboardingBodyMax))
	buf.Write(onboardingMagic[:])
	putU32(buf, p.IssuedAtUnix)
	putU32(buf, p.ExpiresAtUnix)
	buf.WriteByte(byte(p.Family))
	putU16(buf, p.Port)
	buf.Write(p.SignerPub)
	buf.WriteByte(0)
	writeLV(buf, []byte(strings.ToLower(p.Host)))
	writeLV(buf, []byte(strings.ToLower(p.SNI)))
	writeLV(buf, p.CredentialA)
	writeLV(buf, p.CredentialB)
	writeLV(buf, p.CredentialC)
	writeLV(buf, []byte(p.Tag))
	if buf.Len() > onboardingBodyMax {
		return nil, fmt.Errorf("onboarding: body too large")
	}
	return buf.Bytes(), nil
}

func parseOnboardingBody(body []byte) (OnboardingPayload, error) {
	if len(body) < onboardingBodyMin || len(body) > onboardingBodyMax {
		return OnboardingPayload{}, fmt.Errorf("onboarding: body length invalid")
	}
	if !bytes.Equal(body[:4], onboardingMagic[:]) {
		return OnboardingPayload{}, fmt.Errorf("onboarding: unsupported version")
	}
	r := bytes.NewReader(body[4:])
	var issued, expires uint32
	if err := binary.Read(r, binary.BigEndian, &issued); err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	if err := binary.Read(r, binary.BigEndian, &expires); err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	family, err := r.ReadByte()
	if err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	var port uint16
	if err := binary.Read(r, binary.BigEndian, &port); err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	signer := make([]byte, ed25519.PublicKeySize)
	if _, err := r.Read(signer); err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	if _, err := r.ReadByte(); err != nil {
		return OnboardingPayload{}, fmt.Errorf("onboarding: malformed body")
	}
	host, err := readLV(r, 96)
	if err != nil {
		return OnboardingPayload{}, err
	}
	sni, err := readLV(r, 96)
	if err != nil {
		return OnboardingPayload{}, err
	}
	a, err := readLV(r, 48)
	if err != nil {
		return OnboardingPayload{}, err
	}
	b, err := readLV(r, 48)
	if err != nil {
		return OnboardingPayload{}, err
	}
	c, err := readLV(r, 16)
	if err != nil {
		return OnboardingPayload{}, err
	}
	tag, err := readLV(r, 32)
	if err != nil {
		return OnboardingPayload{}, err
	}
	if r.Len() != 0 {
		return OnboardingPayload{}, fmt.Errorf("onboarding: trailing bytes")
	}
	return OnboardingPayload{
		IssuedAtUnix:  int64(issued),
		ExpiresAtUnix: int64(expires),
		Family:        OnboardingFamily(family),
		Port:          int(port),
		SignerPub:     ed25519.PublicKey(signer),
		Host:          string(host),
		SNI:           string(sni),
		CredentialA:   a,
		CredentialB:   b,
		CredentialC:   c,
		Tag:           string(tag),
	}, nil
}

func validateOnboardingPayload(p OnboardingPayload) error {
	if p.IssuedAtUnix <= 0 || p.ExpiresAtUnix <= p.IssuedAtUnix {
		return fmt.Errorf("onboarding: invalid validity window")
	}
	if len(p.SignerPub) != ed25519.PublicKeySize {
		return fmt.Errorf("onboarding: invalid signer key")
	}
	host := strings.TrimSpace(p.Host)
	if host == "" || strings.ContainsAny(host, `/\:@?&#%`) || strings.Contains(host, "..") {
		return fmt.Errorf("onboarding: invalid host")
	}
	if ip := net.ParseIP(host); ip == nil && !onboardingHostPattern.MatchString(host) {
		return fmt.Errorf("onboarding: invalid host")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("onboarding: invalid port")
	}
	if p.SNI != "" && (!onboardingSNIRegex.MatchString(p.SNI) || strings.Contains(p.SNI, "..")) {
		return fmt.Errorf("onboarding: invalid sni")
	}
	if p.Tag != "" && !onboardingTagRegex.MatchString(p.Tag) {
		return fmt.Errorf("onboarding: invalid tag")
	}
	switch p.Family {
	case OnboardingFamilyReality:
		if len(p.CredentialA) != 16 || len(p.CredentialB) != 32 || len(p.CredentialC) == 0 || len(p.CredentialC) > 8 {
			return fmt.Errorf("onboarding: invalid reality credentials")
		}
	case OnboardingFamilyHysteria2:
		if !safeSecretBytes(p.CredentialA, 1, 48) || !safeSecretBytes(p.CredentialB, 1, 48) {
			return fmt.Errorf("onboarding: invalid hysteria2 credentials")
		}
	case OnboardingFamilyTUIC:
		if len(p.CredentialA) != 16 || !safeSecretBytes(p.CredentialB, 1, 48) {
			return fmt.Errorf("onboarding: invalid tuic credentials")
		}
	case OnboardingFamilyShadowTLS, OnboardingFamilyDNS:
	default:
		return fmt.Errorf("onboarding: unsupported family")
	}
	return nil
}

func onboardingSignatureMessage(body []byte) []byte {
	out := make([]byte, 0, len(onboardingSignDomain)+len(body))
	out = append(out, onboardingSignDomain...)
	out = append(out, body...)
	return out
}

func trustedOnboardingSigner(pub ed25519.PublicKey, trusted []ed25519.PublicKey) bool {
	for _, k := range trusted {
		if len(k) == ed25519.PublicKeySize && bytes.Equal(k, pub) {
			return true
		}
	}
	return false
}

func writeLV(buf *bytes.Buffer, b []byte) {
	if len(b) > 255 {
		panic("onboarding lv too large")
	}
	buf.WriteByte(byte(len(b)))
	buf.Write(b)
}

func readLV(r *bytes.Reader, max int) ([]byte, error) {
	n, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("onboarding: malformed body")
	}
	if int(n) > max || int(n) > r.Len() {
		return nil, fmt.Errorf("onboarding: invalid field length")
	}
	out := make([]byte, int(n))
	if _, err := r.Read(out); err != nil {
		return nil, fmt.Errorf("onboarding: malformed body")
	}
	return out, nil
}

func putU32(buf *bytes.Buffer, v int64) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(v))
	buf.Write(tmp[:])
}

func putU16(buf *bytes.Buffer, v int) {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], uint16(v))
	buf.Write(tmp[:])
}

func safeSecretBytes(b []byte, min int, max int) bool {
	if len(b) < min || len(b) > max {
		return false
	}
	for _, c := range b {
		if c < 0x21 || c > 0x7e || c == '/' || c == '\\' {
			return false
		}
	}
	return true
}

func uuidString(raw []byte) string {
	if len(raw) != 16 {
		return ""
	}
	hexed := hex.EncodeToString(raw)
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}

func GenerateOnboardingNonce(n int) ([]byte, error) {
	if n <= 0 || n > 48 {
		return nil, fmt.Errorf("onboarding: invalid nonce size")
	}
	out := make([]byte, n)
	if _, err := rand.Read(out); err != nil {
		return nil, err
	}
	return out, nil
}

var ErrOnboardingUnsupported = errors.New("onboarding: unsupported")
