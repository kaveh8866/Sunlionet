package ledgersync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/mesh"
	"github.com/kaveh/sunlionet-agent/pkg/messaging"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
)

const WireSchemaV1 = 1

type PeerRole string

const (
	PeerRoleNormal PeerRole = "normal"
	PeerRoleRelay  PeerRole = "relay"
	PeerRoleBridge PeerRole = "bridge"
	PeerRoleAgent  PeerRole = "agent"
)

type Peer struct {
	ID        string
	MeshPub   [32]byte
	PreKeyPub *[32]byte
	Role      PeerRole
}

type Options struct {
	MaxPeersPerRound int

	MaxHave int
	MaxWant int

	MaxEvents             int
	MaxEventBytes         int
	MaxEventsMessageBytes int

	MaxInboundMsgsPerMin  int
	MaxInboundBytesPerMin int

	PenaltyBase time.Duration
	PenaltyMax  time.Duration

	AllowContext      func(ctx string) bool
	AllowOutgoingKind func(ctx string, kind string) bool
	AllowIncomingKind func(ctx string, kind string) bool

	SecurityPolicy SecurityPolicy
}

func (o Options) normalize() Options {
	out := o
	if out.MaxPeersPerRound <= 0 {
		out.MaxPeersPerRound = 3
	}
	if out.MaxPeersPerRound > 12 {
		out.MaxPeersPerRound = 12
	}
	if out.MaxHave < 0 {
		out.MaxHave = 256
	}
	if out.MaxHave > 512 {
		out.MaxHave = 512
	}
	if out.MaxWant <= 0 {
		out.MaxWant = 128
	}
	if out.MaxWant > 512 {
		out.MaxWant = 512
	}
	if out.MaxEvents <= 0 {
		out.MaxEvents = 128
	}
	if out.MaxEvents > 512 {
		out.MaxEvents = 512
	}
	if out.MaxEventBytes <= 0 {
		out.MaxEventBytes = 256 * 1024
	}
	if out.MaxEventBytes > 1024*1024 {
		out.MaxEventBytes = 1024 * 1024
	}
	if out.MaxEventsMessageBytes <= 0 {
		out.MaxEventsMessageBytes = 1024 * 1024
	}
	if out.MaxEventsMessageBytes > 4*1024*1024 {
		out.MaxEventsMessageBytes = 4 * 1024 * 1024
	}
	if out.MaxInboundMsgsPerMin <= 0 {
		out.MaxInboundMsgsPerMin = 60
	}
	if out.MaxInboundMsgsPerMin > 600 {
		out.MaxInboundMsgsPerMin = 600
	}
	if out.MaxInboundBytesPerMin <= 0 {
		out.MaxInboundBytesPerMin = 2 * 1024 * 1024
	}
	if out.MaxInboundBytesPerMin > 50*1024*1024 {
		out.MaxInboundBytesPerMin = 50 * 1024 * 1024
	}
	if out.PenaltyBase <= 0 {
		out.PenaltyBase = 30 * time.Second
	}
	if out.PenaltyMax <= 0 {
		out.PenaltyMax = 10 * time.Minute
	}
	if out.PenaltyMax < out.PenaltyBase {
		out.PenaltyMax = out.PenaltyBase
	}
	if out.AllowContext == nil {
		out.AllowContext = func(string) bool { return true }
	}
	if out.AllowOutgoingKind == nil {
		out.AllowOutgoingKind = func(string, string) bool { return true }
	}
	if out.AllowIncomingKind == nil {
		out.AllowIncomingKind = func(string, string) bool { return true }
	}
	out.SecurityPolicy = out.SecurityPolicy.normalize()
	return out
}

type WireMessage struct {
	SchemaVersion int             `json:"v"`
	Type          string          `json:"t"`
	Session       string          `json:"s,omitempty"`
	Context       string          `json:"c,omitempty"`
	Body          json.RawMessage `json:"b,omitempty"`
}

func encodeWire(t string, session string, ctx string, body any) ([]byte, error) {
	var raw json.RawMessage
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	w := WireMessage{
		SchemaVersion: WireSchemaV1,
		Type:          t,
		Session:       session,
		Context:       ctx,
		Body:          raw,
	}
	return json.Marshal(w)
}

