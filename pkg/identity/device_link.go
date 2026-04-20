package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DeviceJoinRequest struct {
	Schema    int       `json:"schema"`
	PersonaID PersonaID `json:"persona_id"`
	DeviceID  string    `json:"device_id"`

	DevicePubB64 string `json:"device_pub_b64url"`

	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	NonceB64  string `json:"nonce_b64url"`
}

func NewDeviceJoinRequest(personaID PersonaID, devicePubB64 string) (*DeviceJoinRequest, error) {
	deviceID, err := newDeviceID()
	if err != nil {
		return nil, err
	}
	return NewDeviceJoinRequestForDevice(personaID, deviceID, devicePubB64)
}

func NewDeviceJoinRequestForDevice(personaID PersonaID, deviceID string, devicePubB64 string) (*DeviceJoinRequest, error) {
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
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode join request: %w", err)
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
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode join package: %w", err)
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
