package ledger

import (
	"container/list"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const SyncSchemaV1 = 1

type HeadsMessage struct {
	SchemaVersion int      `json:"v"`
	Heads         []string `json:"heads"`
}

type InventoryMessage struct {
	SchemaVersion int      `json:"v"`
	Heads         []string `json:"heads"`
	Have          []string `json:"have,omitempty"`
}

type SyncPlan struct {
	Want []string `json:"want"`
}

type WantMessage struct {
	SchemaVersion int      `json:"v"`
	Want          []string `json:"want"`
}

type EventsMessage struct {
	SchemaVersion int     `json:"v"`
	Events        []Event `json:"events"`
}

func (l *Ledger) BuildHeadsMessage() HeadsMessage {
	return HeadsMessage{SchemaVersion: SyncSchemaV1, Heads: l.Heads()}
}

func (l *Ledger) BuildInventoryMessage(maxHave int) InventoryMessage {
	if l == nil {
		return InventoryMessage{SchemaVersion: SyncSchemaV1}
	}
	if maxHave < 0 {
		maxHave = 256
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	var have []string
	if maxHave > 0 {
		have = make([]string, 0, min(maxHave, len(l.events)))
		for id := range l.events {
			have = append(have, id)
			if len(have) >= maxHave {
				break
			}
		}
		sort.Strings(have)
	}
	heads := make([]string, 0, len(l.heads))
	for h := range l.heads {
		heads = append(heads, h)
	}
	sort.Strings(heads)
	return InventoryMessage{
		SchemaVersion: SyncSchemaV1,
		Heads:         heads,
		Have:          have,
	}
}

func (l *Ledger) PlanSyncFromInventory(peer InventoryMessage, maxWant int) SyncPlan {
	if l == nil {
		return SyncPlan{}
	}
	if maxWant <= 0 {
		maxWant = 256
	}
	peerHave := make(map[string]struct{}, len(peer.Have))
	for _, id := range peer.Have {
		if id == "" {
			continue
		}
		peerHave[id] = struct{}{}
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	want := make([]string, 0, min(maxWant, len(l.events)))
	for id := range l.events {
		if _, ok := peerHave[id]; ok {
			continue
		}
		want = append(want, id)
		if len(want) >= maxWant {
			break
		}
	}
	sort.Strings(want)
	return SyncPlan{Want: want}
}

func (l *Ledger) PlanSyncToPeer(peer InventoryMessage, maxWant int) SyncPlan {
	return l.PlanSyncFromInventory(peer, maxWant)
}

func (l *Ledger) PlanSyncFromPeer(peer InventoryMessage, maxWant int) SyncPlan {
	if l == nil {
		return SyncPlan{}
	}
	if maxWant <= 0 {
		maxWant = 256
	}

	peerHas := make(map[string]struct{}, len(peer.Have)+len(peer.Heads))
	for _, id := range peer.Have {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		peerHas[id] = struct{}{}
	}
	for _, id := range peer.Heads {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		peerHas[id] = struct{}{}
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	want := make([]string, 0, maxWant)
	wantSet := make(map[string]struct{})
	addWant := func(id string) bool {
		if id == "" {
			return false
		}
		if len(want) >= maxWant {
			return false
		}
		if _, ok := l.events[id]; ok {
			return false
		}
		if _, ok := peerHas[id]; !ok {
			return false
		}
		if _, ok := wantSet[id]; ok {
			return false
		}
		wantSet[id] = struct{}{}
		want = append(want, id)
		return true
	}

	missing := make([]string, 0, len(l.missingRefs))
	for id := range l.missingRefs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		missing = append(missing, id)
	}
	sort.Slice(missing, func(i, j int) bool {
		a := missing[i]
		b := missing[j]
		wa := 0
		wb := 0
		if l.missingRefWeight != nil {
			wa = l.missingRefWeight[a]
			wb = l.missingRefWeight[b]
		}
		if wa != wb {
			return wa > wb
		}
		ca := l.missingRefs[a]
		cb := l.missingRefs[b]
		if ca != cb {
			return ca > cb
		}
		return a < b
	})
	for _, id := range missing {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}

	heads := append([]string(nil), peer.Heads...)
	for i := range heads {
		heads[i] = strings.TrimSpace(heads[i])
	}
	sort.Slice(heads, func(i, j int) bool {
		a := heads[i]
		b := heads[j]
		wa := 0
		wb := 0
		if l.missingRefWeight != nil {
			wa = l.missingRefWeight[a]
			wb = l.missingRefWeight[b]
		}
		if wa != wb {
			return wa > wb
		}
		ca := 0
		cb := 0
		if l.missingRefs != nil {
			ca = l.missingRefs[a]
			cb = l.missingRefs[b]
		}
		if ca != cb {
			return ca > cb
		}
		return a < b
	})
	for _, id := range heads {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}
	return SyncPlan{Want: want}
}

func (p SyncPlan) WantMessage() WantMessage {
	return WantMessage{SchemaVersion: SyncSchemaV1, Want: append([]string(nil), p.Want...)}
}

func (l *Ledger) BuildEventsMessage(want []string, maxEvents int) EventsMessage {
	if l == nil {
		return EventsMessage{SchemaVersion: SyncSchemaV1}
	}
	if maxEvents <= 0 {
		maxEvents = 256
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Event, 0, min(maxEvents, len(want)))
	for _, id := range want {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ev, ok := l.events[id]
		if !ok {
			continue
		}
		out = append(out, ev)
		if len(out) >= maxEvents {
			break
		}
	}
	return EventsMessage{SchemaVersion: SyncSchemaV1, Events: out}
}

func (l *Ledger) BuildEventsMessageBounded(want []string, maxEvents int, maxEventBytes int, maxMessageBytes int) EventsMessage {
	if l == nil {
		return EventsMessage{SchemaVersion: SyncSchemaV1}
	}
	if maxEvents <= 0 {
		maxEvents = 256
	}
	if maxEventBytes <= 0 {
		maxEventBytes = 256 * 1024
	}
	if maxMessageBytes <= 0 {
		maxMessageBytes = 1024 * 1024
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	seen := map[string]struct{}{}
	out := make([]Event, 0, min(maxEvents, len(want)))
	bytesUsed := 0
	for _, id := range want {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ev, ok := l.events[id]
		if !ok {
			continue
		}
		raw, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		if len(raw) > maxEventBytes {
			continue
		}
		if bytesUsed+len(raw) > maxMessageBytes {
			break
		}
		bytesUsed += len(raw)
		out = append(out, ev)
		if len(out) >= maxEvents {
			break
		}
	}
	return EventsMessage{SchemaVersion: SyncSchemaV1, Events: out}
}

type ApplyReport struct {
	Applied  int
	Dupe     int
	Rejected int
}

func (l *Ledger) ApplyEventsMessage(msg EventsMessage, context string, pol *Policy, observer *Observer) (ApplyReport, error) {
	if l == nil {
		return ApplyReport{}, errors.New("ledger: ledger is nil")
	}
	if msg.SchemaVersion != SyncSchemaV1 {
		return ApplyReport{}, errors.New("ledger: unsupported sync schema")
	}
	events := orderEvents(msg.Events)
	rep := ApplyReport{}
	for i := range events {
		ev := events[i]
		if l.Have(ev.ID) {
			rep.Dupe++
			continue
		}
		if _, err := l.ApplyWithContext(ev, context, pol, observer); err != nil {
			rep.Rejected++
			continue
		}
		rep.Applied++
	}
	return rep, nil
}

func (l *Ledger) ApplyEventsMessageBounded(msg EventsMessage, context string, pol *Policy, observer *Observer, maxEvents int, maxEventBytes int, maxMessageBytes int) (ApplyReport, error) {
	if l == nil {
		return ApplyReport{}, errors.New("ledger: ledger is nil")
	}
	if msg.SchemaVersion != SyncSchemaV1 {
		return ApplyReport{}, errors.New("ledger: unsupported sync schema")
	}
	if maxEvents <= 0 {
		maxEvents = 256
	}
	if maxEventBytes <= 0 {
		maxEventBytes = 256 * 1024
	}
	if maxMessageBytes <= 0 {
		maxMessageBytes = 1024 * 1024
	}
	if len(msg.Events) > maxEvents {
		return ApplyReport{}, errors.New("ledger: events message too large")
	}

	events := orderEvents(msg.Events)
	rep := ApplyReport{}
	bytesUsed := 0
	for i := range events {
		ev := events[i]
		raw, err := json.Marshal(ev)
		if err != nil {
			rep.Rejected++
			continue
		}
		if len(raw) > maxEventBytes {
			rep.Rejected++
			continue
		}
		if bytesUsed+len(raw) > maxMessageBytes {
			return rep, errors.New("ledger: events message too large")
		}
		bytesUsed += len(raw)

		if l.Have(ev.ID) {
			rep.Dupe++
			continue
		}
		if _, err := l.ApplyWithContext(ev, context, pol, observer); err != nil {
			rep.Rejected++
			continue
		}
		rep.Applied++
	}
	return rep, nil
}

type VerifiedEventCache struct {
	mu  sync.Mutex
	max int
	ttl time.Duration
	ll  *list.List
	m   map[string]*list.Element
}

type verifiedEventCacheEntry struct {
	key string
	exp time.Time
}

func NewVerifiedEventCache(max int, ttl time.Duration) *VerifiedEventCache {
	if max <= 0 {
		max = 4096
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &VerifiedEventCache{
		max: max,
		ttl: ttl,
		ll:  list.New(),
		m:   map[string]*list.Element{},
	}
}

func (c *VerifiedEventCache) match(id string, authorKeyB64 string, sigB64 string, now time.Time) bool {
	if c == nil || c.max <= 0 || c.ttl <= 0 {
		return false
	}
	key := id + "\x00" + authorKeyB64 + "\x00" + sigB64
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.m[key]
	if !ok {
		return false
	}
	ent := el.Value.(*verifiedEventCacheEntry)
	if !ent.exp.After(now) {
		c.ll.Remove(el)
		delete(c.m, key)
		return false
	}
	c.ll.MoveToFront(el)
	return true
}

func (c *VerifiedEventCache) put(id string, authorKeyB64 string, sigB64 string, now time.Time) {
	if c == nil || c.max <= 0 || c.ttl <= 0 {
		return
	}
	key := id + "\x00" + authorKeyB64 + "\x00" + sigB64
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		ent := el.Value.(*verifiedEventCacheEntry)
		ent.exp = now.Add(c.ttl)
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&verifiedEventCacheEntry{key: key, exp: now.Add(c.ttl)})
	c.m[key] = el
	for c.ll.Len() > c.max {
		last := c.ll.Back()
		if last == nil {
			break
		}
		ent := last.Value.(*verifiedEventCacheEntry)
		delete(c.m, ent.key)
		c.ll.Remove(last)
	}
}

type BatchReport struct {
	Total     int
	Unique    int
	Precheck  int
	Cached    int
	Verified  int
	Rejected  int
	BadSig    int
	BadDecode int
}

type VerifyBatchOptions struct {
	VerifyWorkers   int
	VerifyBatchSize int
	Cache           *VerifiedEventCache
}

func VerifyBatch(events []Event, pol *Policy, opts VerifyBatchOptions) (BatchReport, []bool) {
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	workers := opts.VerifyWorkers
	if workers <= 0 {
		workers = 1
	}
	batchSize := opts.VerifyBatchSize
	if batchSize <= 0 {
		batchSize = 64
	}

	rep := BatchReport{Total: len(events)}
	out := make([]bool, len(events))
	if len(events) == 0 {
		return rep, out
	}

	indicesByID := map[string][]int{}
	empty := 0
	for i := range events {
		id := strings.TrimSpace(events[i].ID)
		if id == "" {
			empty++
			continue
		}
		indicesByID[id] = append(indicesByID[id], i)
	}
	rep.Unique = len(indicesByID)

	now := time.Now()
	type task struct {
		id   string
		idx  int
		hash [32]byte
	}
	tasks := make([]task, 0, rep.Unique)
	okByID := map[string]bool{}

	for id, idxs := range indicesByID {
		idx := idxs[0]
		hash, err := validateWithPolicyNoCrypto(events[idx], p)
		if err != nil {
			okByID[id] = false
			continue
		}
		wantID := base64.RawURLEncoding.EncodeToString(hash[:])
		if id != wantID {
			okByID[id] = false
			continue
		}
		rep.Precheck++
		if opts.Cache != nil && opts.Cache.match(id, events[idx].AuthorKeyB64, events[idx].SigB64, now) {
			okByID[id] = true
			rep.Cached++
			continue
		}
		tasks = append(tasks, task{id: id, idx: idx, hash: hash})
	}

	if len(tasks) > 0 {
		if batchSize > len(tasks) {
			batchSize = len(tasks)
		}
		workCh := make(chan task, min(batchSize, len(tasks)))
		var wg sync.WaitGroup
		var mu sync.Mutex
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range workCh {
					ev := events[t.idx]
					pub, err := ev.AuthorPublicKey()
					if err != nil {
						mu.Lock()
						rep.BadDecode++
						okByID[t.id] = false
						mu.Unlock()
						continue
					}
					sig, err := base64.RawURLEncoding.DecodeString(ev.SigB64)
					if err != nil {
						mu.Lock()
						rep.BadDecode++
						okByID[t.id] = false
						mu.Unlock()
						continue
					}
					if !ed25519.Verify(pub, t.hash[:], sig) {
						mu.Lock()
						rep.BadSig++
						okByID[t.id] = false
						mu.Unlock()
						continue
					}
					if opts.Cache != nil {
						opts.Cache.put(ev.ID, ev.AuthorKeyB64, ev.SigB64, now)
					}
					mu.Lock()
					okByID[t.id] = true
					mu.Unlock()
				}
			}()
		}
		for _, t := range tasks {
			workCh <- t
		}
		close(workCh)
		wg.Wait()
	}

	for id, idxs := range indicesByID {
		ok := okByID[id]
		if ok {
			rep.Verified++
		}
		for _, i := range idxs {
			out[i] = ok
		}
	}
	rep.Rejected = empty
	for id, idxs := range indicesByID {
		if !okByID[id] {
			rep.Rejected += len(idxs)
		}
	}
	return rep, out
}

type ApplyOptimizations struct {
	VerifyWorkers   int
	VerifyBatchSize int
	Cache           *VerifiedEventCache
	ApplyChunkSize  int
}

func (l *Ledger) ApplyEventsMessageBoundedOptimized(msg EventsMessage, context string, pol *Policy, observer *Observer, maxEvents int, maxEventBytes int, maxMessageBytes int, opt ApplyOptimizations) (ApplyReport, error) {
	if l == nil {
		return ApplyReport{}, errors.New("ledger: ledger is nil")
	}
	if msg.SchemaVersion != SyncSchemaV1 {
		return ApplyReport{}, errors.New("ledger: unsupported sync schema")
	}
	if maxEvents <= 0 {
		maxEvents = 256
	}
	if maxEventBytes <= 0 {
		maxEventBytes = 256 * 1024
	}
	if maxMessageBytes <= 0 {
		maxMessageBytes = 1024 * 1024
	}
	if len(msg.Events) > maxEvents {
		return ApplyReport{}, errors.New("ledger: events message too large")
	}

	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}

	workers := opt.VerifyWorkers
	if workers <= 0 {
		workers = 1
	}
	batchSize := opt.VerifyBatchSize
	if batchSize <= 0 {
		batchSize = 64
	}
	chunkSize := opt.ApplyChunkSize
	if chunkSize <= 0 {
		chunkSize = 64
	}

	events := orderEvents(msg.Events)
	rep := ApplyReport{}
	bytesUsed := 0
	now := time.Now()

	type task struct {
		idx  int
		hash [32]byte
	}
	var tasks []task
	seenIDs := map[string]struct{}{}
	candidate := make([]bool, len(events))
	verified := make([]bool, len(events))

	for i := range events {
		ev := events[i]
		raw, err := json.Marshal(ev)
		if err != nil {
			rep.Rejected++
			continue
		}
		if len(raw) > maxEventBytes {
			rep.Rejected++
			continue
		}
		if bytesUsed+len(raw) > maxMessageBytes {
			return rep, errors.New("ledger: events message too large")
		}
		bytesUsed += len(raw)

		id := strings.TrimSpace(ev.ID)
		if id == "" {
			rep.Rejected++
			continue
		}
		if _, ok := seenIDs[id]; ok {
			rep.Dupe++
			continue
		}
		seenIDs[id] = struct{}{}

		if l.Have(ev.ID) {
			rep.Dupe++
			continue
		}
		hash, err := validateWithPolicyNoCrypto(ev, p)
		if err != nil {
			rep.Rejected++
			continue
		}
		wantID := base64.RawURLEncoding.EncodeToString(hash[:])
		if ev.ID != wantID {
			rep.Rejected++
			continue
		}
		candidate[i] = true
		if opt.Cache != nil && opt.Cache.match(ev.ID, ev.AuthorKeyB64, ev.SigB64, now) {
			verified[i] = true
			continue
		}
		tasks = append(tasks, task{idx: i, hash: hash})
	}

	if len(tasks) > 0 {
		if batchSize > len(tasks) {
			batchSize = len(tasks)
		}
		workCh := make(chan task, min(batchSize, len(tasks)))
		var wg sync.WaitGroup
		var mu sync.Mutex
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range workCh {
					ev := events[t.idx]
					pub, err := ev.AuthorPublicKey()
					if err != nil {
						continue
					}
					sig, err := base64.RawURLEncoding.DecodeString(ev.SigB64)
					if err != nil {
						continue
					}
					if !ed25519.Verify(pub, t.hash[:], sig) {
						continue
					}
					mu.Lock()
					verified[t.idx] = true
					mu.Unlock()
					if opt.Cache != nil {
						opt.Cache.put(ev.ID, ev.AuthorKeyB64, ev.SigB64, now)
					}
				}
			}()
		}
		for _, t := range tasks {
			workCh <- t
		}
		close(workCh)
		wg.Wait()
	}

	processed := 0
	for i := range events {
		if !candidate[i] || !verified[i] {
			if candidate[i] && !verified[i] {
				rep.Rejected++
			}
		} else {
			if _, err := l.ApplyVerifiedWithContext(events[i], context, pol, observer); err != nil {
				rep.Rejected++
			} else {
				rep.Applied++
			}
		}
		processed++
		if processed%chunkSize == 0 {
			runtime.Gosched()
		}
	}
	return rep, nil
}

