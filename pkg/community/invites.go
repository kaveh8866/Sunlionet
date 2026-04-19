package community

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type InviteID string

type Invite struct {
	Schema       int         `json:"schema"`
	ID           InviteID    `json:"id"`
	Community    CommunityID `json:"community"`
	IssuerPubB64 string      `json:"issuer_pub_b64url"`

	IssuedAt  int64 `json:"issued_at"`
	ExpiresAt int64 `json:"expires_at"`
	MaxUses   int   `json:"max_uses,omitempty"`

	NonceB64 string `json:"nonce_b64url"`
	SigB64   string `json:"sig_b64url"`
}

type inviteSignable struct {
	Schema       int         `json:"schema"`
	ID           InviteID    `json:"id"`
	Community    CommunityID `json:"community"`
	IssuerPubB64 string      `json:"issuer_pub_b64url"`
	IssuedAt     int64       `json:"issued_at"`
	ExpiresAt    int64       `json:"expires_at"`
	MaxUses      int         `json:"max_uses,omitempty"`
	NonceB64     string      `json:"nonce_b64url"`
}

func NewInvite(communityID CommunityID, issuerPubB64 string, issuerPriv ed25519.PrivateKey, ttl time.Duration, maxUses int) (*Invite, error) {
	if communityID == "" {
		return nil, errors.New("community: community id required")
	}
	if issuerPubB64 == "" {
		return nil, errors.New("community: issuer pub required")
	}
	if len(issuerPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("community: invalid issuer priv key")
	}
	pub, err := base64.RawURLEncoding.DecodeString(issuerPubB64)
	if err != nil {
		return nil, fmt.Errorf("community: decode issuer pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("community: invalid issuer pub size: %d", len(pub))
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if ttl > 30*24*time.Hour {
		return nil, errors.New("community: ttl too large")
	}
	if maxUses < 0 {
		return nil, errors.New("community: max_uses must be >= 0")
	}
	if maxUses > 100 {
		return nil, errors.New("community: max_uses too large")
	}

	id, err := newInviteID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	signable := inviteSignable{
		Schema:       SchemaV1,
		ID:           id,
		Community:    communityID,
		IssuerPubB64: issuerPubB64,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(ttl).Unix(),
		MaxUses:      maxUses,
		NonceB64:     nonce,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(issuerPriv, raw)
	return &Invite{
		Schema:       signable.Schema,
		ID:           signable.ID,
		Community:    signable.Community,
		IssuerPubB64: signable.IssuerPubB64,
		IssuedAt:     signable.IssuedAt,
		ExpiresAt:    signable.ExpiresAt,
		MaxUses:      signable.MaxUses,
		NonceB64:     signable.NonceB64,
		SigB64:       base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func (inv *Invite) Validate(now time.Time) error {
	if inv == nil {
		return errors.New("community: invite is nil")
	}
	if inv.Schema != SchemaV1 {
		return fmt.Errorf("community: unsupported schema: %d", inv.Schema)
	}
	if inv.ID == "" {
		return errors.New("community: invite id required")
	}
	if inv.Community == "" {
		return errors.New("community: invite community required")
	}
	if inv.IssuerPubB64 == "" {
		return errors.New("community: invite issuer pub required")
	}
	if inv.IssuedAt <= 0 || inv.ExpiresAt <= 0 {
		return errors.New("community: issued_at/expires_at required")
	}
	if inv.ExpiresAt < inv.IssuedAt {
		return errors.New("community: expires_at before issued_at")
	}
	if now.Unix() > inv.ExpiresAt {
		return errors.New("community: invite expired")
	}
	if inv.MaxUses < 0 {
		return errors.New("community: max_uses must be >= 0")
	}
	if inv.NonceB64 == "" || inv.SigB64 == "" {
		return errors.New("community: nonce/sig required")
	}
	return inv.VerifySignature()
}

func (inv *Invite) VerifySignature() error {
	if inv == nil {
		return errors.New("community: invite is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(inv.IssuerPubB64)
	if err != nil {
		return fmt.Errorf("community: decode issuer pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("community: invalid issuer pub size: %d", len(pub))
	}
	sig, err := base64.RawURLEncoding.DecodeString(inv.SigB64)
	if err != nil {
		return fmt.Errorf("community: decode sig: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("community: invalid sig size: %d", len(sig))
	}
	signable := inviteSignable{
		Schema:       inv.Schema,
		ID:           inv.ID,
		Community:    inv.Community,
		IssuerPubB64: inv.IssuerPubB64,
		IssuedAt:     inv.IssuedAt,
		ExpiresAt:    inv.ExpiresAt,
		MaxUses:      inv.MaxUses,
		NonceB64:     inv.NonceB64,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return errors.New("community: invalid invite signature")
	}
	return nil
}

func (inv *Invite) Encode() (string, error) {
	if inv == nil {
		return "", errors.New("community: invite is nil")
	}
	raw, err := json.Marshal(inv)
	if err != nil {
		return "", err
	}
	return "sn4inv:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeInvite(s string) (*Invite, error) {
	if s == "" {
		return nil, errors.New("community: invite string is empty")
	}
	if len(s) < 7 || s[:7] != "sn4inv:" {
		return nil, errors.New("community: invalid invite prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[7:])
	if err != nil {
		return nil, fmt.Errorf("community: decode invite: %w", err)
	}
	var inv Invite
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, err
	}
	if err := inv.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &inv, nil
}

type JoinRequest struct {
	Schema    int         `json:"schema"`
	InviteID  InviteID    `json:"invite_id"`
	Community CommunityID `json:"community"`

	ApplicantPubB64 string `json:"applicant_pub_b64url"`

	CreatedAt int64  `json:"created_at"`
	NonceB64  string `json:"nonce_b64url"`
	SigB64    string `json:"sig_b64url"`
}

type joinReqSignable struct {
	Schema          int         `json:"schema"`
	InviteID        InviteID    `json:"invite_id"`
	Community       CommunityID `json:"community"`
	ApplicantPubB64 string      `json:"applicant_pub_b64url"`
	CreatedAt       int64       `json:"created_at"`
	NonceB64        string      `json:"nonce_b64url"`
}

func NewJoinRequest(inv *Invite, applicantPubB64 string, applicantPriv ed25519.PrivateKey) (*JoinRequest, error) {
	if inv == nil {
		return nil, errors.New("community: invite is nil")
	}
	if err := inv.Validate(time.Now()); err != nil {
		return nil, err
	}
	if applicantPubB64 == "" {
		return nil, errors.New("community: applicant pub required")
	}
	if len(applicantPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("community: invalid applicant priv key")
	}
	apub, err := base64.RawURLEncoding.DecodeString(applicantPubB64)
	if err != nil {
		return nil, fmt.Errorf("community: decode applicant pub: %w", err)
	}
	if len(apub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("community: invalid applicant pub size: %d", len(apub))
	}
	now := time.Now()
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	signable := joinReqSignable{
		Schema:          SchemaV1,
		InviteID:        inv.ID,
		Community:       inv.Community,
		ApplicantPubB64: applicantPubB64,
		CreatedAt:       now.Unix(),
		NonceB64:        nonce,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(applicantPriv, raw)
	return &JoinRequest{
		Schema:          signable.Schema,
		InviteID:        signable.InviteID,
		Community:       signable.Community,
		ApplicantPubB64: signable.ApplicantPubB64,
		CreatedAt:       signable.CreatedAt,
		NonceB64:        signable.NonceB64,
		SigB64:          base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func (r *JoinRequest) Validate(now time.Time) error {
	if r == nil {
		return errors.New("community: join request is nil")
	}
	if r.Schema != SchemaV1 {
		return fmt.Errorf("community: unsupported schema: %d", r.Schema)
	}
	if r.InviteID == "" || r.Community == "" {
		return errors.New("community: invite_id/community required")
	}
	if r.ApplicantPubB64 == "" {
		return errors.New("community: applicant pub required")
	}
	if r.CreatedAt <= 0 {
		return errors.New("community: created_at required")
	}
	if now.Unix() < r.CreatedAt-300 || now.Unix() > r.CreatedAt+7*24*3600 {
		return errors.New("community: join request time window invalid")
	}
	if r.NonceB64 == "" || r.SigB64 == "" {
		return errors.New("community: nonce/sig required")
	}
	return r.VerifySignature()
}

func (r *JoinRequest) VerifySignature() error {
	if r == nil {
		return errors.New("community: join request is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(r.ApplicantPubB64)
	if err != nil {
		return fmt.Errorf("community: decode applicant pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("community: invalid applicant pub size: %d", len(pub))
	}
	sig, err := base64.RawURLEncoding.DecodeString(r.SigB64)
	if err != nil {
		return fmt.Errorf("community: decode sig: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("community: invalid sig size: %d", len(sig))
	}
	signable := joinReqSignable{
		Schema:          r.Schema,
		InviteID:        r.InviteID,
		Community:       r.Community,
		ApplicantPubB64: r.ApplicantPubB64,
		CreatedAt:       r.CreatedAt,
		NonceB64:        r.NonceB64,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return errors.New("community: invalid join request signature")
	}
	return nil
}

func (r *JoinRequest) Encode() (string, error) {
	if r == nil {
		return "", errors.New("community: join request is nil")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return "sn4jr:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeJoinRequest(s string) (*JoinRequest, error) {
	if s == "" {
		return nil, errors.New("community: join request string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4jr:" {
		return nil, errors.New("community: invalid join request prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("community: decode join request: %w", err)
	}
	var req JoinRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if err := req.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &req, nil
}

type JoinApproval struct {
	Schema    int         `json:"schema"`
	InviteID  InviteID    `json:"invite_id"`
	Community CommunityID `json:"community"`

	ApplicantPubB64 string `json:"applicant_pub_b64url"`
	GrantedRole     Role   `json:"granted_role"`

	IssuerPubB64 string `json:"issuer_pub_b64url"`
	IssuedAt     int64  `json:"issued_at"`
	NonceB64     string `json:"nonce_b64url"`
	SigB64       string `json:"sig_b64url"`
}

type joinApprovalSignable struct {
	Schema          int         `json:"schema"`
	InviteID        InviteID    `json:"invite_id"`
	Community       CommunityID `json:"community"`
	ApplicantPubB64 string      `json:"applicant_pub_b64url"`
	GrantedRole     Role        `json:"granted_role"`
	IssuerPubB64    string      `json:"issuer_pub_b64url"`
	IssuedAt        int64       `json:"issued_at"`
	NonceB64        string      `json:"nonce_b64url"`
}

func ApproveJoin(inv *Invite, req *JoinRequest, issuerPubB64 string, issuerPriv ed25519.PrivateKey, role Role) (*JoinApproval, error) {
	if inv == nil || req == nil {
		return nil, errors.New("community: invite/request required")
	}
	if err := inv.Validate(time.Now()); err != nil {
		return nil, err
	}
	if err := req.Validate(time.Now()); err != nil {
		return nil, err
	}
	if inv.ID != req.InviteID || inv.Community != req.Community {
		return nil, errors.New("community: invite/request mismatch")
	}
	switch role {
	case RoleOwner, RoleModerator, RoleMember:
	default:
		return nil, fmt.Errorf("community: invalid role: %q", role)
	}
	if issuerPubB64 == "" {
		return nil, errors.New("community: issuer pub required")
	}
	if len(issuerPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("community: invalid issuer priv key")
	}

	now := time.Now()
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	signable := joinApprovalSignable{
		Schema:          SchemaV1,
		InviteID:        inv.ID,
		Community:       inv.Community,
		ApplicantPubB64: req.ApplicantPubB64,
		GrantedRole:     role,
		IssuerPubB64:    issuerPubB64,
		IssuedAt:        now.Unix(),
		NonceB64:        nonce,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(issuerPriv, raw)
	return &JoinApproval{
		Schema:          signable.Schema,
		InviteID:        signable.InviteID,
		Community:       signable.Community,
		ApplicantPubB64: signable.ApplicantPubB64,
		GrantedRole:     signable.GrantedRole,
		IssuerPubB64:    signable.IssuerPubB64,
		IssuedAt:        signable.IssuedAt,
		NonceB64:        signable.NonceB64,
		SigB64:          base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func (a *JoinApproval) Validate(now time.Time) error {
	if a == nil {
		return errors.New("community: join approval is nil")
	}
	if a.Schema != SchemaV1 {
		return fmt.Errorf("community: unsupported schema: %d", a.Schema)
	}
	if a.InviteID == "" || a.Community == "" {
		return errors.New("community: invite_id/community required")
	}
	if a.ApplicantPubB64 == "" || a.IssuerPubB64 == "" {
		return errors.New("community: applicant/issuer pub required")
	}
	switch a.GrantedRole {
	case RoleOwner, RoleModerator, RoleMember:
	default:
		return fmt.Errorf("community: invalid role: %q", a.GrantedRole)
	}
	if a.IssuedAt <= 0 {
		return errors.New("community: issued_at required")
	}
	if now.Unix() < a.IssuedAt-300 || now.Unix() > a.IssuedAt+30*24*3600 {
		return errors.New("community: approval time window invalid")
	}
	if a.NonceB64 == "" || a.SigB64 == "" {
		return errors.New("community: nonce/sig required")
	}
	return a.VerifySignature()
}

func (a *JoinApproval) VerifySignature() error {
	if a == nil {
		return errors.New("community: join approval is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(a.IssuerPubB64)
	if err != nil {
		return fmt.Errorf("community: decode issuer pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("community: invalid issuer pub size: %d", len(pub))
	}
	sig, err := base64.RawURLEncoding.DecodeString(a.SigB64)
	if err != nil {
		return fmt.Errorf("community: decode sig: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("community: invalid sig size: %d", len(sig))
	}
	signable := joinApprovalSignable{
		Schema:          a.Schema,
		InviteID:        a.InviteID,
		Community:       a.Community,
		ApplicantPubB64: a.ApplicantPubB64,
		GrantedRole:     a.GrantedRole,
		IssuerPubB64:    a.IssuerPubB64,
		IssuedAt:        a.IssuedAt,
		NonceB64:        a.NonceB64,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return errors.New("community: invalid join approval signature")
	}
	return nil
}

func (a *JoinApproval) Encode() (string, error) {
	if a == nil {
		return "", errors.New("community: join approval is nil")
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return "sn4ja:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeJoinApproval(s string) (*JoinApproval, error) {
	if s == "" {
		return nil, errors.New("community: join approval string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4ja:" {
		return nil, errors.New("community: invalid join approval prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("community: decode join approval: %w", err)
	}
	var a JoinApproval
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if err := a.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &a, nil
}

func newInviteID() (InviteID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return InviteID(base64.RawURLEncoding.EncodeToString(b[:])), nil
}

func newNonceB64(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("community: nonce size must be positive")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
