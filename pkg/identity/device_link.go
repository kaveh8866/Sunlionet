package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"filippo.io/age"
)

const (
	maxDeviceLinkEncodedLen  = 64 * 1024
	maxDeviceLinkRawLen      = 64 * 1024
	maxDeviceLinkLifetimeSec = 10 * 60
)

type DeviceLinkBundle struct {
	Schema int `json:"schema"`

	Persona     Persona           `json:"persona"`
	JoinPackage DeviceJoinPackage `json:"join_package"`

	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	NonceB64  string `json:"nonce_b64url"`
}

func NewDeviceLinkBundle(persona *Persona, req *DeviceJoinRequest, pkg *DeviceJoinPackage) (*DeviceLinkBundle, error) {
	if persona == nil {
		return nil, errors.New("identity: persona is nil")
	}
	if req == nil {
		return nil, errors.New("identity: join request is nil")
	}
	if pkg == nil {
		return nil, errors.New("identity: join package is nil")
	}
	if err := persona.Validate(); err != nil {
		return nil, err
	}
	if err := req.Validate(time.Now()); err != nil {
		return nil, err
	}
	if pkg.PersonaID != persona.ID || pkg.DeviceID != req.DeviceID {
		return nil, errors.New("identity: link bundle mismatch")
	}
	now := time.Now().Unix()
	return &DeviceLinkBundle{
		Schema:      SchemaV1,
		Persona:     *persona,
		JoinPackage: *pkg,
		CreatedAt:   now,
		ExpiresAt:   min64(req.ExpiresAt, now+(10*60)),
		NonceB64:    req.NonceB64,
	}, nil
}

func (b *DeviceLinkBundle) Validate(now time.Time) error {
	if b == nil {
		return errors.New("identity: link bundle is nil")
	}
	if b.Schema != SchemaV1 {
		return fmt.Errorf("identity: unsupported link bundle schema: %d", b.Schema)
	}
	if b.CreatedAt <= 0 || b.ExpiresAt <= 0 || b.ExpiresAt < b.CreatedAt {
		return errors.New("identity: invalid link bundle timestamps")
	}
	if b.ExpiresAt-b.CreatedAt > maxDeviceLinkLifetimeSec {
		return errors.New("identity: link bundle lifetime too long")
	}
	if now.Unix() > b.ExpiresAt {
		return errors.New("identity: link bundle expired")
	}
	if strings.TrimSpace(b.NonceB64) == "" {
		return errors.New("identity: link bundle nonce required")
	}
	if err := b.Persona.Validate(); err != nil {
		return err
	}
	if b.JoinPackage.PersonaID != b.Persona.ID {
		return errors.New("identity: link bundle persona mismatch")
	}
	if err := b.JoinPackage.Validate(b.Persona.SignPubB64); err != nil {
		return err
	}
	return nil
}

