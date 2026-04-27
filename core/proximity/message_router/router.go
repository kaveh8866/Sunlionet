package message_router

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/anti_flood_guard"
	"github.com/kaveh/sunlionet-agent/core/proximity/cache_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/gossip_controller"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/partition_sync"
	"github.com/kaveh/sunlionet-agent/core/proximity/relay_scoring"
	"github.com/kaveh/sunlionet-agent/core/proximity/replay_protection"
	"github.com/kaveh/sunlionet-agent/core/proximity/routing_engine"
)

type Transport interface {
	Broadcast(data []byte) error
	Receive(ctx context.Context) ([]byte, error)
}

type TransportFrom interface {
	Broadcast(data []byte) error
	ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error)
}

type Router struct {
	transport Transport
	cache     *cache_manager.Cache
	reasm     *Reassembler

	currentIdentity func(now time.Time) identity_manager.Identity

	maxFrameBytes       int
	maxHop              uint8
	minRebroadcastEvery time.Duration
	clockSkewAllowance  time.Duration
	maxPayloadBytes     int

	scorer *relay_scoring.Scorer
	guard  *anti_flood_guard.Guard
	replay *replay_protection.Protector
	gossip *gossip_controller.Controller
	route  *routing_engine.Engine

	invEvery time.Duration
	invMax   int
	wantMax  int

	coverMu     sync.RWMutex
	coverEvery  time.Duration
	coverSize   int
	lastInvAt   time.Time
	lastCoverAt time.Time
	coverUpdate chan struct{}

	lastForward map[MessageID]time.Time
	onMessage   func(Message)

	validate func(Message) bool
}

type Options struct {
	MaxFrameBytes        int
	MaxHop               uint8
	MinRebroadcastEvery  time.Duration
	ReassemblyStaleAfter time.Duration
	CacheMaxItems        int
	ClockSkewAllowance   time.Duration
	MaxPayloadBytes      int

	ReplayBucketDuration time.Duration
	ReplayWindowDuration time.Duration

	GlobalRateCapacity float64
	GlobalRate         float64
	PerSenderCapacity  float64
	PerSenderRate      float64

	GossipSeed int64

	RoutingBaseProb     float64
	RoutingMaxJitter    time.Duration
	RoutingMinTTLRemain time.Duration

	InventoryEvery time.Duration
	InventoryMax   int
	WantMax        int

	CoverTrafficEvery time.Duration
	CoverTrafficSize  int

	Validate func(Message) bool
}

func NewRouter(transport Transport, identityFn func(now time.Time) identity_manager.Identity, opts Options) (*Router, error) {
	if transport == nil {
		return nil, errors.New("transport required")
	}
	if identityFn == nil {
		return nil, errors.New("identity function required")
	}
	if opts.MaxFrameBytes <= 0 {
		opts.MaxFrameBytes = 220
	}
	if opts.MaxHop == 0 {
		opts.MaxHop = 6
	}
	if opts.MinRebroadcastEvery <= 0 {
		opts.MinRebroadcastEvery = 800 * time.Millisecond
	}
	if opts.ClockSkewAllowance <= 0 {
		opts.ClockSkewAllowance = 2 * time.Minute
	}
	if opts.MaxPayloadBytes <= 0 {
		opts.MaxPayloadBytes = 32 * 1024
	}
	if opts.InventoryEvery <= 0 {
		opts.InventoryEvery = 15 * time.Second
	}
	if opts.InventoryMax <= 0 {
		opts.InventoryMax = 12
	}
	if opts.WantMax <= 0 {
		opts.WantMax = 10
	}
	if opts.CoverTrafficEvery < 0 {
		opts.CoverTrafficEvery = 0
	}
	if opts.CoverTrafficSize <= 0 {
		opts.CoverTrafficSize = 24
	}
	now := time.Now()
	r := &Router{
		transport:           transport,
		cache:               cache_manager.New(opts.CacheMaxItems),
		reasm:               NewReassemblerWithLimit(opts.ReassemblyStaleAfter, opts.MaxPayloadBytes),
		currentIdentity:     identityFn,
		maxFrameBytes:       opts.MaxFrameBytes,
		maxHop:              opts.MaxHop,
		minRebroadcastEvery: opts.MinRebroadcastEvery,
		clockSkewAllowance:  opts.ClockSkewAllowance,
		maxPayloadBytes:     opts.MaxPayloadBytes,
		scorer:              relay_scoring.New(),
		guard: anti_flood_guard.New(anti_flood_guard.Options{
			GlobalCapacity: opts.GlobalRateCapacity,
			GlobalRate:     opts.GlobalRate,
			PerCapacity:    opts.PerSenderCapacity,
			PerRate:        opts.PerSenderRate,
		}, now),
		replay: replay_protection.New(replay_protection.Options{
			BucketDuration: opts.ReplayBucketDuration,
			WindowDuration: opts.ReplayWindowDuration,
		}),
		gossip:      gossip_controller.New(opts.GossipSeed),
		route:       routing_engine.New(),
		invEvery:    opts.InventoryEvery,
		invMax:      opts.InventoryMax,
		wantMax:     opts.WantMax,
		coverEvery:  opts.CoverTrafficEvery,
		coverSize:   opts.CoverTrafficSize,
		lastCoverAt: now,
		coverUpdate: make(chan struct{}, 1),
		lastForward: make(map[MessageID]time.Time),
		validate:    opts.Validate,
	}
	if opts.RoutingBaseProb > 0 {
		r.route.BaseProb = opts.RoutingBaseProb
	}
	if opts.RoutingMaxJitter > 0 {
		r.route.MaxJitter = opts.RoutingMaxJitter
	}
	if opts.RoutingMinTTLRemain > 0 {
		r.route.MinTTLRemain = opts.RoutingMinTTLRemain
	}
	return r, nil
}

