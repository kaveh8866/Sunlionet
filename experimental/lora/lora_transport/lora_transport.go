package lora_transport

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type packet struct {
	from identity_manager.NodeID
	raw  []byte
}

type SimNet struct {
	mu    sync.RWMutex
	links map[identity_manager.NodeID]map[identity_manager.NodeID]struct{}
	inbox map[identity_manager.NodeID]chan packet
}

func NewSimNet() *SimNet {
	return &SimNet{
		links: make(map[identity_manager.NodeID]map[identity_manager.NodeID]struct{}),
		inbox: make(map[identity_manager.NodeID]chan packet),
	}
}

func (n *SimNet) Register(node identity_manager.NodeID, buf int) *Endpoint {
	if buf <= 0 {
		buf = 512
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.inbox[node] == nil {
		n.inbox[node] = make(chan packet, buf)
	}
	if n.links[node] == nil {
		n.links[node] = make(map[identity_manager.NodeID]struct{})
	}
	return &Endpoint{
		net: nodeNet{net: n},
		id:  node,
		in:  n.inbox[node],
	}
}

func (n *SimNet) Connect(a, b identity_manager.NodeID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.links[a] == nil {
		n.links[a] = make(map[identity_manager.NodeID]struct{})
	}
	if n.links[b] == nil {
		n.links[b] = make(map[identity_manager.NodeID]struct{})
	}
	n.links[a][b] = struct{}{}
	n.links[b][a] = struct{}{}
}

func (n *SimNet) Disconnect(a, b identity_manager.NodeID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.links[a] != nil {
		delete(n.links[a], b)
	}
	if n.links[b] != nil {
		delete(n.links[b], a)
	}
}

type nodeNet struct {
	net *SimNet
}

type Endpoint struct {
	net nodeNet
	id  identity_manager.NodeID
	in  chan packet

	available     atomic.Bool
	txCount       atomic.Int64
	maxFrameBytes int
	bytesPerSec   int
}

func (e *Endpoint) Name() string { return "lora" }

func (e *Endpoint) Available() bool { return e.available.Load() }

func (e *Endpoint) SetAvailable(v bool) { e.available.Store(v) }

func (e *Endpoint) Configure(maxFrameBytes int, bytesPerSec int) {
	if maxFrameBytes <= 0 {
		maxFrameBytes = 96
	}
	if bytesPerSec <= 0 {
		bytesPerSec = 32
	}
	e.maxFrameBytes = maxFrameBytes
	e.bytesPerSec = bytesPerSec
}

func (e *Endpoint) Broadcast(data []byte) error {
	if !e.available.Load() {
		return errors.New("transport unavailable")
	}
	if e.maxFrameBytes > 0 && len(data) > e.maxFrameBytes {
		return errors.New("frame too large")
	}

	e.txCount.Add(1)
	cp := append([]byte(nil), data...)

	e.net.net.mu.RLock()
	neighbors := e.net.net.links[e.id]
	inboxes := e.net.net.inbox
	e.net.net.mu.RUnlock()

	delay := time.Duration(0)
	if e.bytesPerSec > 0 {
		delay = time.Duration(float64(len(cp))/float64(e.bytesPerSec)*float64(time.Second)) + 15*time.Millisecond
	}

	for peer := range neighbors {
		ch := inboxes[peer]
		if ch == nil {
			continue
		}
		p := packet{from: e.id, raw: cp}
		go func(dst chan packet) {
			if delay > 0 {
				time.Sleep(delay)
			}
			select {
			case dst <- p:
			default:
			}
		}(ch)
	}
	return nil
}

func (e *Endpoint) ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error) {
	select {
	case <-ctx.Done():
		return nil, identity_manager.NodeID{}, ctx.Err()
	case p := <-e.in:
		return p.raw, p.from, nil
	}
}

func (e *Endpoint) LinkScore(_ identity_manager.NodeID, _ time.Time) float64 {
	if !e.available.Load() {
		return 0
	}
	return 0.2
}

func (e *Endpoint) TxCount() int64 { return e.txCount.Load() }
