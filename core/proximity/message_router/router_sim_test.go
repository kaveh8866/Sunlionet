package message_router

import (
	"context"
	"crypto/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type simNet struct {
	mu    sync.RWMutex
	links map[*simTransport][]*simTransport
}

func newSimNet() *simNet {
	return &simNet{links: make(map[*simTransport][]*simTransport)}
}

func (n *simNet) connect(a, b *simTransport) {
	if a == nil || b == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if !containsTransport(n.links[a], b) {
		n.links[a] = append(n.links[a], b)
	}
	if !containsTransport(n.links[b], a) {
		n.links[b] = append(n.links[b], a)
	}
}

func containsTransport(xs []*simTransport, t *simTransport) bool {
	for _, x := range xs {
		if x == t {
			return true
		}
	}
	return false
}

// disconnect breaks the bidirectional link between a and b.
func (n *simNet) disconnect(a, b *simTransport) {
	if a == nil || b == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.links[a] = removeTransport(n.links[a], b)
	n.links[b] = removeTransport(n.links[b], a)
}

func removeTransport(xs []*simTransport, t *simTransport) []*simTransport {
	out := xs[:0]
	for _, x := range xs {
		if x != t {
			out = append(out, x)
		}
	}
	// Zero out the remaining elements to avoid memory leaks (GC can reclaim them)
	for i := len(out); i < len(xs); i++ {
		xs[i] = nil
	}
	return out
}

type simTransport struct {
	net   *simNet
	node  identity_manager.NodeID
	inbox chan simPacket
}

type simPacket struct {
	from identity_manager.NodeID
	raw  []byte
}

func newSimTransport(net *simNet, node identity_manager.NodeID) *simTransport {
	return &simTransport{
		net:   net,
		node:  node,
		inbox: make(chan simPacket, 4096),
	}
}

func (t *simTransport) Broadcast(data []byte) error {
	cp := append([]byte(nil), data...)
	t.net.mu.RLock()
	peers := append([]*simTransport(nil), t.net.links[t]...)
	t.net.mu.RUnlock()
	for _, p := range peers {
		select {
		case p.inbox <- simPacket{from: t.node, raw: cp}:
		case <-time.After(2 * time.Millisecond):
		}
	}
	return nil
}

func (t *simTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p := <-t.inbox:
		return p.raw, nil
	}
}

func (t *simTransport) ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error) {
	select {
	case <-ctx.Done():
		return nil, identity_manager.NodeID{}, ctx.Err()
	case p := <-t.inbox:
		return p.raw, p.from, nil
	}
}

type injectTransport struct {
	in chan simPacket
}

func (t *injectTransport) Broadcast(data []byte) error { return nil }

func (t *injectTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p := <-t.in:
		return p.raw, nil
	}
}

func (t *injectTransport) ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error) {
	select {
	case <-ctx.Done():
		return nil, identity_manager.NodeID{}, ctx.Err()
	case p := <-t.in:
		return p.raw, p.from, nil
	}
}

