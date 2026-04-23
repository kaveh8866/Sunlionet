package ledgersync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

	VerifyWorkers   int
	VerifyBatchSize int

	VerifiedCacheMax int
	VerifiedCacheTTL time.Duration

	ApplyChunkSize int

	SchedulerCriticalWeight   int
	SchedulerNormalWeight     int
	SchedulerBackgroundWeight int

	SchedulerDrainPerReceive   int
	SchedulerDrainAfterRelay   int
	SchedulerMaxQueuedTotal    int
	SchedulerMaxQueuedPerPeer  int
	SchedulerMaxQueuedCritical int
	SchedulerMaxQueuedNormal   int
	SchedulerMaxQueuedBg       int

	MaxInboundMsgsPerMin  int
	MaxInboundBytesPerMin int

	PenaltyBase time.Duration
	PenaltyMax  time.Duration

	RecentMsgTTL time.Duration
	RecentMsgMax int

	AllowContext      func(ctx string) bool
	AllowOutgoingKind func(ctx string, kind string) bool
	AllowIncomingKind func(ctx string, kind string) bool

	SecurityPolicy SecurityPolicy
	Privacy        PrivacyPolicy
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
	if out.VerifyWorkers <= 0 {
		n := runtime.NumCPU()
		if n <= 0 {
			n = 1
		}
		if n > 4 {
			n = 4
		}
		out.VerifyWorkers = n
	}
	if out.VerifyWorkers > 16 {
		out.VerifyWorkers = 16
	}
	if out.VerifyBatchSize <= 0 {
		out.VerifyBatchSize = 64
	}
	if out.VerifyBatchSize > 512 {
		out.VerifyBatchSize = 512
	}
	if out.VerifiedCacheMax <= 0 {
		out.VerifiedCacheMax = 4096
	}
	if out.VerifiedCacheMax > 65536 {
		out.VerifiedCacheMax = 65536
	}
	if out.VerifiedCacheTTL <= 0 {
		out.VerifiedCacheTTL = 10 * time.Minute
	}
	if out.VerifiedCacheTTL > 2*time.Hour {
		out.VerifiedCacheTTL = 2 * time.Hour
	}
	if out.ApplyChunkSize <= 0 {
		out.ApplyChunkSize = 64
	}
	if out.ApplyChunkSize > 1024 {
		out.ApplyChunkSize = 1024
	}
	if out.SchedulerCriticalWeight <= 0 {
		out.SchedulerCriticalWeight = 5
	}
	if out.SchedulerNormalWeight <= 0 {
		out.SchedulerNormalWeight = 3
	}
	if out.SchedulerBackgroundWeight <= 0 {
		out.SchedulerBackgroundWeight = 1
	}
	if out.SchedulerCriticalWeight > 32 {
		out.SchedulerCriticalWeight = 32
	}
	if out.SchedulerNormalWeight > 32 {
		out.SchedulerNormalWeight = 32
	}
	if out.SchedulerBackgroundWeight > 32 {
		out.SchedulerBackgroundWeight = 32
	}
	if out.SchedulerDrainPerReceive <= 0 {
		out.SchedulerDrainPerReceive = 8
	}
	if out.SchedulerDrainPerReceive > 1024 {
		out.SchedulerDrainPerReceive = 1024
	}
	if out.SchedulerDrainAfterRelay <= 0 {
		out.SchedulerDrainAfterRelay = 256
	}
	if out.SchedulerDrainAfterRelay > 8192 {
		out.SchedulerDrainAfterRelay = 8192
	}
	if out.SchedulerMaxQueuedTotal <= 0 {
		out.SchedulerMaxQueuedTotal = 2048
	}
	if out.SchedulerMaxQueuedTotal > 200000 {
		out.SchedulerMaxQueuedTotal = 200000
	}
	if out.SchedulerMaxQueuedPerPeer <= 0 {
		out.SchedulerMaxQueuedPerPeer = 256
	}
	if out.SchedulerMaxQueuedPerPeer > 20000 {
		out.SchedulerMaxQueuedPerPeer = 20000
	}
	if out.SchedulerMaxQueuedCritical <= 0 {
		out.SchedulerMaxQueuedCritical = 64
	}
	if out.SchedulerMaxQueuedNormal <= 0 {
		out.SchedulerMaxQueuedNormal = 128
	}
	if out.SchedulerMaxQueuedBg <= 0 {
		out.SchedulerMaxQueuedBg = 64
	}
	if out.SchedulerMaxQueuedCritical > out.SchedulerMaxQueuedPerPeer {
		out.SchedulerMaxQueuedCritical = out.SchedulerMaxQueuedPerPeer
	}
	if out.SchedulerMaxQueuedNormal > out.SchedulerMaxQueuedPerPeer {
		out.SchedulerMaxQueuedNormal = out.SchedulerMaxQueuedPerPeer
	}
	if out.SchedulerMaxQueuedBg > out.SchedulerMaxQueuedPerPeer {
		out.SchedulerMaxQueuedBg = out.SchedulerMaxQueuedPerPeer
	}
	if out.SchedulerMaxQueuedPerPeer > out.SchedulerMaxQueuedTotal {
		out.SchedulerMaxQueuedPerPeer = out.SchedulerMaxQueuedTotal
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
	if out.RecentMsgTTL <= 0 {
		out.RecentMsgTTL = 2 * time.Minute
	}
	if out.RecentMsgTTL > 30*time.Minute {
		out.RecentMsgTTL = 30 * time.Minute
	}
	if out.RecentMsgMax <= 0 {
		out.RecentMsgMax = 512
	}
	if out.RecentMsgMax > 8192 {
		out.RecentMsgMax = 8192
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
	out.Privacy = out.Privacy.normalize()
	return out
}

type PrivacyLevel string

const (
	PrivacyLow      PrivacyLevel = "low"
	PrivacyMedium   PrivacyLevel = "medium"
	PrivacyHigh     PrivacyLevel = "high"
	PrivacyParanoid PrivacyLevel = "paranoid"
)

type PrivacyPolicy struct {
	Level PrivacyLevel

	OutboundDelayMax time.Duration

	PaddingBucketBytes int

	InventorySamplePermille int

	WantSamplePermille int
	WantNoiseCount     int

	CoverPeers int
}

func (p PrivacyPolicy) normalize() PrivacyPolicy {
	out := p
	if out.Level == "" {
		out.Level = PrivacyMedium
	}

	switch out.Level {
	case PrivacyLow:
		if out.PaddingBucketBytes == 0 {
			out.PaddingBucketBytes = 0
		}
		if out.InventorySamplePermille == 0 {
			out.InventorySamplePermille = 1000
		}
		if out.WantSamplePermille == 0 {
			out.WantSamplePermille = 1000
		}
	case PrivacyMedium:
		if out.PaddingBucketBytes == 0 {
			out.PaddingBucketBytes = 4096
		}
		if out.InventorySamplePermille == 0 {
			out.InventorySamplePermille = 1000
		}
		if out.WantSamplePermille == 0 {
			out.WantSamplePermille = 1000
		}
	case PrivacyHigh:
		if out.PaddingBucketBytes == 0 {
			out.PaddingBucketBytes = 8192
		}
		if out.OutboundDelayMax == 0 {
			out.OutboundDelayMax = 2 * time.Second
		}
		if out.InventorySamplePermille == 0 {
			out.InventorySamplePermille = 600
		}
		if out.WantSamplePermille == 0 {
			out.WantSamplePermille = 700
		}
		if out.WantNoiseCount == 0 {
			out.WantNoiseCount = 8
		}
		if out.CoverPeers == 0 {
			out.CoverPeers = 1
		}
	case PrivacyParanoid:
		if out.PaddingBucketBytes == 0 {
			out.PaddingBucketBytes = 16384
		}
		if out.OutboundDelayMax == 0 {
			out.OutboundDelayMax = 8 * time.Second
		}
		if out.InventorySamplePermille == 0 {
			out.InventorySamplePermille = 400
		}
		if out.WantSamplePermille == 0 {
			out.WantSamplePermille = 500
		}
		if out.WantNoiseCount == 0 {
			out.WantNoiseCount = 32
		}
		if out.CoverPeers == 0 {
			out.CoverPeers = 2
		}
	default:
		out.Level = PrivacyMedium
		if out.PaddingBucketBytes == 0 {
			out.PaddingBucketBytes = 4096
		}
		if out.InventorySamplePermille == 0 {
			out.InventorySamplePermille = 1000
		}
		if out.WantSamplePermille == 0 {
			out.WantSamplePermille = 1000
		}
	}

	if out.OutboundDelayMax < 0 {
		out.OutboundDelayMax = 0
	}
	if out.PaddingBucketBytes < 0 {
		out.PaddingBucketBytes = 0
	}
	if out.InventorySamplePermille <= 0 {
		out.InventorySamplePermille = 1000
	}
	if out.InventorySamplePermille > 1000 {
		out.InventorySamplePermille = 1000
	}
	if out.WantSamplePermille <= 0 {
		out.WantSamplePermille = 1000
	}
	if out.WantSamplePermille > 1000 {
		out.WantSamplePermille = 1000
	}
	if out.WantNoiseCount < 0 {
		out.WantNoiseCount = 0
	}
	if out.CoverPeers < 0 {
		out.CoverPeers = 0
	}
	if out.CoverPeers > 4 {
		out.CoverPeers = 4
	}
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

var privacyPadMagic = [4]byte{'S', 'L', 'P', '1'}

func padPlaintext(plaintext []byte, bucketBytes int, r ioRand) ([]byte, error) {
	if bucketBytes <= 0 || len(plaintext) == 0 {
		return plaintext, nil
	}
	header := 8
	minSize := len(plaintext) + header
	if minSize >= bucketBytes {
		return plaintext, nil
	}
	out := make([]byte, bucketBytes)
	copy(out[:4], privacyPadMagic[:])
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(plaintext)))
	copy(out[8:], plaintext)
	if err := r(out[minSize:]); err != nil {
		return nil, err
	}
	return out, nil
}