func decodeWire(raw []byte) (WireMessage, error) {
	var w WireMessage
	if err := json.Unmarshal(raw, &w); err != nil {
		return WireMessage{}, err
	}
	if w.SchemaVersion != WireSchemaV1 {
		return WireMessage{}, errors.New("ledgersync: unsupported wire schema")
	}
	if w.Type == "" {
		return WireMessage{}, errors.New("ledgersync: wire type missing")
	}
	return w, nil
}

func newSessionID(r ioRand) (string, error) {
	var b [12]byte
	if err := r(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

type ioRand func([]byte) error

func cryptoRand(b []byte) error {
	_, err := rand.Read(b)
	return err
}

func peerIDFromMeshPub(pub [32]byte) string {
	return base64.RawURLEncoding.EncodeToString(pub[:])
}

func msgIDFromPlaintext(pt []byte) string {
	sum := sha256.Sum256(pt)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

type peerState struct {
	lastInWindow time.Time
	inMsgs       int
	inBytes      int

	penaltyUntil time.Time
	penalty      time.Duration

	recentMsg map[string]time.Time

	lastContact time.Time
	role        PeerRole
}

type Service struct {
	mesh     mesh.Mesh
	crypto   *mesh.Crypto
	ledger   *ledger.Ledger
	policy   *ledger.Policy
	observer *ledger.Observer
	opts     Options
	rand     ioRand
	security *SecurityLayer

	mu    sync.Mutex
	peers map[string]*peerState
}

func (s *Service) Ledger() *ledger.Ledger {
	if s == nil {
		return nil
	}
	return s.ledger
}

func New(m mesh.Mesh, c *mesh.Crypto, l *ledger.Ledger, pol *ledger.Policy, obs *ledger.Observer, opts Options) (*Service, error) {
	if m == nil {
		return nil, errors.New("ledgersync: mesh is nil")
	}
	if c == nil {
		return nil, errors.New("ledgersync: crypto is nil")
	}
	if l == nil {
		return nil, errors.New("ledgersync: ledger is nil")
	}
	norm := opts.normalize()
	s := &Service{
		mesh:     m,
		crypto:   c,
		ledger:   l,
		policy:   pol,
		observer: obs,
		opts:     norm,
		rand:     cryptoRand,
		security: NewSecurityLayer(norm.SecurityPolicy),
		peers:    map[string]*peerState{},
	}
	return s, nil
}

func (s *Service) TouchPeer(id string, now time.Time) *peerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.peers[id]
	if !ok {
		ps = &peerState{recentMsg: map[string]time.Time{}}
		s.peers[id] = ps
	}
	ps.lastContact = now
	return ps
}

func (s *Service) setPeerRole(id string, role PeerRole, now time.Time) {
	if s == nil || strings.TrimSpace(id) == "" {
		return
	}
	s.mu.Lock()
	ps, ok := s.peers[id]
	if !ok {
		ps = &peerState{recentMsg: map[string]time.Time{}}
		s.peers[id] = ps
	}
	if role != "" {
		ps.role = role
	}
	ps.lastContact = now
	s.mu.Unlock()
	if s.security != nil && role != "" {
		s.security.SetPeerRole(id, role, now)
	}
}

func (s *Service) peerRole(id string) PeerRole {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.peers[id]
	if ps == nil || ps.role == "" {
		return PeerRoleNormal
	}
	return ps.role
}

func (s *Service) allowInbound(peerID string, size int, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.peers[peerID]
	if !ok {
		ps = &peerState{recentMsg: map[string]time.Time{}}
		s.peers[peerID] = ps
	}
	if !ps.penaltyUntil.IsZero() && now.Before(ps.penaltyUntil) {
		return false
	}

	windowStart := now.Truncate(time.Minute)
	if ps.lastInWindow.IsZero() || !ps.lastInWindow.Equal(windowStart) {
		ps.lastInWindow = windowStart
		ps.inMsgs = 0
		ps.inBytes = 0
	}
	ps.inMsgs++
	ps.inBytes += size
	if ps.inMsgs > s.opts.MaxInboundMsgsPerMin || ps.inBytes > s.opts.MaxInboundBytesPerMin {
		if ps.penalty < s.opts.PenaltyBase {
			ps.penalty = s.opts.PenaltyBase
		} else {
			ps.penalty *= 2
			if ps.penalty > s.opts.PenaltyMax {
				ps.penalty = s.opts.PenaltyMax
			}
		}
		ps.penaltyUntil = now.Add(ps.penalty)
		return false
	}
	return true
}

func (s *Service) seenMsg(peerID string, msgID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.peers[peerID]
	if ps == nil {
		ps = &peerState{recentMsg: map[string]time.Time{}}
		s.peers[peerID] = ps
	}
	for k, exp := range ps.recentMsg {
		if now.After(exp) {
			delete(ps.recentMsg, k)
		}
	}
	if _, ok := ps.recentMsg[msgID]; ok {
		return true
	}
	ps.recentMsg[msgID] = now.Add(2 * time.Minute)
	if len(ps.recentMsg) > 512 {
		type kv struct {
			k string
			t time.Time
		}
		all := make([]kv, 0, len(ps.recentMsg))
		for k, t := range ps.recentMsg {
			all = append(all, kv{k: k, t: t})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].t.Before(all[j].t) })
		for i := 0; i < len(all)/2; i++ {
			delete(ps.recentMsg, all[i].k)
		}
	}
	return false
}