func orderEvents(in []Event) []Event {
	if len(in) <= 1 {
		return in
	}
	out := append([]Event(nil), in...)
	refCount := map[string]int{}
	inBatch := map[string]struct{}{}
	for i := range out {
		id := strings.TrimSpace(out[i].ID)
		if id == "" {
			continue
		}
		inBatch[id] = struct{}{}
	}
	for i := range out {
		ev := out[i]
		if strings.TrimSpace(ev.Prev) != "" {
			if _, ok := inBatch[ev.Prev]; ok {
				refCount[ev.Prev]++
			}
		}
		for _, p := range ev.Parents {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := inBatch[p]; ok {
				refCount[p]++
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i]
		b := out[j]
		pa := kindPriority(a.Kind)
		pb := kindPriority(b.Kind)
		if pa != pb {
			return pa < pb
		}
		ra := refCount[a.ID]
		rb := refCount[b.ID]
		if ra != rb {
			return ra > rb
		}
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt < b.CreatedAt
		}
		return a.ID < b.ID
	})
	return out
}

func kindPriority(kind string) int {
	switch strings.TrimSpace(kind) {
	case KindIdentityRevoke, KindIdentityRotate, KindIdentityIntroduce:
		return 0
	case KindMisbehaviorEquivoc, KindMisbehaviorReplay, KindMisbehaviorSybil:
		return 1
	case KindWitnessCheckpoint, KindWitnessAttest:
		return 2
	case KindGroupMembership, KindGroupCreate, KindGroupJoin:
		return 3
	case KindAgentAction:
		return 4
	case KindChatMessage:
		return 5
	case KindSyncSummary, KindLedgerEvent:
		return 7
	default:
		return 10
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