func (r *Router) SetOnMessage(fn func(Message)) {
	r.onMessage = fn
}

func (r *Router) Send(now time.Time, payload []byte, ttlSec uint16) (MessageID, error) {
	id := r.currentIdentity(now)
	msg, err := NewMessage(id.NodeID, now, ttlSec, payload)
	if err != nil {
		return MessageID{}, err
	}
	if r.validate != nil && !r.validate(msg) {
		return MessageID{}, errors.New("message rejected by validator")
	}
	r.cache.Put(cache_manager.Entry{
		ID:        msg.ID,
		Sender:    msg.Sender,
		Timestamp: msg.Timestamp,
		TTLSec:    msg.TTLSec,
		Inserted:  now,
		ExpiresAt: msg.ExpiresAt(),
		Payload:   msg.Payload,
	}, now)
	if err := r.broadcastMessage(msg, now); err != nil {
		return MessageID{}, err
	}
	return msg.ID, nil
}

func (r *Router) Run(ctx context.Context) error {
	sweepTicker := time.NewTicker(5 * time.Second)
	defer sweepTicker.Stop()
	invTicker := time.NewTicker(r.invEvery)
	defer invTicker.Stop()
	var coverTimer *time.Timer
	var coverC <-chan time.Time
	r.coverMu.RLock()
	every := r.coverEvery
	last := r.lastCoverAt
	r.coverMu.RUnlock()
	if every > 0 {
		wait := every - time.Since(last)
		if wait < 0 {
			wait = 0
		}
		coverTimer = time.NewTimer(wait)
		coverC = coverTimer.C
	}

	type rx struct {
		raw     []byte
		from    identity_manager.NodeID
		hasFrom bool
	}

	framesCh := make(chan rx, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(framesCh)
		for {
			var (
				b   []byte
				fr  identity_manager.NodeID
				has bool
				err error
			)
			if tf, ok := r.transport.(TransportFrom); ok {
				b, fr, err = tf.ReceiveFrom(ctx)
				has = true
			} else {
				b, err = r.transport.Receive(ctx)
			}
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			select {
			case framesCh <- rx{raw: b, from: fr, hasFrom: has}:
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			if coverTimer != nil {
				coverTimer.Stop()
			}
			return ctx.Err()
		case <-sweepTicker.C:
			now := time.Now()
			_ = r.cache.Sweep(now)
			_ = r.reasm.Sweep(now)
		case <-invTicker.C:
			r.maybeBroadcastInventory(time.Now())
		case <-coverC:
			now := time.Now()
			r.broadcastCover(now)
			r.coverMu.Lock()
			every = r.coverEvery
			r.lastCoverAt = now
			r.coverMu.Unlock()
			if coverTimer != nil && every > 0 {
				coverTimer.Reset(every)
			}
		case <-r.coverUpdate:
			r.coverMu.RLock()
			every = r.coverEvery
			last = r.lastCoverAt
			r.coverMu.RUnlock()
			if every <= 0 {
				if coverTimer != nil {
					if !coverTimer.Stop() {
						select {
						case <-coverTimer.C:
						default:
						}
					}
				}
				coverTimer = nil
				coverC = nil
				continue
			}
			wait := every - time.Since(last)
			if wait < 0 {
				wait = 0
			}
			if coverTimer == nil {
				coverTimer = time.NewTimer(wait)
				coverC = coverTimer.C
				continue
			}
			if !coverTimer.Stop() {
				select {
				case <-coverTimer.C:
				default:
				}
			}
			coverTimer.Reset(wait)
		case err := <-errCh:
			return err
		case p, ok := <-framesCh:
			if !ok {
				return ctx.Err()
			}
			r.handleFrame(p.raw, p.from, p.hasFrom, time.Now())
		}
	}
}

func (r *Router) SetCoverTraffic(every time.Duration, size int, now time.Time) {
	if every < 0 {
		every = 0
	}
	if size <= 0 {
		size = 24
	}
	r.coverMu.Lock()
	r.coverEvery = every
	r.coverSize = size
	r.lastCoverAt = now
	r.coverMu.Unlock()
	select {
	case r.coverUpdate <- struct{}{}:
	default:
	}
}

func (r *Router) handleFrame(raw []byte, from identity_manager.NodeID, hasFrom bool, now time.Time) {
	ver, kind, ok := DecodeHeader(raw)
	if !ok || ver != 1 {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if hasFrom {
		r.scorer.OnSeen(from, now)
		if !r.scorer.AllowFrom(from, now) {
			return
		}
	}

	switch kind {
	case wireKindChunk:
		r.handleChunk(raw, from, hasFrom, now)
	case partition_sync.KindInventory:
		r.handleInventory(raw, from, hasFrom, now)
	case partition_sync.KindWant:
		r.handleWant(raw, from, hasFrom, now)
	case partition_sync.KindCover:
		return
	default:
		return
	}
}

func (r *Router) handleChunk(raw []byte, from identity_manager.NodeID, hasFrom bool, now time.Time) {
	frame, err := DecodeChunkFrame(raw)
	if err != nil {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}

	if frame.Timestamp.After(now.Add(r.clockSkewAllowance)) {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if !now.Before(frame.ExpiresAt()) {
		return
	}
	if frame.Hop > r.maxHop {
		return
	}
	limitKey := frame.Sender
	if hasFrom {
		limitKey = from
	}
	cost := 0.25 + 0.03*float64(frame.Hop)
	if len(raw) > 220 {
		cost += 0.25
	}
	if !r.guard.Allow(limitKey, now, cost) {
		if hasFrom {
			r.scorer.OnRateLimitedAt(from, now)
		}
		return
	}

	msg, complete, err := r.reasm.Add(frame, now)
	if err != nil || !complete {
		if err != nil && hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if msg.ID != frame.MsgID {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if !now.Before(msg.ExpiresAt()) {
		return
	}
	if r.validate != nil && !r.validate(msg) {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if r.replay.SeenOrAdd(msg.ID, now) {
		return
	}
	if r.cache.Has(msg.ID, now) {
		return
	}
	r.cache.Put(cache_manager.Entry{
		ID:        msg.ID,
		Sender:    msg.Sender,
		Timestamp: msg.Timestamp,
		TTLSec:    msg.TTLSec,
		Inserted:  now,
		ExpiresAt: msg.ExpiresAt(),
		Payload:   msg.Payload,
	}, now)

	if r.onMessage != nil {
		r.onMessage(msg)
	}

	r.maybeForward(msg, from, hasFrom, now, false)
}

func (r *Router) maybeForward(msg Message, from identity_manager.NodeID, hasFrom bool, now time.Time, rateLimited bool) {
	if msg.Hop >= r.maxHop {
		return
	}
	if last, ok := r.lastForward[msg.ID]; ok && now.Sub(last) < r.minRebroadcastEvery {
		return
	}

	prob := r.route.Probability(routing_engine.Input{
		From:        from,
		Hop:         msg.Hop,
		MaxHop:      r.maxHop,
		ExpiresAt:   msg.ExpiresAt(),
		Now:         now,
		HasFrom:     hasFrom,
		RateLimited: rateLimited,
	}, r.scorer)
	dec := r.gossip.Decide(prob, r.route.JitterMax())
	if !dec.Forward {
		return
	}

	r.lastForward[msg.ID] = now
	cp := msg
	cp.Hop = msg.Hop + 1
	time.AfterFunc(dec.Delay, func() {
		_ = r.broadcastMessage(cp, time.Now())
	})
}

func (r *Router) broadcastMessage(msg Message, now time.Time) error {
	if !r.guard.AllowGlobal(now, 2.0) {
		return nil
	}
	frames, err := ChunkMessage(msg, r.maxFrameBytes)
	if err != nil {
		return err
	}
	if !r.guard.AllowGlobal(now, 1.0*float64(len(frames))) {
		return nil
	}
	for i := range frames {
		b, err := EncodeChunkFrame(frames[i])
		if err != nil {
			continue
		}
		_ = r.transport.Broadcast(b)
	}
	return nil
}

func (r *Router) maybeBroadcastInventory(now time.Time) {
	if !r.lastInvAt.IsZero() && now.Sub(r.lastInvAt) < r.invEvery {
		return
	}
	if !r.guard.AllowGlobal(now, 0.75) {
		return
	}
	items := make([]partition_sync.InventoryItem, 0, r.invMax)
	entries := r.cache.List(now, r.invMax)
	for i := range entries {
		items = append(items, partition_sync.InventoryItem{
			ID:        entries[i].ID,
			ExpiresAt: entries[i].ExpiresAt,
		})
	}
	if len(items) == 0 {
		return
	}
	id := r.currentIdentity(now)
	b, err := partition_sync.EncodeInventory(id.NodeID, now, items, r.invMax)
	if err != nil {
		return
	}
	_ = r.transport.Broadcast(b)
	r.lastInvAt = now
}

func (r *Router) handleInventory(raw []byte, from identity_manager.NodeID, hasFrom bool, now time.Time) {
	if hasFrom {
		if !r.guard.Allow(from, now, 0.6) {
			r.scorer.OnRateLimitedAt(from, now)
			return
		}
	}
	inv, err := partition_sync.DecodeInventory(raw)
	if err != nil {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	if inv.Timestamp.After(now.Add(r.clockSkewAllowance)) {
		return
	}
	want := make([]partition_sync.MessageID, 0, r.wantMax)
	for i := range inv.Items {
		if len(want) >= r.wantMax {
			break
		}
		if !now.Before(inv.Items[i].ExpiresAt) {
			continue
		}
		if r.cache.Has(inv.Items[i].ID, now) {
			continue
		}
		if r.replay.Seen(inv.Items[i].ID, now) {
			continue
		}
		want = append(want, inv.Items[i].ID)
	}
	if len(want) == 0 {
		return
	}
	id := r.currentIdentity(now)
	b, err := partition_sync.EncodeWant(id.NodeID, want, r.wantMax)
	if err != nil {
		return
	}
	if !r.guard.AllowGlobal(now, 0.5) {
		return
	}
	_ = r.transport.Broadcast(b)
}

func (r *Router) handleWant(raw []byte, from identity_manager.NodeID, hasFrom bool, now time.Time) {
	if hasFrom {
		if !r.guard.Allow(from, now, 0.6) {
			r.scorer.OnRateLimitedAt(from, now)
			return
		}
	}
	w, err := partition_sync.DecodeWant(raw)
	if err != nil {
		if hasFrom {
			r.scorer.OnInvalidAt(from, now)
		}
		return
	}
	for i := range w.Wants {
		e, ok := r.cache.Get(w.Wants[i], now)
		if !ok {
			continue
		}
		cp := Message{
			Sender:    e.Sender,
			Timestamp: e.Timestamp,
			TTLSec:    e.TTLSec,
			Hop:       0,
			Payload:   e.Payload,
		}
		cp.ID = ComputeMessageID(cp.Sender, cp.Timestamp, cp.TTLSec, cp.Payload)
		if cp.ID != e.ID {
			continue
		}
		_ = r.broadcastMessage(cp, now)
	}
}

func (r *Router) broadcastCover(now time.Time) {
	id := r.currentIdentity(now)
	_ = id
	b, err := partition_sync.EncodeCover(now, r.coverSize)
	if err != nil {
		return
	}
	if !r.guard.AllowGlobal(now, 0.4) {
		return
	}
	_ = r.transport.Broadcast(b)
}