func (s *Service) sendMesh(peer Peer, plaintext []byte) error {
	if s == nil {
		return errors.New("ledgersync: service is nil")
	}
	msg, err := s.crypto.EncryptPayload(plaintext, peer.MeshPub)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.mesh.Broadcast(raw)
}

func (s *Service) SendHeads(ctx context.Context, peer Peer, syncContext string) (string, error) {
	_ = ctx
	if s == nil {
		return "", errors.New("ledgersync: service is nil")
	}
	if !s.opts.AllowContext(syncContext) {
		return "", errors.New("ledgersync: context not allowed")
	}
	sid, err := newSessionID(s.rand)
	if err != nil {
		return "", err
	}
	h := s.ledger.BuildHeadsMessage()
	pt, err := encodeWire("heads", sid, syncContext, h)
	if err != nil {
		return "", err
	}
	if peer.ID == "" {
		peer.ID = peerIDFromMeshPub(peer.MeshPub)
	}
	s.setPeerRole(peer.ID, peer.Role, time.Now())
	return sid, s.sendMesh(peer, pt)
}

func (s *Service) ReceiveOnce(ctx context.Context) error {
	if s == nil {
		return errors.New("ledgersync: service is nil")
	}
	raw, err := s.mesh.Receive(ctx)
	if err != nil {
		return err
	}
	var mm mesh.MeshMessage
	if err := json.Unmarshal(raw, &mm); err != nil {
		return nil
	}
	pt, err := s.crypto.DecryptPayload(mm)
	if err != nil {
		return nil
	}

	peerID := peerIDFromMeshPub(mm.SenderPub)
	now := time.Now()
	role := s.peerRole(peerID)
	if !s.allowInbound(peerID, len(pt), now) {
		return nil
	}
	mid := msgIDFromPlaintext(pt)
	if s.seenMsg(peerID, mid, now) {
		return nil
	}
	s.TouchPeer(peerID, now)

	w, err := decodeWire(pt)
	if err != nil {
		return nil
	}
	if !s.opts.AllowContext(w.Context) {
		return nil
	}
	if s.security != nil {
		switch s.security.ObserveInbound(peerID, role, w.Type, len(pt), now) {
		case DecisionReject:
			return nil
		case DecisionDefer:
			return nil
		default:
		}
	}

	from := Peer{ID: peerID, MeshPub: mm.SenderPub, Role: role}
	switch w.Type {
	case "heads":
		var hm ledger.HeadsMessage
		if err := json.Unmarshal(w.Body, &hm); err != nil {
			return nil
		}
		_ = hm
		inv := s.ledger.BuildInventoryMessage(s.opts.MaxHave)
		reply, err := encodeWire("inv", w.Session, w.Context, inv)
		if err != nil {
			return nil
		}
		return s.sendMesh(from, reply)
	case "inv":
		var inv ledger.InventoryMessage
		if err := json.Unmarshal(w.Body, &inv); err != nil {
			return nil
		}
		if s.security != nil {
			switch s.security.ObserveInventory(peerID, role, inv, now) {
			case DecisionReject:
				return nil
			case DecisionDefer:
				return nil
			default:
			}
		}
		plan := s.ledger.PlanSyncFromPeer(inv, s.opts.MaxWant)
		want := plan.WantMessage()
		reply, err := encodeWire("want", w.Session, w.Context, want)
		if err != nil {
			return nil
		}
		return s.sendMesh(from, reply)
	case "want":
		var want ledger.WantMessage
		if err := json.Unmarshal(w.Body, &want); err != nil {
			return nil
		}
		wantIDs := s.filterOutgoingIDs(want.Want, w.Context)
		msg := s.ledger.BuildEventsMessageBounded(wantIDs, s.opts.MaxEvents, s.opts.MaxEventBytes, s.opts.MaxEventsMessageBytes)
		reply, err := encodeWire("events", w.Session, w.Context, msg)
		if err != nil {
			return nil
		}
		return s.sendMesh(from, reply)
	case "events":
		var em ledger.EventsMessage
		if err := json.Unmarshal(w.Body, &em); err != nil {
			return nil
		}
		em, dropped := s.filterIncomingEvents(em, w.Context)
		rep, _ := s.ledger.ApplyEventsMessageBounded(em, w.Context, s.policy, s.observer, s.opts.MaxEvents, s.opts.MaxEventBytes, s.opts.MaxEventsMessageBytes)
		rep.Rejected += dropped
		if s.security != nil {
			s.security.ObserveApplyReport(peerID, role, rep, now)
		}
		return nil
	default:
		return nil
	}
}