func TestSimMesh_ScenarioA_Normal10Nodes(t *testing.T) {
	net := newSimNet()
	nodes := make([]*simTransport, 10)
	ids := make([]*identity_manager.Manager, 10)
	for i := range nodes {
		m, _ := identity_manager.New(200 * time.Millisecond)
		ids[i] = m
		nodeID := m.Current(time.Now()).NodeID
		nodes[i] = newSimTransport(net, nodeID)
	}

	for i := 0; i < 9; i++ {
		net.connect(nodes[i], nodes[i+1])
	}
	net.connect(nodes[2], nodes[7])
	net.connect(nodes[1], nodes[5])

	opts := Options{
		MaxFrameBytes:        120,
		MaxHop:               10,
		MinRebroadcastEvery:  5 * time.Millisecond,
		ReassemblyStaleAfter: 2 * time.Second,
		CacheMaxItems:        1024,
		ClockSkewAllowance:   5 * time.Second,
		GlobalRateCapacity:   200,
		GlobalRate:           120,
		PerSenderCapacity:    50,
		PerSenderRate:        40,
		GossipSeed:           123,
		RoutingBaseProb:      0.95,
		RoutingMaxJitter:     2 * time.Millisecond,
		InventoryEvery:       50 * time.Millisecond,
		InventoryMax:         16,
		WantMax:              12,
	}

	routers := make([]*Router, 10)
	for i := range routers {
		r, _ := NewRouter(nodes[i], ids[i].Current, opts)
		routers[i] = r
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var gotMu sync.Mutex
	got := 0
	routers[9].SetOnMessage(func(m Message) {
		gotMu.Lock()
		got++
		gotMu.Unlock()
	})

	for i := range routers {
		go func(r *Router) { _ = r.Run(ctx) }(routers[i])
	}

	now := time.Now()
	_, err := routers[0].Send(now, []byte("hello"), 2)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	<-ctx.Done()
	gotMu.Lock()
	defer gotMu.Unlock()
	if got == 0 {
		t.Fatalf("expected message to reach node 9")
	}
}

func TestSimMesh_ScenarioB_SpamNodes(t *testing.T) {
	net := newSimNet()
	nodes := make([]*simTransport, 10)
	ids := make([]*identity_manager.Manager, 10)
	for i := range nodes {
		m, _ := identity_manager.New(150 * time.Millisecond)
		ids[i] = m
		nodeID := m.Current(time.Now()).NodeID
		nodes[i] = newSimTransport(net, nodeID)
	}

	for i := 0; i < 9; i++ {
		net.connect(nodes[i], nodes[i+1])
	}
	net.connect(nodes[3], nodes[8])
	net.connect(nodes[1], nodes[3])
	net.connect(nodes[5], nodes[7])

	opts := Options{
		MaxFrameBytes:        110,
		MaxHop:               10,
		MinRebroadcastEvery:  5 * time.Millisecond,
		ReassemblyStaleAfter: 2 * time.Second,
		CacheMaxItems:        1024,
		ClockSkewAllowance:   5 * time.Second,
		GlobalRateCapacity:   80,
		GlobalRate:           40,
		PerSenderCapacity:    8,
		PerSenderRate:        4,
		GossipSeed:           7,
		RoutingBaseProb:      1.0,
		RoutingMaxJitter:     2 * time.Millisecond,
		InventoryEvery:       70 * time.Millisecond,
		InventoryMax:         12,
		WantMax:              10,
	}

	routers := make([]*Router, 10)
	for i := range routers {
		r, _ := NewRouter(nodes[i], ids[i].Current, opts)
		routers[i] = r
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var got1 atomic.Int64
	var got3 atomic.Int64
	var got5 atomic.Int64
	var got8 atomic.Int64
	var got9 atomic.Int64
	routers[9].SetOnMessage(func(m Message) {
		if string(m.Payload) == "target" {
			got9.Add(1)
		}
	})
	routers[5].SetOnMessage(func(m Message) {
		if string(m.Payload) == "target" {
			got5.Add(1)
		}
	})
	routers[1].SetOnMessage(func(m Message) {
		if string(m.Payload) == "target" {
			got1.Add(1)
		}
	})
	routers[3].SetOnMessage(func(m Message) {
		if string(m.Payload) == "target" {
			got3.Add(1)
		}
	})
	routers[8].SetOnMessage(func(m Message) {
		if string(m.Payload) == "target" {
			got8.Add(1)
		}
	})

	for i := range routers {
		go func(r *Router) { _ = r.Run(ctx) }(routers[i])
	}

	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				buf := make([]byte, 32)
				_, _ = rand.Read(buf)
				_, _ = routers[2].Send(time.Now(), buf, 2)
				_, _ = routers[6].Send(time.Now(), buf, 2)
			}
		}
	}()

	time.Sleep(120 * time.Millisecond)
	_, err := routers[0].Send(time.Now(), []byte("target"), 4)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	<-ctx.Done()
	if got9.Load() == 0 {
		t.Fatalf("expected target message to reach node 9 despite spam (seen: n1=%d n3=%d n5=%d n8=%d n9=%d)", got1.Load(), got3.Load(), got5.Load(), got8.Load(), got9.Load())
	}
}