func (b *DeviceLinkBundle) Encode() (string, error) {
	if b == nil {
		return "", errors.New("identity: link bundle is nil")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return "sn4dl:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeDeviceLinkBundle(s string) (*DeviceLinkBundle, error) {
	if s == "" {
		return nil, errors.New("identity: link bundle string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4dl:" {
		return nil, errors.New("identity: invalid link bundle prefix")
	}
	if len(s) > maxDeviceLinkEncodedLen {
		return nil, errors.New("identity: link bundle too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode link bundle: %w", err)
	}
	if len(raw) > maxDeviceLinkRawLen {
		return nil, errors.New("identity: link bundle too large")
	}
	var b DeviceLinkBundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	if err := b.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &b, nil
}

type DeviceLinkEnvelope struct {
	Schema int `json:"schema"`

	Cipher         string `json:"cipher"`
	SenderPubB64   string `json:"sender_pub_b64url"`
	RecipientKeyID string `json:"recipient_key_id"`

	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	NonceB64  string `json:"nonce_b64url"`

	CiphertextB64 string `json:"ciphertext_b64url"`
	SigB64        string `json:"sig_b64url"`
}

func (e *DeviceLinkEnvelope) Validate(now time.Time) error {
	if e == nil {
		return errors.New("identity: link envelope is nil")
	}
	if e.Schema != SchemaV1 {
		return fmt.Errorf("identity: unsupported link envelope schema: %d", e.Schema)
	}
	if e.Cipher != "age-x25519" {
		return errors.New("identity: unsupported link envelope cipher")
	}
	if strings.TrimSpace(e.SenderPubB64) == "" {
		return errors.New("identity: sender_pub_b64url required")
	}
	if strings.TrimSpace(e.RecipientKeyID) == "" {
		return errors.New("identity: recipient_key_id required")
	}
	if e.CreatedAt <= 0 || e.ExpiresAt <= 0 || e.ExpiresAt < e.CreatedAt {
		return errors.New("identity: invalid link envelope timestamps")
	}
	if e.ExpiresAt-e.CreatedAt > maxDeviceLinkLifetimeSec {
		return errors.New("identity: link envelope lifetime too long")
	}
	if now.Unix() > e.ExpiresAt {
		return errors.New("identity: link envelope expired")
	}
	if strings.TrimSpace(e.NonceB64) == "" {
		return errors.New("identity: nonce required")
	}
	if strings.TrimSpace(e.CiphertextB64) == "" {
		return errors.New("identity: ciphertext required")
	}
	if strings.TrimSpace(e.SigB64) == "" {
		return errors.New("identity: signature required")
	}
	return nil
}

func (e *DeviceLinkEnvelope) Encode() (string, error) {
	if e == nil {
		return "", errors.New("identity: link envelope is nil")
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return "sn4dl:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeDeviceLinkEnvelope(s string) (*DeviceLinkEnvelope, error) {
	if s == "" {
		return nil, errors.New("identity: link envelope string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4dl:" {
		return nil, errors.New("identity: invalid link envelope prefix")
	}
	if len(s) > maxDeviceLinkEncodedLen {
		return nil, errors.New("identity: link envelope too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode link envelope: %w", err)
	}
	if len(raw) > maxDeviceLinkRawLen {
		return nil, errors.New("identity: link envelope too large")
	}
	var e DeviceLinkEnvelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	if e.Cipher == "" {
		return nil, errors.New("identity: not a link envelope")
	}
	if err := e.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &e, nil
}

func EncryptDeviceLinkBundle(persona *Persona, recipient string, bundle *DeviceLinkBundle) (string, error) {
	if persona == nil {
		return "", errors.New("identity: persona is nil")
	}
	if bundle == nil {
		return "", errors.New("identity: bundle is nil")
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", errors.New("identity: recipient required")
	}
	if err := bundle.Validate(time.Now()); err != nil {
		return "", err
	}
	if bundle.Persona.ID != persona.ID {
		return "", errors.New("identity: bundle persona mismatch")
	}
	r, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return "", fmt.Errorf("identity: parse age recipient: %w", err)
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return "", err
	}
	var ciphertextBuf bytes.Buffer
	w, err := age.Encrypt(&ciphertextBuf, r)
	if err != nil {
		return "", fmt.Errorf("identity: age encrypt: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return "", fmt.Errorf("identity: age encrypt write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("identity: age encrypt close: %w", err)
	}
	ciphertext := ciphertextBuf.Bytes()

	env := DeviceLinkEnvelope{
		Schema:         SchemaV1,
		Cipher:         "age-x25519",
		SenderPubB64:   persona.SignPubB64,
		RecipientKeyID: "age-x25519:" + fingerprint16(recipient),
		CreatedAt:      bundle.CreatedAt,
		ExpiresAt:      bundle.ExpiresAt,
		NonceB64:       bundle.NonceB64,
		CiphertextB64:  base64.RawURLEncoding.EncodeToString(ciphertext),
		SigB64:         "",
	}

	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}
	envCopy := env
	envCopy.SigB64 = ""
	headerBytes, err := json.Marshal(envCopy)
	if err != nil {
		return "", err
	}
	var sigInput bytes.Buffer
	sigInput.Write(headerBytes)
	sigInput.Write(ciphertext)
	sig := ed25519.Sign(ed25519.PrivateKey(priv), sigInput.Bytes())
	env.SigB64 = base64.RawURLEncoding.EncodeToString(sig)

	return env.Encode()
}

func DecryptDeviceLink(s string, ageIdentity string) (*DeviceLinkBundle, error) {
	if s == "" {
		return nil, errors.New("identity: link string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4dl:" {
		return nil, errors.New("identity: invalid link prefix")
	}
	if len(s) > maxDeviceLinkEncodedLen {
		return nil, errors.New("identity: link string too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode link: %w", err)
	}
	if len(raw) > maxDeviceLinkRawLen {
		return nil, errors.New("identity: link too large")
	}

	var probe struct {
		Cipher string `json:"cipher"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if strings.TrimSpace(probe.Cipher) == "" {
		return DecodeDeviceLinkBundle(s)
	}

	env, err := DecodeDeviceLinkEnvelope(s)
	if err != nil {
		return nil, err
	}
	if err := env.Validate(time.Now()); err != nil {
		return nil, err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(env.CiphertextB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode ciphertext: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(env.SigB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode signature: %w", err)
	}
	pub, err := base64.RawURLEncoding.DecodeString(env.SenderPubB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode sender pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("identity: invalid sender pub size")
	}
	envCopy := *env
	envCopy.SigB64 = ""
	headerBytes, err := json.Marshal(envCopy)
	if err != nil {
		return nil, err
	}
	var sigInput bytes.Buffer
	sigInput.Write(headerBytes)
	sigInput.Write(ciphertext)
	if !ed25519.Verify(ed25519.PublicKey(pub), sigInput.Bytes(), sigBytes) {
		return nil, errors.New("identity: invalid link envelope signature")
	}

	ageIdentity = strings.TrimSpace(ageIdentity)
	if ageIdentity == "" {
		return nil, errors.New("identity: age identity required")
	}
	id, err := age.ParseX25519Identity(ageIdentity)
	if err != nil {
		return nil, fmt.Errorf("identity: parse age identity: %w", err)
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		return nil, fmt.Errorf("identity: age decrypt: %w", err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("identity: read decrypted payload: %w", err)
	}
	var b DeviceLinkBundle
	if err := json.Unmarshal(plaintext, &b); err != nil {
		return nil, err
	}
	if err := b.Validate(time.Now()); err != nil {
		return nil, err
	}
	if b.Persona.SignPubB64 != env.SenderPubB64 {
		return nil, errors.New("identity: sender mismatch")
	}
	if b.CreatedAt != env.CreatedAt || b.ExpiresAt != env.ExpiresAt {
		return nil, errors.New("identity: timestamp mismatch")
	}
	if b.NonceB64 != env.NonceB64 {
		return nil, errors.New("identity: nonce mismatch")
	}
	return &b, nil
}

func DeviceLinkSAS(s string) (string, error) {
	if s == "" {
		return "", errors.New("identity: link string is empty")
	}
	env, err := DecodeDeviceLinkEnvelope(s)
	if err == nil {
		return sasFromEnvelope(env)
	}
	b, err2 := DecodeDeviceLinkBundle(s)
	if err2 != nil {
		return "", err
	}
	return sasFromBundle(b)
}

func sasFromEnvelope(e *DeviceLinkEnvelope) (string, error) {
	if err := e.Validate(time.Now()); err != nil {
		return "", err
	}
	pub, err := base64.RawURLEncoding.DecodeString(e.SenderPubB64)
	if err != nil {
		return "", errors.New("identity: invalid sender pub")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(e.NonceB64)
	if err != nil {
		return "", errors.New("identity: invalid nonce")
	}
	ct, err := base64.RawURLEncoding.DecodeString(e.CiphertextB64)
	if err != nil {
		return "", errors.New("identity: invalid ciphertext")
	}
	sum := sha256.New()
	_, _ = sum.Write([]byte("sn4dl-sas-v1"))
	_, _ = sum.Write(pub)
	_, _ = sum.Write([]byte(e.RecipientKeyID))
	_, _ = sum.Write(nonce)
	ctSum := sha256Sum(ct)
	_, _ = sum.Write(ctSum[:])
	return formatSAS(sum.Sum(nil)), nil
}

func sasFromBundle(b *DeviceLinkBundle) (string, error) {
	if err := b.Validate(time.Now()); err != nil {
		return "", err
	}
	pub, err := base64.RawURLEncoding.DecodeString(b.Persona.SignPubB64)
	if err != nil {
		return "", errors.New("identity: invalid persona pub")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(b.NonceB64)
	if err != nil {
		return "", errors.New("identity: invalid nonce")
	}
	sum := sha256.New()
	_, _ = sum.Write([]byte("sn4dl-sas-v1"))
	_, _ = sum.Write(pub)
	_, _ = sum.Write([]byte(b.JoinPackage.DeviceID))
	_, _ = sum.Write(nonce)
	return formatSAS(sum.Sum(nil)), nil
}

func formatSAS(sum []byte) string {
	if len(sum) < 8 {
		return ""
	}
	v := binary.BigEndian.Uint64(sum[:8]) % 1_000_000_000_000
	a := v / 100_000_000
	b := (v / 10_000) % 10_000
	c := v % 10_000
	return fmt.Sprintf("%04d-%04d-%04d", a, b, c)
}

type DeviceJoinRequest struct {
	Schema    int       `json:"schema"`
	PersonaID PersonaID `json:"persona_id"`
	DeviceID  string    `json:"device_id"`

	DevicePubB64 string `json:"device_pub_b64url"`
	AgeRecipient string `json:"age_recipient,omitempty"`

	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	NonceB64  string `json:"nonce_b64url"`
}

func NewDeviceJoinRequest(personaID PersonaID, devicePubB64 string) (*DeviceJoinRequest, error) {
	deviceID, err := newDeviceID()
	if err != nil {
		return nil, err
	}
	return NewDeviceJoinRequestForDevice(personaID, deviceID, devicePubB64, "")
}

func NewDeviceJoinRequestForDevice(personaID PersonaID, deviceID string, devicePubB64 string, ageRecipient string) (*DeviceJoinRequest, error) {
	if personaID == "" {
		return nil, errors.New("identity: persona_id required")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("identity: device_id required")
	}
	if devicePubB64 == "" {
		return nil, errors.New("identity: device_pub_b64url required")
	}
	pub, err := base64.RawURLEncoding.DecodeString(devicePubB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode device_pub_b64url: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("identity: invalid device pub size: %d", len(pub))
	}
	ageRecipient = strings.TrimSpace(ageRecipient)
	if ageRecipient != "" {
		if _, err := age.ParseX25519Recipient(ageRecipient); err != nil {
			return nil, fmt.Errorf("identity: invalid age recipient: %w", err)
		}
	}
	now := time.Now()
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	return &DeviceJoinRequest{
		Schema:       SchemaV1,
		PersonaID:    personaID,
		DeviceID:     deviceID,
		DevicePubB64: devicePubB64,
		AgeRecipient: ageRecipient,
		CreatedAt:    now.Unix(),
		ExpiresAt:    now.Add(10 * time.Minute).Unix(),
		NonceB64:     nonce,
	}, nil
}

func (r *DeviceJoinRequest) Validate(now time.Time) error {
	if r == nil {
		return errors.New("identity: join request is nil")
	}
	if r.Schema != SchemaV1 {
		return fmt.Errorf("identity: unsupported schema: %d", r.Schema)
	}
	if r.PersonaID == "" {
		return errors.New("identity: persona_id required")
	}
	if r.DeviceID == "" {
		return errors.New("identity: device_id required")
	}
	if r.CreatedAt <= 0 || r.ExpiresAt <= 0 {
		return errors.New("identity: created_at/expires_at required")
	}
	if r.ExpiresAt < r.CreatedAt {
		return errors.New("identity: expires_at before created_at")
	}
	if now.Unix() > r.ExpiresAt {
		return errors.New("identity: join request expired")
	}
	if r.NonceB64 == "" {
		return errors.New("identity: nonce required")
	}
	pub, err := base64.RawURLEncoding.DecodeString(r.DevicePubB64)
	if err != nil {
		return fmt.Errorf("identity: decode device_pub_b64url: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("identity: invalid device pub size: %d", len(pub))
	}
	if strings.TrimSpace(r.AgeRecipient) != "" {
		if _, err := age.ParseX25519Recipient(strings.TrimSpace(r.AgeRecipient)); err != nil {
			return fmt.Errorf("identity: invalid age recipient: %w", err)
		}
	}
	return nil
}

func (r *DeviceJoinRequest) Encode() (string, error) {
	if r == nil {
		return "", errors.New("identity: join request is nil")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return "sn4dj:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeDeviceJoinRequest(s string) (*DeviceJoinRequest, error) {
	if s == "" {
		return nil, errors.New("identity: join request string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4dj:" {
		return nil, errors.New("identity: invalid join request prefix")
	}
	if len(s) > maxDeviceLinkEncodedLen {
		return nil, errors.New("identity: join request too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode join request: %w", err)
	}
	if len(raw) > maxDeviceLinkRawLen {
		return nil, errors.New("identity: join request too large")
	}
	var req DeviceJoinRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if err := req.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &req, nil
}

type DeviceJoinPackage struct {
	PersonaID PersonaID   `json:"persona_id"`
	DeviceID  string      `json:"device_id"`
	Trust     DeviceTrust `json:"trust"`

	DevicePubB64 string `json:"device_pub_b64url"`
	Cert         DeviceCert
	IssuedAt     int64 `json:"issued_at"`
}

func ApproveDeviceJoinRequest(persona *Persona, req *DeviceJoinRequest) (*DeviceJoinPackage, error) {
	if persona == nil {
		return nil, errors.New("identity: persona is nil")
	}
	if req == nil {
		return nil, errors.New("identity: join request is nil")
	}
	if req.PersonaID != persona.ID {
		return nil, errors.New("identity: persona mismatch")
	}
	if err := req.Validate(time.Now()); err != nil {
		return nil, err
	}
	cert, err := IssueDeviceCert(persona, req.DeviceID, req.DevicePubB64)
	if err != nil {
		return nil, err
	}
	return &DeviceJoinPackage{
		PersonaID:    req.PersonaID,
		DeviceID:     req.DeviceID,
		Trust:        DeviceTrusted,
		DevicePubB64: req.DevicePubB64,
		Cert:         *cert,
		IssuedAt:     cert.IssuedAt,
	}, nil
}

func (p *DeviceJoinPackage) Validate(personaPubB64 string) error {
	if p == nil {
		return errors.New("identity: join package is nil")
	}
	if p.PersonaID == "" || p.DeviceID == "" || p.DevicePubB64 == "" || p.IssuedAt <= 0 {
		return errors.New("identity: malformed join package")
	}
	if p.Trust != DeviceTrusted {
		return fmt.Errorf("identity: unexpected trust in join package: %q", p.Trust)
	}
	if err := VerifyDeviceCert(personaPubB64, &p.Cert); err != nil {
		return err
	}
	return nil
}

func (p *DeviceJoinPackage) Encode() (string, error) {
	if p == nil {
		return "", errors.New("identity: join package is nil")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "sn4dp:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeDeviceJoinPackage(s string) (*DeviceJoinPackage, error) {
	if s == "" {
		return nil, errors.New("identity: join package string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4dp:" {
		return nil, errors.New("identity: invalid join package prefix")
	}
	if len(s) > maxDeviceLinkEncodedLen {
		return nil, errors.New("identity: join package too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode join package: %w", err)
	}
	if len(raw) > maxDeviceLinkRawLen {
		return nil, errors.New("identity: join package too large")
	}
	var pkg DeviceJoinPackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func UpsertDeviceFromJoinPackage(state *State, personaPubB64 string, pkg *DeviceJoinPackage) error {
	if state == nil {
		return errors.New("identity: state is nil")
	}
	if pkg == nil {
		return errors.New("identity: join package is nil")
	}
	if err := pkg.Validate(personaPubB64); err != nil {
		return err
	}
	now := time.Now().Unix()
	for i := range state.Devices {
		if state.Devices[i].PersonaID == pkg.PersonaID && state.Devices[i].DeviceID == pkg.DeviceID {
			state.Devices[i].SignPubB64 = pkg.DevicePubB64
			state.Devices[i].Trust = DeviceTrusted
			state.Devices[i].RevokedAt = 0
			state.UpdatedAt = now
			return nil
		}
	}
	state.Devices = append(state.Devices, Device{
		PersonaID:   pkg.PersonaID,
		DeviceID:    pkg.DeviceID,
		CreatedAt:   now,
		Trust:       DeviceTrusted,
		Label:       "linked-device",
		SignPubB64:  pkg.DevicePubB64,
		SignPrivB64: "",
	})
	state.UpdatedAt = now
	return nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func fingerprint16(s string) string {
	sum := sha256Sum([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

func sha256Sum(b []byte) [32]byte {
	return sha256.Sum256(b)
}