func (s *Service) TryApplyWirePlaintext(peerID string, plaintext []byte) (bool, ledger.ApplyReport, error) {
	if s == nil {
		return false, ledger.ApplyReport{}, errors.New("ledgersync: service is nil")
	}
	if s.ledger == nil {
		return false, ledger.ApplyReport{}, errors.New("ledgersync: ledger is nil")
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		peerID = "unknown"
	}
	now := time.Now()
	role := PeerRoleNormal
	if strings.HasPrefix(peerID, "relay:") {
		role = PeerRoleRelay
	}
	s.setPeerRole(peerID, role, now)
	if !s.allowInbound(peerID, len(plaintext), now) {
		return true, ledger.ApplyReport{}, nil
	}
	mid := msgIDFromPlaintext(plaintext)
	if s.seenMsg(peerID, mid, now) {
		return true, ledger.ApplyReport{}, nil
	}
	s.TouchPeer(peerID, now)

	w, err := decodeWire(plaintext)
	if err != nil {
		return false, ledger.ApplyReport{}, nil
	}
	if w.Type != "events" {
		return false, ledger.ApplyReport{}, nil
	}
	if !s.opts.AllowContext(w.Context) {
		return true, ledger.ApplyReport{}, nil
	}
	if s.security != nil {
		switch s.security.ObserveInbound(peerID, role, w.Type, len(plaintext), now) {
		case DecisionReject:
			return true, ledger.ApplyReport{}, nil
		case DecisionDefer:
			return true, ledger.ApplyReport{}, nil
		default:
		}
	}

	var em ledger.EventsMessage
	if err := json.Unmarshal(w.Body, &em); err != nil {
		return true, ledger.ApplyReport{}, nil
	}
	em, dropped := s.filterIncomingEvents(em, w.Context)
	rep, err := s.ledger.ApplyEventsMessageBounded(em, w.Context, s.policy, s.observer, s.opts.MaxEvents, s.opts.MaxEventBytes, s.opts.MaxEventsMessageBytes)
	rep.Rejected += dropped
	if s.security != nil {
		s.security.ObserveApplyReport(peerID, role, rep, now)
	}
	return true, rep, err
}

func (s *Service) filterOutgoingIDs(ids []string, ctx string) []string {
	if s == nil || s.ledger == nil || len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for i := range ids {
		id := ids[i]
		ev, ok := s.ledger.Get(id)
		if !ok {
			continue
		}
		if !s.opts.AllowOutgoingKind(ctx, ev.Kind) {
			continue
		}
		out = append(out, id)
	}
	return out
}

func (s *Service) filterIncomingEvents(msg ledger.EventsMessage, ctx string) (ledger.EventsMessage, int) {
	if s == nil || len(msg.Events) == 0 {
		return msg, 0
	}
	out := make([]ledger.Event, 0, len(msg.Events))
	dropped := 0
	for i := range msg.Events {
		ev := msg.Events[i]
		if !s.opts.AllowIncomingKind(ctx, ev.Kind) {
			dropped++
			continue
		}
		out = append(out, ev)
	}
	msg.Events = out
	return msg, dropped
}