func TestSimMesh_ScenarioC_PartitionSplitMerge(t *testing.T) {
	net := newSimNet()
	nodes := make([]*simTransport, 10)
	ids := make([]*identity_manager.Manager, 10)
	for i := range nodes {
		m, _ := identity_manager.New(200 * time.Millisecond)
		ids[i] = m
		nodeID := m.Current(time.Now()).NodeID
		nodes[i] = newSimTransport(net, nodeID)
	}

	for i := 0; i < 4; i++ {
		net.connect(nodes[i], nodes[i+1])
	}
	for i := 5; i < 9; i++ {
		net.connect(nodes[i], nodes[i+1])
	}

	opts := Options{
		MaxFrameBytes:        120,
		MaxHop:               12,
		MinRebroadcastEvery:  5 * time.Millisecond,
		ReassemblyStaleAfter: 2 * time.Second,
		CacheMaxItems:        2048,
		ClockSkewAllowance:   5 * time.Second,
		GlobalRateCapacity:   200,
		GlobalRate:           120,
		PerSenderCapacity:    40,
		PerSenderRate:        25,
		GossipSeed:           99,
		RoutingBaseProb:      0.9,
		RoutingMaxJitter:     2 * time.Millisecond,
		InventoryEvery:       60 * time.Millisecond,
		InventoryMax:         18,
		WantMax:              12,
	}

	routers := make([]*Router, 10)
	for i := range routers {
		r, _ := NewRouter(nodes[i], ids[i].Current, opts)
		routers[i] = r
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	var gotMu sync.Mutex
	got := 0
	routers[9].SetOnMessage(func(m Message) {
		if string(m.Payload) == "partitioned" {
			gotMu.Lock()
			got++
			gotMu.Unlock()
		}
	})

	for i := range routers {
		go func(r *Router) { _ = r.Run(ctx) }(routers[i])
	}

	_, err := routers[0].Send(time.Now(), []byte("partitioned"), 3)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	time.Sleep(400 * time.Millisecond)
	net.connect(nodes[4], nodes[5])

	<-ctx.Done()
	gotMu.Lock()
	defer gotMu.Unlock()
	if got == 0 {
		t.Fatalf("expected message to reach node 9 after partition merge")
	}
}

func TestSimMesh_ScenarioD_ReplayAttack(t *testing.T) {
	m2, _ := identity_manager.New(200 * time.Millisecond)
	tr := &injectTransport{in: make(chan simPacket, 16)}

	opts := Options{
		MaxFrameBytes:        120,
		MaxHop:               10,
		MinRebroadcastEvery:  5 * time.Millisecond,
		ReassemblyStaleAfter: 2 * time.Second,
		CacheMaxItems:        1024,
		ClockSkewAllowance:   5 * time.Second,
		GlobalRateCapacity:   200,
		GlobalRate:           120,
		PerSenderCapacity:    30,
		PerSenderRate:        20,
		GossipSeed:           5,
		RoutingBaseProb:      1.0,
		RoutingMaxJitter:     1 * time.Millisecond,
		ReplayBucketDuration: 20 * time.Millisecond,
		ReplayWindowDuration: 600 * time.Millisecond,
	}

	r2, _ := NewRouter(tr, m2.Current, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()

	var gotMu sync.Mutex
	got := 0
	r2.SetOnMessage(func(m Message) {
		if string(m.Payload) == "replay" {
			gotMu.Lock()
			got++
			gotMu.Unlock()
		}
	})

	go func() { _ = r2.Run(ctx) }()

	now := time.Now()
	var sender identity_manager.NodeID
	_, _ = rand.Read(sender[:])
	msg, err := NewMessage(sender, now, 2, []byte("replay"))
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	frames, err := ChunkMessage(msg, 220)
	if err != nil || len(frames) != 1 {
		t.Fatalf("chunk: %v frames=%d", err, len(frames))
	}
	raw, err := EncodeChunkFrame(frames[0])
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var attacker identity_manager.NodeID
	_, _ = rand.Read(attacker[:])
	tr.in <- simPacket{from: attacker, raw: raw}
	tr.in <- simPacket{from: attacker, raw: raw}

	<-ctx.Done()
	gotMu.Lock()
	defer gotMu.Unlock()
	if got != 1 {
		t.Fatalf("expected exactly one delivery under replay attack, got %d", got)
	}
}

func TestSimNet_ConnectDisconnect(t *testing.T) {
	net := newSimNet()
	id1 := identity_manager.NodeID{1}
	id2 := identity_manager.NodeID{2}
	t1 := newSimTransport(net, id1)
	t2 := newSimTransport(net, id2)

	// 1. Test connect
	net.connect(t1, t2)
	if !containsTransport(net.links[t1], t2) || !containsTransport(net.links[t2], t1) {
		t.Errorf("expected t1 and t2 to be connected")
	}

	// 2. Test idempotency
	net.connect(t1, t2)
	if len(net.links[t1]) != 1 || len(net.links[t2]) != 1 {
		t.Errorf("expected connect to be idempotent, got lengths %d and %d", len(net.links[t1]), len(net.links[t2]))
	}

	// 3. Test disconnect
	net.disconnect(t1, t2)
	if containsTransport(net.links[t1], t2) || containsTransport(net.links[t2], t1) {
		t.Errorf("expected t1 and t2 to be disconnected")
	}
	if len(net.links[t1]) != 0 || len(net.links[t2]) != 0 {
		t.Errorf("expected links to be empty after disconnect")
	}

	// 4. Test disconnect non-existent
	net.disconnect(t1, t2) // Should not panic or cause issues

	// 5. Test nil safety
	net.connect(nil, t1)
	net.connect(t1, nil)
	net.disconnect(nil, t1)
	net.disconnect(t1, nil)
}
