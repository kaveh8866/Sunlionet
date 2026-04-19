package community

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type MemberStatus string

const (
	MemberActive  MemberStatus = "active"
	MemberRevoked MemberStatus = "revoked"
	MemberBanned  MemberStatus = "banned"
)

type Member struct {
	Community        CommunityID  `json:"community"`
	MemberPubB64     string       `json:"member_pub_b64url"`
	Role             Role         `json:"role"`
	Status           MemberStatus `json:"status"`
	JoinedAt         int64        `json:"joined_at"`
	ApprovedByPubB64 string       `json:"approved_by_pub_b64url"`
	InviteID         InviteID     `json:"invite_id"`
}

type RevocationKind string

const (
	RevokeMember RevocationKind = "revoke"
	BanMember    RevocationKind = "ban"
)

type Revocation struct {
	Schema        int            `json:"schema"`
	Community     CommunityID    `json:"community"`
	Kind          RevocationKind `json:"kind"`
	SubjectPubB64 string         `json:"subject_pub_b64url"`

	IssuerPubB64 string `json:"issuer_pub_b64url"`
	IssuedAt     int64  `json:"issued_at"`
	NonceB64     string `json:"nonce_b64url"`
	SigB64       string `json:"sig_b64url"`
}

type revocationSignable struct {
	Schema        int            `json:"schema"`
	Community     CommunityID    `json:"community"`
	Kind          RevocationKind `json:"kind"`
	SubjectPubB64 string         `json:"subject_pub_b64url"`
	IssuerPubB64  string         `json:"issuer_pub_b64url"`
	IssuedAt      int64          `json:"issued_at"`
	NonceB64      string         `json:"nonce_b64url"`
}