func (s *Service) SelectPeers(candidates []Peer, now time.Time) []Peer {
	if s == nil {
		return nil
	}
	opts := s.opts
	type scored struct {
		p     Peer
		score int64
	}
	all := make([]scored, 0, len(candidates))
	relayLimit := 1
	agentLimit := 1
	if s.security != nil {
		p := s.security.Policy()
		relayLimit = p.MaxRelaysPerRound
		agentLimit = p.MaxAgentsPerRound
	}
	s.mu.Lock()
	for i := range candidates {
		p := candidates[i]
		if p.ID == "" {
			p.ID = peerIDFromMeshPub(p.MeshPub)
		}
		if p.Role == "" {
			p.Role = PeerRoleNormal
		}
		ps := s.peers[p.ID]
		if ps != nil && !ps.penaltyUntil.IsZero() && now.Before(ps.penaltyUntil) {
			continue
		}
		if ps != nil && ps.role != "" && p.Role == PeerRoleNormal {
			p.Role = ps.role
		}
		age := int64(0)
		if ps != nil && !ps.lastContact.IsZero() {
			age = int64(now.Sub(ps.lastContact) / time.Second)
		} else {
			age = 1 << 62
		}
		if s.security != nil {
			score := s.security.PeerPriority(p.ID, p.Role, age, now)
			if score < 0 {
				continue
			}
			all = append(all, scored{p: p, score: score})
		} else {
			all = append(all, scored{p: p, score: age})
		}
	}
	s.mu.Unlock()
	sort.Slice(all, func(i, j int) bool {
		if all[i].score == all[j].score {
			return all[i].p.ID < all[j].p.ID
		}
		return all[i].score > all[j].score
	})

	out := make([]Peer, 0, min(opts.MaxPeersPerRound, len(all)))
	relays := 0
	agents := 0
	for i := range all {
		switch all[i].p.Role {
		case PeerRoleRelay:
			if relays >= relayLimit {
				continue
			}
			relays++
		case PeerRoleAgent:
			if agents >= agentLimit {
				continue
			}
			agents++
		}
		s.setPeerRole(all[i].p.ID, all[i].p.Role, now)
		out = append(out, all[i].p)
		if len(out) >= opts.MaxPeersPerRound {
			break
		}
	}
	return out
}

func (s *Service) SyncRound(ctx context.Context, candidates []Peer, syncContext string) error {
	if s == nil {
		return errors.New("ledgersync: service is nil")
	}
	now := time.Now()
	peers := s.SelectPeers(candidates, now)
	for i := range peers {
		_, _ = s.SendHeads(ctx, peers[i], syncContext)
	}
	return nil
}

func EncodeForRelay(plaintext []byte, recipientPreKeyPub [32]byte) (string, error) {
	env, _, err := messaging.EncryptToPreKey(plaintext, recipientPreKeyPub)
	if err != nil {
		return "", err
	}
	return env.Encode()
}

func DecodeFromRelay(envelope string, recipientPreKeyPriv [32]byte) ([]byte, [32]byte, error) {
	env, err := messaging.DecodeEnvelope(envelope)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return messaging.DecryptWithPreKey(env, recipientPreKeyPriv)
}

func (s *Service) PublishEventsToRelay(ctx context.Context, r relay.Relay, mailbox relay.MailboxID, recipientPreKeyPub [32]byte, syncContext string, want []string, ttlSec int, powBits int) (int, error) {
	if s == nil {
		return 0, errors.New("ledgersync: service is nil")
	}
	if r == nil {
		return 0, errors.New("ledgersync: relay is nil")
	}
	if !s.opts.AllowContext(syncContext) {
		return 0, errors.New("ledgersync: context not allowed")
	}
	if len(want) == 0 {
		return 0, nil
	}
	sid, err := newSessionID(s.rand)
	if err != nil {
		return 0, err
	}

	maxBatches := 8
	relayPlainMax := 120 * 1024
	ledgerMsgMax := s.opts.MaxEventsMessageBytes
	if ledgerMsgMax > relayPlainMax {
		ledgerMsgMax = relayPlainMax
	}
	if ledgerMsgMax <= 0 {
		ledgerMsgMax = relayPlainMax
	}

	pending := append([]string(nil), want...)
	sentEvents := 0

	for batches := 0; batches < maxBatches && len(pending) > 0; batches++ {
		pending = s.filterOutgoingIDs(pending, syncContext)
		if len(pending) == 0 {
			break
		}
		msg := s.ledger.BuildEventsMessageBounded(pending, s.opts.MaxEvents, s.opts.MaxEventBytes, ledgerMsgMax)
		if len(msg.Events) == 0 {
			break
		}
		pt, err := encodeWire("events", sid, syncContext, msg)
		if err != nil {
			return sentEvents, err
		}
		if len(pt) > relayPlainMax {
			return sentEvents, errors.New("ledgersync: relay payload too large")
		}
		env, err := EncodeForRelay(pt, recipientPreKeyPub)
		if err != nil {
			return sentEvents, err
		}

		req := relay.PushRequest{
			Mailbox:        mailbox,
			Envelope:       relay.Envelope(env),
			TTLSec:         ttlSec,
			PoWBits:        powBits,
			PoWNonceB64URL: "",
		}
		if _, err := r.Push(ctx, req); err != nil {
			return sentEvents, err
		}

		sentSet := make(map[string]struct{}, len(msg.Events))
		for i := range msg.Events {
			sentSet[msg.Events[i].ID] = struct{}{}
		}
		kept := pending[:0]
		for i := range pending {
			if _, ok := sentSet[pending[i]]; ok {
				continue
			}
			kept = append(kept, pending[i])
		}
		pending = kept
		sentEvents += len(msg.Events)
	}

	return sentEvents, nil
}