func unpadPlaintext(plaintext []byte) ([]byte, bool) {
	if len(plaintext) < 8 {
		return plaintext, false
	}
	var magic [4]byte
	copy(magic[:], plaintext[:4])
	if magic != privacyPadMagic {
		return plaintext, false
	}
	n := int(binary.LittleEndian.Uint32(plaintext[4:8]))
	if n < 0 || n > len(plaintext)-8 {
		return plaintext, false
	}
	return plaintext[8 : 8+n], true
}

func sampleStringsPermille(ids []string, permille int, r ioRand) []string {
	if len(ids) == 0 {
		return nil
	}
	if permille >= 1000 {
		return append([]string(nil), ids...)
	}
	if permille <= 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for i := range ids {
		rn, err := randUint32(r)
		if err != nil {
			out = append(out, ids[i])
			continue
		}
		if int(rn%1000) < permille {
			out = append(out, ids[i])
		}
	}
	return out
}

func shuffleStrings(in []string, r ioRand) []string {
	if len(in) <= 1 {
		return append([]string(nil), in...)
	}
	out := append([]string(nil), in...)
	for i := len(out) - 1; i > 0; i-- {
		rn, err := randUint32(r)
		if err != nil {
			continue
		}
		j := int(rn % uint32(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func shufflePeers(in []Peer, r ioRand) []Peer {
	if len(in) <= 1 {
		return append([]Peer(nil), in...)
	}
	out := append([]Peer(nil), in...)
	for i := len(out) - 1; i > 0; i-- {
		rn, err := randUint32(r)
		if err != nil {
			continue
		}
		j := int(rn % uint32(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func randDuration(r ioRand, max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	rn, err := randUint32(r)
	if err != nil {
		return 0
	}
	return time.Duration(rn) % (max + 1)
}

func peerIDFromMeshPub(pub [32]byte) string {
	return base64.RawURLEncoding.EncodeToString(pub[:])
}

func msgIDFromPlaintext(pt []byte) string {
	sum := sha256.Sum256(pt)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func randUint32(r ioRand) (uint32, error) {
	var b [4]byte
	if err := r(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

type peerState struct {
	lastInWindow time.Time
	inMsgs       int
	inBytes      int

	meshPub    [32]byte
	meshPubSet bool

	penaltyUntil time.Time
	penalty      time.Duration

	recentMsg map[string]time.Time

	lastContact time.Time
	role        PeerRole
}

type Service struct {
	mesh        mesh.Mesh
	crypto      *mesh.Crypto
	ledger      *ledger.Ledger
	policy      *ledger.Policy
	observer    *ledger.Observer
	opts        Options
	rand        ioRand
	security    *SecurityLayer
	verifyCache *ledger.VerifiedEventCache
	scheduler   *PriorityScheduler

	mu    sync.Mutex
	peers map[string]*peerState
	stats syncStats
}

type PriorityClass uint8

const (
	PriorityCritical PriorityClass = iota
	PriorityNormal
	PriorityBackground
)

type schedItem struct {
	peerID          string
	role            PeerRole
	context         string
	msg             ledger.EventsMessage
	maxMessageBytes int
}

type fifoQueue struct {
	items []schedItem
	head  int
}

func (q *fifoQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.items) - q.head
}

func (q *fifoQueue) Push(it schedItem) {
	q.items = append(q.items, it)
}

func (q *fifoQueue) Pop() (schedItem, bool) {
	if q.Len() == 0 {
		return schedItem{}, false
	}
	it := q.items[q.head]
	q.head++
	if q.head > 64 && q.head*2 >= len(q.items) {
		q.items = append([]schedItem(nil), q.items[q.head:]...)
		q.head = 0
	}
	return it, true
}

type peerQuota struct {
	total int
	c     int
	n     int
	b     int
}

type PriorityScheduler struct {
	mu sync.Mutex

	maxTotal   int
	maxPerPeer int
	maxC       int
	maxN       int
	maxB       int

	qC fifoQueue
	qN fifoQueue
	qB fifoQueue

	peers map[string]*peerQuota

	pattern []PriorityClass
	patIdx  int
}

func NewPriorityScheduler(opts Options) *PriorityScheduler {
	p := make([]PriorityClass, 0, opts.SchedulerCriticalWeight+opts.SchedulerNormalWeight+opts.SchedulerBackgroundWeight)
	for i := 0; i < opts.SchedulerCriticalWeight; i++ {
		p = append(p, PriorityCritical)
	}
	for i := 0; i < opts.SchedulerNormalWeight; i++ {
		p = append(p, PriorityNormal)
	}
	for i := 0; i < opts.SchedulerBackgroundWeight; i++ {
		p = append(p, PriorityBackground)
	}
	if len(p) == 0 {
		p = []PriorityClass{PriorityCritical, PriorityNormal, PriorityBackground}
	}
	return &PriorityScheduler{
		maxTotal:   opts.SchedulerMaxQueuedTotal,
		maxPerPeer: opts.SchedulerMaxQueuedPerPeer,
		maxC:       opts.SchedulerMaxQueuedCritical,
		maxN:       opts.SchedulerMaxQueuedNormal,
		maxB:       opts.SchedulerMaxQueuedBg,
		peers:      map[string]*peerQuota{},
		pattern:    p,
	}
}

func (s *PriorityScheduler) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.qC.Len() + s.qN.Len() + s.qB.Len()
}

func (s *PriorityScheduler) Enqueue(peerID string, cls PriorityClass, it schedItem) bool {
	if s == nil {
		return false
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		peerID = "unknown"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ps := s.peers[peerID]
	if ps == nil {
		ps = &peerQuota{}
		s.peers[peerID] = ps
	}

	for {
		switch cls {
		case PriorityCritical:
			if ps.c >= s.maxC || ps.total >= s.maxPerPeer {
				cls = PriorityNormal
				continue
			}
		case PriorityNormal:
			if ps.n >= s.maxN || ps.total >= s.maxPerPeer {
				cls = PriorityBackground
				continue
			}
		default:
			if ps.b >= s.maxB || ps.total >= s.maxPerPeer {
				return false
			}
		}
		break
	}

	total := s.qC.Len() + s.qN.Len() + s.qB.Len()
	if s.maxTotal > 0 && total >= s.maxTotal {
		if cls != PriorityCritical {
			return false
		}
		for total >= s.maxTotal {
			if _, ok := s.qB.Pop(); ok {
				total--
				continue
			}
			if _, ok := s.qN.Pop(); ok {
				total--
				continue
			}
			return false
		}
	}

	it.peerID = peerID
	switch cls {
	case PriorityCritical:
		s.qC.Push(it)
		ps.c++
	case PriorityNormal:
		s.qN.Push(it)
		ps.n++
	default:
		s.qB.Push(it)
		ps.b++
	}
	ps.total++
	return true
}

func (s *PriorityScheduler) Next() (schedItem, bool) {
	if s == nil {
		return schedItem{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qC.Len()+s.qN.Len()+s.qB.Len() == 0 {
		return schedItem{}, false
	}

	tries := len(s.pattern)
	if tries <= 0 {
		tries = 1
	}
	for i := 0; i < tries; i++ {
		cls := s.pattern[s.patIdx]
		s.patIdx++
		if s.patIdx >= len(s.pattern) {
			s.patIdx = 0
		}
		var it schedItem
		var ok bool
		switch cls {
		case PriorityCritical:
			it, ok = s.qC.Pop()
		case PriorityNormal:
			it, ok = s.qN.Pop()
		default:
			it, ok = s.qB.Pop()
		}
		if !ok {
			continue
		}

		ps := s.peers[it.peerID]
		if ps != nil {
			ps.total--
			switch cls {
			case PriorityCritical:
				ps.c--
			case PriorityNormal:
				ps.n--
			default:
				ps.b--
			}
			if ps.total <= 0 {
				delete(s.peers, it.peerID)
			}
		}
		return it, true
	}

	return schedItem{}, false
}

func (s *Service) Ledger() *ledger.Ledger {
	if s == nil {
		return nil
	}
	return s.ledger
}

type SyncStats struct {
	Deferred    uint64
	Rejected    uint64
	Duplicate   uint64
	Quarantined uint64
}

type syncStats struct {
	deferred    atomic.Uint64
	rejected    atomic.Uint64
	duplicate   atomic.Uint64
	quarantined atomic.Uint64
}

func (s *Service) Stats() SyncStats {
	if s == nil {
		return SyncStats{}
	}
	return SyncStats{
		Deferred:    s.stats.deferred.Load(),
		Rejected:    s.stats.rejected.Load(),
		Duplicate:   s.stats.duplicate.Load(),
		Quarantined: s.stats.quarantined.Load(),
	}
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
		mesh:        m,
		crypto:      c,
		ledger:      l,
		policy:      pol,
		observer:    obs,
		opts:        norm,
		rand:        cryptoRand,
		security:    NewSecurityLayer(norm.SecurityPolicy),
		verifyCache: ledger.NewVerifiedEventCache(norm.VerifiedCacheMax, norm.VerifiedCacheTTL),
		scheduler:   NewPriorityScheduler(norm),
		peers:       map[string]*peerState{},
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

func (s *Service) notePeerMeshPub(id string, pub [32]byte, now time.Time) {
	if s == nil || strings.TrimSpace(id) == "" {
		return
	}
	s.mu.Lock()
	ps, ok := s.peers[id]
	if !ok {
		ps = &peerState{recentMsg: map[string]time.Time{}}
		s.peers[id] = ps
	}
	ps.meshPub = pub
	ps.meshPubSet = true
	ps.lastContact = now
	s.mu.Unlock()
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
	ps.recentMsg[msgID] = now.Add(s.opts.RecentMsgTTL)
	if len(ps.recentMsg) > s.opts.RecentMsgMax {
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

func (s *Service) alreadySeenMsg(peerID string, msgID string, now time.Time) bool {
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
	_, ok := ps.recentMsg[msgID]
	return ok
}

func (s *Service) sendMesh(peer Peer, plaintext []byte) error {
	if s == nil {
		return errors.New("ledgersync: service is nil")
	}
	if s.opts.Privacy.OutboundDelayMax > 0 {
		if d := randDuration(s.rand, s.opts.Privacy.OutboundDelayMax); d > 0 {
			time.Sleep(d)
		}
	}
	if s.opts.Privacy.PaddingBucketBytes > 0 {
		pt, err := padPlaintext(plaintext, s.opts.Privacy.PaddingBucketBytes, s.rand)
		if err == nil {
			plaintext = pt
		}
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
	s.notePeerMeshPub(peer.ID, peer.MeshPub, time.Now())
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
	s.notePeerMeshPub(peerID, mm.SenderPub, now)
	role := s.peerRole(peerID)
	if !s.allowInbound(peerID, len(pt), now) {
		s.stats.rejected.Add(1)
		return nil
	}
	upt, _ := unpadPlaintext(pt)
	mid := msgIDFromPlaintext(upt)
	if s.alreadySeenMsg(peerID, mid, now) {
		s.stats.duplicate.Add(1)
		return nil
	}
	markSeen := func() {
		_ = s.seenMsg(peerID, mid, now)
	}
	s.TouchPeer(peerID, now)

	w, err := decodeWire(upt)
	if err != nil {
		markSeen()
		return nil
	}
	if !s.opts.AllowContext(w.Context) {
		markSeen()
		return nil
	}
	if s.security != nil {
		switch s.security.ObserveInbound(peerID, role, w.Type, len(pt), now) {
		case DecisionReject:
			s.stats.rejected.Add(1)
			if s.security.TrustTier(peerID, now) == TrustBlocked {
				s.stats.quarantined.Add(1)
			}
			markSeen()
			return nil
		case DecisionDefer:
			s.stats.deferred.Add(1)
			return nil
		default:
		}
	}

	from := Peer{ID: peerID, MeshPub: mm.SenderPub, Role: role}
	switch w.Type {
	case "heads":
		var hm ledger.HeadsMessage
		if err := json.Unmarshal(w.Body, &hm); err != nil {
			markSeen()
			return nil
		}
		if hm.SchemaVersion != ledger.SyncSchemaV1 {
			markSeen()
			return nil
		}
		inv := s.ledger.BuildInventoryMessage(s.opts.MaxHave)
		if s.opts.Privacy.InventorySamplePermille < 1000 && len(inv.Have) >= 32 {
			inv.Have = sampleStringsPermille(inv.Have, s.opts.Privacy.InventorySamplePermille, s.rand)
		}
		reply, err := encodeWire("inv", w.Session, w.Context, inv)
		if err != nil {
			markSeen()
			return nil
		}
		if err := s.sendMesh(from, reply); err != nil {
			return err
		}
		markSeen()
		return nil
	case "inv":
		var inv ledger.InventoryMessage
		if err := json.Unmarshal(w.Body, &inv); err != nil {
			markSeen()
			return nil
		}
		if inv.SchemaVersion != ledger.SyncSchemaV1 {
			markSeen()
			return nil
		}
		if s.security != nil {
			switch s.security.ObserveInventory(peerID, role, inv, now) {
			case DecisionReject:
				s.stats.rejected.Add(1)
				if s.security.TrustTier(peerID, now) == TrustBlocked {
					s.stats.quarantined.Add(1)
				}
				markSeen()
				return nil
			case DecisionDefer:
				if s.observer != nil && s.ledger != nil {
					peers := s.security.InventoryClusterPeers(inv, now)
					thr := s.security.Policy().InventoryClusterThreshold
					if thr > 0 && len(peers) == thr {
						_, _ = s.ledger.EmitSybilMisbehavior(w.Context, peers, "inventory_cluster", s.policy, s.observer)
					}
				}
				s.stats.deferred.Add(1)
				return nil
			default:
			}
		}
		plan := s.ledger.PlanSyncFromPeer(inv, s.opts.MaxWant)
		wantIDs := plan.Want
		if s.opts.Privacy.WantSamplePermille < 1000 && len(wantIDs) >= 8 {
			wantIDs = sampleStringsPermille(wantIDs, s.opts.Privacy.WantSamplePermille, s.rand)
			if len(wantIDs) == 0 && len(plan.Want) > 0 {
				wantIDs = plan.Want[:1]
			}
		}
		if s.opts.Privacy.WantNoiseCount > 0 && len(inv.Have) >= 8 {
			noise := make([]string, 0, len(inv.Have))
			for i := range inv.Have {
				id := inv.Have[i]
				if id == "" {
					continue
				}
				if s.ledger.Have(id) {
					noise = append(noise, id)
				}
			}
			if len(noise) > 0 {
				noise = shuffleStrings(noise, s.rand)
				n := s.opts.Privacy.WantNoiseCount
				if n > len(noise) {
					n = len(noise)
				}
				wantIDs = append(wantIDs, noise[:n]...)
			}
		}
		if len(wantIDs) > 1 {
			seen := map[string]struct{}{}
			uniq := wantIDs[:0]
			for i := range wantIDs {
				id := wantIDs[i]
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				uniq = append(uniq, id)
			}
			wantIDs = uniq
		}
		if s.opts.MaxWant > 0 && len(wantIDs) > s.opts.MaxWant {
			wantIDs = wantIDs[:s.opts.MaxWant]
		}
		want := ledger.WantMessage{SchemaVersion: ledger.SyncSchemaV1, Want: wantIDs}
		reply, err := encodeWire("want", w.Session, w.Context, want)
		if err != nil {
			markSeen()
			return nil
		}
		if err := s.sendMesh(from, reply); err != nil {
			return err
		}
		markSeen()
		return nil
	case "want":
		var want ledger.WantMessage
		if err := json.Unmarshal(w.Body, &want); err != nil {
			markSeen()
			return nil
		}
		if want.SchemaVersion != ledger.SyncSchemaV1 {
			markSeen()
			return nil
		}
		if s.security != nil {
			switch s.security.ObserveWant(peerID, role, want, now) {
			case DecisionReject:
				s.stats.rejected.Add(1)
				if s.security.TrustTier(peerID, now) == TrustBlocked {
					s.stats.quarantined.Add(1)
				}
				markSeen()
				return nil
			case DecisionDefer:
				s.stats.deferred.Add(1)
				return nil
			default:
			}
		}
		if s.opts.MaxWant > 0 && len(want.Want) > s.opts.MaxWant {
			want.Want = want.Want[:s.opts.MaxWant]
		} else if s.opts.MaxWant <= 0 && len(want.Want) > 256 {
			want.Want = want.Want[:256]
		}
		wantIDs := s.filterOutgoingIDs(want.Want, w.Context)
		msg := s.ledger.BuildEventsMessageBounded(wantIDs, s.opts.MaxEvents, s.opts.MaxEventBytes, s.opts.MaxEventsMessageBytes)
		reply, err := encodeWire("events", w.Session, w.Context, msg)
		if err != nil {
			markSeen()
			return nil
		}
		if err := s.sendMesh(from, reply); err != nil {
			return err
		}
		markSeen()
		return nil
	case "events":
		var em ledger.EventsMessage
		if err := json.Unmarshal(w.Body, &em); err != nil {
			markSeen()
			return nil
		}
		if em.SchemaVersion != ledger.SyncSchemaV1 {
			markSeen()
			return nil
		}
		em, dropped := s.filterIncomingEvents(em, w.Context)
		if dropped > 0 {
			s.stats.rejected.Add(uint64(dropped))
		}
		cm, nm, bm := splitEventsByPriority(em)
		enq := 0
		if len(cm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityCritical, schedItem{role: role, context: w.Context, msg: cm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
			enq++
		}
		if len(nm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityNormal, schedItem{role: role, context: w.Context, msg: nm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
			enq++
		}
		if len(bm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityBackground, schedItem{role: role, context: w.Context, msg: bm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
			enq++
		}
		if enq == 0 {
			s.stats.deferred.Add(1)
			return nil
		}
		_ = s.drainInbound(s.opts.SchedulerDrainPerReceive)
		markSeen()
		return nil
	default:
		markSeen()
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
		s.stats.rejected.Add(1)
		return true, ledger.ApplyReport{}, nil
	}
	upt, _ := unpadPlaintext(plaintext)
	mid := msgIDFromPlaintext(upt)
	if s.seenMsg(peerID, mid, now) {
		s.stats.duplicate.Add(1)
		return true, ledger.ApplyReport{}, nil
	}
	s.TouchPeer(peerID, now)

	w, err := decodeWire(upt)
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
			s.stats.rejected.Add(1)
			if s.security.TrustTier(peerID, now) == TrustBlocked {
				s.stats.quarantined.Add(1)
			}
			return true, ledger.ApplyReport{}, nil
		case DecisionDefer:
			s.stats.deferred.Add(1)
			return true, ledger.ApplyReport{}, nil
		default:
		}
	}

	var em ledger.EventsMessage
	if err := json.Unmarshal(w.Body, &em); err != nil {
		return true, ledger.ApplyReport{}, nil
	}
	if em.SchemaVersion != ledger.SyncSchemaV1 {
		return true, ledger.ApplyReport{}, nil
	}
	em, dropped := s.filterIncomingEvents(em, w.Context)
	if dropped > 0 {
		s.stats.rejected.Add(uint64(dropped))
	}
	cm, nm, bm := splitEventsByPriority(em)
	enq := 0
	if len(cm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityCritical, schedItem{role: role, context: w.Context, msg: cm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
		enq++
	}
	if len(nm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityNormal, schedItem{role: role, context: w.Context, msg: nm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
		enq++
	}
	if len(bm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityBackground, schedItem{role: role, context: w.Context, msg: bm, maxMessageBytes: s.opts.MaxEventsMessageBytes}) {
		enq++
	}
	if enq == 0 {
		s.stats.deferred.Add(1)
		return true, ledger.ApplyReport{Rejected: dropped}, nil
	}
	rep := s.drainInbound(s.opts.SchedulerDrainPerReceive)
	rep.Rejected += dropped
	return true, rep, nil
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

func priorityForKind(kind string) PriorityClass {
	switch kind {
	case ledger.KindIdentityIntroduce,
		ledger.KindIdentityRotate,
		ledger.KindIdentityRevoke,
		ledger.KindMisbehaviorEquivoc,
		ledger.KindMisbehaviorReplay,
		ledger.KindMisbehaviorSybil,
		ledger.KindWitnessAttest,
		ledger.KindWitnessCheckpoint,
		ledger.KindGroupMembership,
		ledger.KindAgentAction,
		ledger.KindSyncSummary,
		ledger.KindLedgerEvent:
		return PriorityCritical
	case ledger.KindChatMessage,
		ledger.KindGroupCreate,
		ledger.KindGroupJoin:
		return PriorityNormal
	default:
		return PriorityBackground
	}
}

func splitEventsByPriority(msg ledger.EventsMessage) (ledger.EventsMessage, ledger.EventsMessage, ledger.EventsMessage) {
	if len(msg.Events) == 0 {
		return ledger.EventsMessage{SchemaVersion: msg.SchemaVersion}, ledger.EventsMessage{SchemaVersion: msg.SchemaVersion}, ledger.EventsMessage{SchemaVersion: msg.SchemaVersion}
	}
	c := make([]ledger.Event, 0, len(msg.Events))
	n := make([]ledger.Event, 0, len(msg.Events))
	b := make([]ledger.Event, 0, len(msg.Events))
	for i := range msg.Events {
		ev := msg.Events[i]
		switch priorityForKind(ev.Kind) {
		case PriorityCritical:
			c = append(c, ev)
		case PriorityNormal:
			n = append(n, ev)
		default:
			b = append(b, ev)
		}
	}
	return ledger.EventsMessage{SchemaVersion: msg.SchemaVersion, Events: c}, ledger.EventsMessage{SchemaVersion: msg.SchemaVersion, Events: n}, ledger.EventsMessage{SchemaVersion: msg.SchemaVersion, Events: b}
}

func (s *Service) drainInbound(max int) ledger.ApplyReport {
	if s == nil || s.ledger == nil || s.scheduler == nil || max <= 0 {
		return ledger.ApplyReport{}
	}
	rep := ledger.ApplyReport{}
	for i := 0; i < max; i++ {
		it, ok := s.scheduler.Next()
		if !ok {
			break
		}
		msgMax := s.opts.MaxEventsMessageBytes
		if it.maxMessageBytes > 0 && it.maxMessageBytes < msgMax {
			msgMax = it.maxMessageBytes
		}
		ctx := strings.TrimSpace(it.context)
		beforeSeq := uint64(0)
		if s.observer != nil && ctx != "" {
			beforeSeq = s.ledger.AuthorMaxSeq(s.observer.Author)
		}
		now := time.Now()
		rp, _ := s.ledger.ApplyEventsMessageBoundedOptimized(it.msg, it.context, s.policy, s.observer, s.opts.MaxEvents, s.opts.MaxEventBytes, msgMax, ledger.ApplyOptimizations{
			VerifyWorkers:   s.opts.VerifyWorkers,
			VerifyBatchSize: s.opts.VerifyBatchSize,
			Cache:           s.verifyCache,
			ApplyChunkSize:  s.opts.ApplyChunkSize,
		})
		if s.observer != nil && ctx != "" {
			afterSeq := s.ledger.AuthorMaxSeq(s.observer.Author)
			if afterSeq > beforeSeq {
				s.broadcastObserverEvents(ctx, beforeSeq+1, afterSeq)
			}
		}
		rep.Applied += rp.Applied
		rep.Dupe += rp.Dupe
		rep.Rejected += rp.Rejected
		if rp.Dupe > 0 {
			s.stats.duplicate.Add(uint64(rp.Dupe))
		}
		if rp.Rejected > 0 {
			s.stats.rejected.Add(uint64(rp.Rejected))
			if s.security != nil && s.security.TrustTier(it.peerID, now) == TrustBlocked {
				s.stats.quarantined.Add(1)
			}
		}
		if s.security != nil {
			s.security.ObserveApplyReport(it.peerID, it.role, rp, now)
		}
	}
	return rep
}

func (s *Service) broadcastObserverEvents(ctx string, fromSeq uint64, toSeq uint64) {
	if s == nil || s.ledger == nil || s.observer == nil {
		return
	}
	if !s.opts.AllowContext(ctx) {
		return
	}
	author := strings.TrimSpace(s.observer.Author)
	if author == "" {
		return
	}
	ids := s.ledger.AuthorEventIDsBetween(author, fromSeq, toSeq)
	ids = s.filterOutgoingIDs(ids, ctx)
	if len(ids) == 0 {
		return
	}
	msg := s.ledger.BuildEventsMessageBounded(ids, s.opts.MaxEvents, s.opts.MaxEventBytes, s.opts.MaxEventsMessageBytes)
	if len(msg.Events) == 0 {
		return
	}
	sid, err := newSessionID(s.rand)
	if err != nil {
		return
	}
	pt, err := encodeWire("events", sid, ctx, msg)
	if err != nil {
		return
	}

	s.mu.Lock()
	peers := make([]Peer, 0, len(s.peers))
	for id, ps := range s.peers {
		if ps == nil || !ps.meshPubSet {
			continue
		}
		peers = append(peers, Peer{ID: id, MeshPub: ps.meshPub, Role: ps.role})
	}
	s.mu.Unlock()
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })
	if s.opts.MaxPeersPerRound > 0 && len(peers) > s.opts.MaxPeersPerRound {
		peers = peers[:s.opts.MaxPeersPerRound]
	}
	for i := range peers {
		_ = s.sendMesh(peers[i], pt)
	}
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

	maxPeers := min(opts.MaxPeersPerRound, len(all))
	out := make([]Peer, 0, maxPeers)
	relays := 0
	agents := 0
	selected := map[string]struct{}{}
	clusterUsed := map[string]int{}

	tryAdd := func(p Peer, enforceClusterDiversity bool) bool {
		if _, ok := selected[p.ID]; ok {
			return false
		}
		switch p.Role {
		case PeerRoleRelay:
			if relays >= relayLimit {
				return false
			}
		case PeerRoleAgent:
			if agents >= agentLimit {
				return false
			}
		}
		cluster := ""
		if enforceClusterDiversity && s.security != nil {
			cluster = s.security.PeerClusterKey(p.ID, now)
			if cluster != "" && clusterUsed[cluster] > 0 {
				return false
			}
		}
		switch p.Role {
		case PeerRoleRelay:
			relays++
		case PeerRoleAgent:
			agents++
		}
		s.setPeerRole(p.ID, p.Role, now)
		selected[p.ID] = struct{}{}
		if cluster != "" {
			clusterUsed[cluster]++
		}
		out = append(out, p)
		return true
	}

	for i := range all {
		if len(out) >= maxPeers {
			break
		}
		_ = tryAdd(all[i].p, len(out) < maxPeers-1)
	}
	for i := range all {
		if len(out) >= maxPeers {
			break
		}
		_ = tryAdd(all[i].p, false)
	}

	if s.rand != nil && len(out) >= 2 && len(all) > len(out) {
		rn, err := randUint32(s.rand)
		if err == nil {
			last := out[len(out)-1]
			start := len(out)
			if start < len(all) {
				width := len(all) - start
				offset := int(rn % uint32(width))
				tries := width
				if tries > 8 {
					tries = 8
				}
				for i := 0; i < tries; i++ {
					pick := start + ((offset + i) % width)
					cand := all[pick].p
					if cand.Role != last.Role {
						continue
					}
					if _, ok := selected[cand.ID]; ok {
						continue
					}
					out[len(out)-1] = cand
					delete(selected, last.ID)
					selected[cand.ID] = struct{}{}
					s.setPeerRole(cand.ID, cand.Role, now)
					break
				}
			}
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
	if s.opts.Privacy.CoverPeers > 0 && len(candidates) > len(peers) {
		selected := map[string]struct{}{}
		for i := range peers {
			selected[peers[i].ID] = struct{}{}
		}
		extras := make([]Peer, 0, len(candidates))
		for i := range candidates {
			if _, ok := selected[candidates[i].ID]; ok {
				continue
			}
			extras = append(extras, candidates[i])
		}
		extras = shufflePeers(extras, s.rand)
		n := s.opts.Privacy.CoverPeers
		if n > len(extras) {
			n = len(extras)
		}
		for i := 0; i < n; i++ {
			_, _ = s.SendHeads(ctx, extras[i], syncContext)
		}
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

type RelayHop struct {
	Mailbox   relay.MailboxID
	PreKeyPub [32]byte
}

type relayRouteMessage struct {
	SchemaVersion int    `json:"v"`
	NextMailbox   string `json:"next_mailbox"`
	NextEnvelope  string `json:"next_envelope"`
}

func (m relayRouteMessage) validate() error {
	if m.SchemaVersion != 1 {
		return errors.New("ledgersync: invalid relay route schema")
	}
	if err := (relay.PullRequest{Mailbox: relay.MailboxID(m.NextMailbox)}).Validate(); err != nil {
		return errors.New("ledgersync: invalid next mailbox")
	}
	if err := (relay.PushRequest{Mailbox: relay.MailboxID(m.NextMailbox), Envelope: relay.Envelope(m.NextEnvelope)}).Validate(); err != nil {
		return errors.New("ledgersync: invalid next envelope")
	}
	return nil
}

func EncodeForRelayMultiHop(plaintext []byte, recipientMailbox relay.MailboxID, recipientPreKeyPub [32]byte, hops []RelayHop) (relay.MailboxID, string, error) {
	env, err := EncodeForRelay(plaintext, recipientPreKeyPub)
	if err != nil {
		return "", "", err
	}
	nextMailbox := recipientMailbox
	nextEnv := env
	for i := len(hops) - 1; i >= 0; i-- {
		layer := relayRouteMessage{
			SchemaVersion: 1,
			NextMailbox:   string(nextMailbox),
			NextEnvelope:  nextEnv,
		}
		b, err := json.Marshal(layer)
		if err != nil {
			return "", "", err
		}
		wrapped, err := EncodeForRelay(b, hops[i].PreKeyPub)
		if err != nil {
			return "", "", err
		}
		nextMailbox = hops[i].Mailbox
		nextEnv = wrapped
	}
	return nextMailbox, nextEnv, nil
}

func ForwardRelayOnce(ctx context.Context, r relay.Relay, mailbox relay.MailboxID, hopPreKeyPriv [32]byte, limit int, ack bool, rnd ioRand, delayMax time.Duration, ttlSec int, powBits int) (int, int, error) {
	if r == nil {
		return 0, 0, errors.New("ledgersync: relay is nil")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	msgs, err := r.Pull(ctx, relay.PullRequest{Mailbox: mailbox, Limit: limit})
	if err != nil {
		return 0, 0, err
	}
	if rnd == nil {
		rnd = cryptoRand
	}

	forwarded := 0
	ackIDs := make([]relay.MessageID, 0, len(msgs))
	for i := range msgs {
		pt, _, err := DecodeFromRelay(string(msgs[i].Envelope), hopPreKeyPriv)
		if err != nil {
			continue
		}
		var m relayRouteMessage
		if err := json.Unmarshal(pt, &m); err != nil {
			continue
		}
		if err := m.validate(); err != nil {
			continue
		}

		req := relay.PushRequest{
			Mailbox:        relay.MailboxID(m.NextMailbox),
			Envelope:       relay.Envelope(m.NextEnvelope),
			TTLSec:         ttlSec,
			PoWBits:        powBits,
			PoWNonceB64URL: "",
		}
		if delayMax > 0 {
			d := randDuration(rnd, delayMax)
			if d >= time.Second {
				ds := int(d.Round(time.Second).Seconds())
				if ds > 3600 {
					ds = 3600
				}
				req.DelaySec = ds
			}
		}
		if _, err := r.Push(ctx, req); err != nil {
			continue
		}
		forwarded++
		if ack {
			ackIDs = append(ackIDs, msgs[i].ID)
		}
	}
	if ack && len(ackIDs) > 0 {
		_ = r.Ack(ctx, relay.AckRequest{Mailbox: mailbox, IDs: ackIDs})
	}
	return forwarded, len(msgs), nil
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
		if s.opts.Privacy.OutboundDelayMax > 0 {
			if d := randDuration(s.rand, s.opts.Privacy.OutboundDelayMax); d > 0 {
				time.Sleep(d)
			}
		}
		if s.opts.Privacy.PaddingBucketBytes > 0 {
			ppt, perr := padPlaintext(pt, s.opts.Privacy.PaddingBucketBytes, s.rand)
			if perr == nil {
				pt = ppt
			}
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

func (s *Service) PublishEventsToRelayMultiHop(ctx context.Context, r relay.Relay, hops []RelayHop, recipientMailbox relay.MailboxID, recipientPreKeyPub [32]byte, syncContext string, want []string, ttlSec int, powBits int) (int, error) {
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
		if s.opts.Privacy.OutboundDelayMax > 0 {
			if d := randDuration(s.rand, s.opts.Privacy.OutboundDelayMax); d > 0 {
				time.Sleep(d)
			}
		}
		if s.opts.Privacy.PaddingBucketBytes > 0 {
			ppt, perr := padPlaintext(pt, s.opts.Privacy.PaddingBucketBytes, s.rand)
			if perr == nil {
				pt = ppt
			}
		}
		if len(pt) > relayPlainMax {
			return sentEvents, errors.New("ledgersync: relay payload too large")
		}
		firstMailbox, firstEnv, err := EncodeForRelayMultiHop(pt, recipientMailbox, recipientPreKeyPub, hops)
		if err != nil {
			return sentEvents, err
		}

		req := relay.PushRequest{
			Mailbox:        firstMailbox,
			Envelope:       relay.Envelope(firstEnv),
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
	droppedTotal := 0

	for i := range msgs {
		ackThis := false
		envStr := string(msgs[i].Envelope)
		pt, _, err := DecodeFromRelay(envStr, recipientPreKeyPriv)
		if err != nil {
			continue
		}
		if !s.allowInbound(peerID, len(pt), now) {
			continue
		}
		upt, _ := unpadPlaintext(pt)
		mid := msgIDFromPlaintext(upt)
		if s.alreadySeenMsg(peerID, mid, now) {
			if ack {
				ackIDs = append(ackIDs, msgs[i].ID)
			}
			continue
		}

		w, err := decodeWire(upt)
		if err != nil {
			if ack {
				_ = s.seenMsg(peerID, mid, now)
				ackIDs = append(ackIDs, msgs[i].ID)
			}
			continue
		}
		if !s.opts.AllowContext(w.Context) {
			if ack {
				_ = s.seenMsg(peerID, mid, now)
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
					_ = s.seenMsg(peerID, mid, now)
					ackIDs = append(ackIDs, msgs[i].ID)
				}
				continue
			}
			if em.SchemaVersion != ledger.SyncSchemaV1 {
				if ack {
					_ = s.seenMsg(peerID, mid, now)
					ackIDs = append(ackIDs, msgs[i].ID)
				}
				continue
			}
			em, dropped := s.filterIncomingEvents(em, w.Context)
			droppedTotal += dropped
			if dropped > 0 {
				s.stats.rejected.Add(uint64(dropped))
			}
			cm, nm, bm := splitEventsByPriority(em)
			enq := 0
			if len(cm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityCritical, schedItem{role: role, context: w.Context, msg: cm, maxMessageBytes: ledgerMsgMax}) {
				enq++
			}
			if len(nm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityNormal, schedItem{role: role, context: w.Context, msg: nm, maxMessageBytes: ledgerMsgMax}) {
				enq++
			}
			if len(bm.Events) > 0 && s.scheduler != nil && s.scheduler.Enqueue(peerID, PriorityBackground, schedItem{role: role, context: w.Context, msg: bm, maxMessageBytes: ledgerMsgMax}) {
				enq++
			}
			if enq == 0 {
				s.stats.deferred.Add(1)
				continue
			}
			ackThis = ack
		default:
		}
		if ackThis {
			_ = s.seenMsg(peerID, mid, now)
			ackIDs = append(ackIDs, msgs[i].ID)
		}
	}

	rp := s.drainInbound(s.opts.SchedulerDrainAfterRelay)
	rep.Applied += rp.Applied
	rep.Dupe += rp.Dupe
	rep.Rejected += rp.Rejected + droppedTotal

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