func NewRevocation(community CommunityID, kind RevocationKind, subjectPubB64 string, issuerPubB64 string, issuerPriv ed25519.PrivateKey) (*Revocation, error) {
	if community == "" {
		return nil, errors.New("community: community required")
	}
	switch kind {
	case RevokeMember, BanMember:
	default:
		return nil, fmt.Errorf("community: invalid revocation kind: %q", kind)
	}
	if subjectPubB64 == "" {
		return nil, errors.New("community: subject pub required")
	}
	if issuerPubB64 == "" {
		return nil, errors.New("community: issuer pub required")
	}
	if len(issuerPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("community: invalid issuer priv key")
	}
	subjectPub, err := base64.RawURLEncoding.DecodeString(subjectPubB64)
	if err != nil {
		return nil, fmt.Errorf("community: decode subject pub: %w", err)
	}
	if len(subjectPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("community: invalid subject pub size: %d", len(subjectPub))
	}
	issuerPub, err := base64.RawURLEncoding.DecodeString(issuerPubB64)
	if err != nil {
		return nil, fmt.Errorf("community: decode issuer pub: %w", err)
	}
	if len(issuerPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("community: invalid issuer pub size: %d", len(issuerPub))
	}
	now := time.Now()
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	signable := revocationSignable{
		Schema:        SchemaV1,
		Community:     community,
		Kind:          kind,
		SubjectPubB64: subjectPubB64,
		IssuerPubB64:  issuerPubB64,
		IssuedAt:      now.Unix(),
		NonceB64:      nonce,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(issuerPriv, raw)
	return &Revocation{
		Schema:        signable.Schema,
		Community:     signable.Community,
		Kind:          signable.Kind,
		SubjectPubB64: signable.SubjectPubB64,
		IssuerPubB64:  signable.IssuerPubB64,
		IssuedAt:      signable.IssuedAt,
		NonceB64:      signable.NonceB64,
		SigB64:        base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func (r *Revocation) Validate(now time.Time) error {
	if r == nil {
		return errors.New("community: revocation is nil")
	}
	if r.Schema != SchemaV1 {
		return fmt.Errorf("community: unsupported schema: %d", r.Schema)
	}
	if r.Community == "" {
		return errors.New("community: community required")
	}
	switch r.Kind {
	case RevokeMember, BanMember:
	default:
		return fmt.Errorf("community: invalid revocation kind: %q", r.Kind)
	}
	if r.SubjectPubB64 == "" || r.IssuerPubB64 == "" {
		return errors.New("community: subject/issuer pub required")
	}
	if r.IssuedAt <= 0 {
		return errors.New("community: issued_at required")
	}
	if now.Unix() < r.IssuedAt-300 || now.Unix() > r.IssuedAt+30*24*3600 {
		return errors.New("community: revocation time window invalid")
	}
	if r.NonceB64 == "" || r.SigB64 == "" {
		return errors.New("community: nonce/sig required")
	}
	return r.VerifySignature()
}

func (r *Revocation) VerifySignature() error {
	if r == nil {
		return errors.New("community: revocation is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(r.IssuerPubB64)
	if err != nil {
		return fmt.Errorf("community: decode issuer pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("community: invalid issuer pub size: %d", len(pub))
	}
	sig, err := base64.RawURLEncoding.DecodeString(r.SigB64)
	if err != nil {
		return fmt.Errorf("community: decode sig: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("community: invalid sig size: %d", len(sig))
	}
	signable := revocationSignable{
		Schema:        r.Schema,
		Community:     r.Community,
		Kind:          r.Kind,
		SubjectPubB64: r.SubjectPubB64,
		IssuerPubB64:  r.IssuerPubB64,
		IssuedAt:      r.IssuedAt,
		NonceB64:      r.NonceB64,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return errors.New("community: invalid revocation signature")
	}
	return nil
}

func (r *Revocation) Encode() (string, error) {
	if r == nil {
		return "", errors.New("community: revocation is nil")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return "sn4rv:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeRevocation(s string) (*Revocation, error) {
	if s == "" {
		return nil, errors.New("community: revocation string is empty")
	}
	if len(s) < 6 || s[:6] != "sn4rv:" {
		return nil, errors.New("community: invalid revocation prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[6:])
	if err != nil {
		return nil, fmt.Errorf("community: decode revocation: %w", err)
	}
	var r Revocation
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if err := r.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &r, nil
}

type JoinRateLimit struct {
	MaxPerHour int
}

func (l JoinRateLimit) normalize() JoinRateLimit {
	out := l
	if out.MaxPerHour <= 0 {
		out.MaxPerHour = 5
	}
	if out.MaxPerHour > 50 {
		out.MaxPerHour = 50
	}
	return out
}

type Registry struct {
	mu sync.Mutex

	invites      map[InviteID]Invite
	inviteUses   map[InviteID]int
	members      map[CommunityID]map[string]Member
	joinAttempts map[CommunityID]map[string][]int64
}

func NewRegistry() *Registry {
	return &Registry{
		invites:      make(map[InviteID]Invite),
		inviteUses:   make(map[InviteID]int),
		members:      make(map[CommunityID]map[string]Member),
		joinAttempts: make(map[CommunityID]map[string][]int64),
	}
}

func (r *Registry) AddInvite(inv *Invite) error {
	if r == nil {
		return errors.New("community: registry is nil")
	}
	if inv == nil {
		return errors.New("community: invite is nil")
	}
	if err := inv.Validate(time.Now()); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invites[inv.ID] = *inv
	if _, ok := r.inviteUses[inv.ID]; !ok {
		r.inviteUses[inv.ID] = 0
	}
	return nil
}

func (r *Registry) ApplyJoin(inv *Invite, req *JoinRequest, approval *JoinApproval, rate JoinRateLimit) (Member, error) {
	if r == nil {
		return Member{}, errors.New("community: registry is nil")
	}
	if inv == nil || req == nil || approval == nil {
		return Member{}, errors.New("community: invite/request/approval required")
	}
	now := time.Now()
	if err := inv.Validate(now); err != nil {
		return Member{}, err
	}
	if err := req.Validate(now); err != nil {
		return Member{}, err
	}
	if err := approval.Validate(now); err != nil {
		return Member{}, err
	}
	if inv.ID != req.InviteID || inv.Community != req.Community {
		return Member{}, errors.New("community: invite/request mismatch")
	}
	if approval.InviteID != inv.ID || approval.Community != inv.Community || approval.ApplicantPubB64 != req.ApplicantPubB64 {
		return Member{}, errors.New("community: approval mismatch")
	}
	if approval.GrantedRole != RoleMember && approval.GrantedRole != RoleModerator && approval.GrantedRole != RoleOwner {
		return Member{}, errors.New("community: invalid granted role")
	}

	rate = rate.normalize()

	r.mu.Lock()
	defer r.mu.Unlock()

	if stored, ok := r.invites[inv.ID]; ok {
		inv = &stored
	}
	if inv.MaxUses > 0 && r.inviteUses[inv.ID] >= inv.MaxUses {
		return Member{}, errors.New("community: invite max uses exceeded")
	}
	if err := r.checkAndRecordJoinAttemptLocked(now, inv.Community, req.ApplicantPubB64, rate.MaxPerHour); err != nil {
		return Member{}, err
	}

	if _, ok := r.members[inv.Community]; !ok {
		r.members[inv.Community] = make(map[string]Member)
	}
	existing, ok := r.members[inv.Community][req.ApplicantPubB64]
	if ok && existing.Status == MemberBanned {
		return Member{}, errors.New("community: applicant banned")
	}
	r.inviteUses[inv.ID]++
	m := Member{
		Community:        inv.Community,
		MemberPubB64:     req.ApplicantPubB64,
		Role:             approval.GrantedRole,
		Status:           MemberActive,
		JoinedAt:         now.Unix(),
		ApprovedByPubB64: approval.IssuerPubB64,
		InviteID:         inv.ID,
	}
	r.members[inv.Community][req.ApplicantPubB64] = m
	return m, nil
}

func (r *Registry) ApplyRevocation(rev *Revocation) (Member, bool, error) {
	if r == nil {
		return Member{}, false, errors.New("community: registry is nil")
	}
	if rev == nil {
		return Member{}, false, errors.New("community: revocation is nil")
	}
	now := time.Now()
	if err := rev.Validate(now); err != nil {
		return Member{}, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	mm := r.members[rev.Community]
	if mm == nil {
		return Member{}, false, nil
	}
	m, ok := mm[rev.SubjectPubB64]
	if !ok {
		return Member{}, false, nil
	}
	switch rev.Kind {
	case RevokeMember:
		m.Status = MemberRevoked
	case BanMember:
		m.Status = MemberBanned
	default:
		return Member{}, false, fmt.Errorf("community: invalid revocation kind: %q", rev.Kind)
	}
	mm[rev.SubjectPubB64] = m
	return m, true, nil
}

func (r *Registry) checkAndRecordJoinAttemptLocked(now time.Time, community CommunityID, applicantPubB64 string, maxPerHour int) error {
	if maxPerHour <= 0 {
		maxPerHour = 5
	}
	if r.joinAttempts[community] == nil {
		r.joinAttempts[community] = make(map[string][]int64)
	}
	hits := r.joinAttempts[community][applicantPubB64]
	cut := now.Add(-1 * time.Hour).Unix()
	kept := hits[:0]
	for i := range hits {
		if hits[i] >= cut {
			kept = append(kept, hits[i])
		}
	}
	if len(kept) >= maxPerHour {
		r.joinAttempts[community][applicantPubB64] = kept
		return errors.New("community: join rate limit exceeded")
	}
	kept = append(kept, now.Unix())
	r.joinAttempts[community][applicantPubB64] = kept
	return nil
}

func newMemberKeypairB64() (string, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, err
	}
	return base64.RawURLEncoding.EncodeToString(pub), priv, nil
}