func (s *Service) PullAndApplyFromRelay(ctx context.Context, r relay.Relay, mailbox relay.MailboxID, recipientPreKeyPriv [32]byte, limit int, ack bool) (ledger.ApplyReport, int, error) {
	if s == nil {
		return ledger.ApplyReport{}, 0, errors.New("ledgersync: service is nil")
	}
	if r == nil {
		return ledger.ApplyReport{}, 0, errors.New("ledgersync: relay is nil")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	msgs, err := r.Pull(ctx, relay.PullRequest{Mailbox: mailbox, Limit: limit})
	if err != nil {
		return ledger.ApplyReport{}, 0, err
	}

	now := time.Now()
	peerID := "relay:" + string(mailbox)
	role := PeerRoleRelay
	s.setPeerRole(peerID, role, now)
	relayPlainMax := 120 * 1024
	ledgerMsgMax := s.opts.MaxEventsMessageBytes
	if ledgerMsgMax > relayPlainMax {
		ledgerMsgMax = relayPlainMax
	}
	if ledgerMsgMax <= 0 {
		ledgerMsgMax = relayPlainMax
	}

	var rep ledger.ApplyReport
	ackIDs := make([]relay.MessageID, 0, len(msgs))

	for i := range msgs {
		envStr := string(msgs[i].Envelope)
		pt, _, err := DecodeFromRelay(envStr, recipientPreKeyPriv)
		if err != nil {
			continue
		}
		if !s.allowInbound(peerID, len(pt), now) {
			continue
		}
		mid := msgIDFromPlaintext(pt)
		if s.seenMsg(peerID, mid, now) {
			if ack {
				ackIDs = append(ackIDs, msgs[i].ID)
			}
			continue
		}

		w, err := decodeWire(pt)
		if err != nil {
			if ack {
				ackIDs = append(ackIDs, msgs[i].ID)
			}
			continue
		}
		if !s.opts.AllowContext(w.Context) {
			if ack {
				ackIDs = append(ackIDs, msgs[i].ID)
			}
			continue
		}
		if s.security != nil {
			switch s.security.ObserveInbound(peerID, role, w.Type, len(pt), now) {
			case DecisionReject:
				continue
			case DecisionDefer:
				continue
			default:
			}
		}

		switch w.Type {
		case "events":
			var em ledger.EventsMessage
			if err := json.Unmarshal(w.Body, &em); err != nil {
				if ack {
					ackIDs = append(ackIDs, msgs[i].ID)
				}
				continue
			}
			em, dropped := s.filterIncomingEvents(em, w.Context)
			rp, _ := s.ledger.ApplyEventsMessageBounded(em, w.Context, s.policy, s.observer, s.opts.MaxEvents, s.opts.MaxEventBytes, ledgerMsgMax)
			rep.Applied += rp.Applied
			rep.Dupe += rp.Dupe
			rep.Rejected += rp.Rejected + dropped
			if s.security != nil {
				rp.Rejected += dropped
				s.security.ObserveApplyReport(peerID, role, rp, now)
			}
		default:
		}

		if ack {
			ackIDs = append(ackIDs, msgs[i].ID)
		}
	}

	if ack && len(ackIDs) > 0 {
		_ = r.Ack(ctx, relay.AckRequest{Mailbox: mailbox, IDs: ackIDs})
	}
	return rep, len(msgs), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
