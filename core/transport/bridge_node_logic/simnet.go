package bridge_node_logic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type Medium string

const (
	MediumBLE  Medium = "ble"
	MediumWiFi Medium = "wifi_direct"
)

type packet struct {
	from identity_manager.NodeID
	raw  []byte
}

type SimNet struct {
	mu      sync.RWMutex
	links   map[Medium]map[identity_manager.NodeID]map[identity_manager.NodeID]struct{}
	inboxes map[Medium]map[identity_manager.NodeID]chan packet
}

func NewSimNet() *SimNet {
	return &SimNet{
		links:   make(map[Medium]map[identity_manager.NodeID]map[identity_manager.NodeID]struct{}),
		inboxes: make(map[Medium]map[identity_manager.NodeID]chan packet),
	}
}

func (n *SimNet) Register(m Medium, node identity_manager.NodeID, buf int) *Endpoint {
	if buf <= 0 {
		buf = 2048
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.links[m] == nil {
		n.links[m] = make(map[identity_manager.NodeID]map[identity_manager.NodeID]struct{})
	}
	if n.inboxes[m] == nil {
		n.inboxes[m] = make(map[identity_manager.NodeID]chan packet)
	}
	if n.inboxes[m][node] == nil {
		n.inboxes[m][node] = make(chan packet, buf)
	}
	if n.links[m][node] == nil {
		n.links[m][node] = make(map[identity_manager.NodeID]struct{})
	}
	return &Endpoint{net: n, medium: m, node: node, inbox: n.inboxes[m][node]}
}

func (n *SimNet) Connect(m Medium, a, b identity_manager.NodeID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.links[m] == nil {
		n.links[m] = make(map[identity_manager.NodeID]map[identity_manager.NodeID]struct{})
	}
	if n.links[m][a] == nil {
		n.links[m][a] = make(map[identity_manager.NodeID]struct{})
	}
	if n.links[m][b] == nil {
		n.links[m][b] = make(map[identity_manager.NodeID]struct{})
	}
	n.links[m][a][b] = struct{}{}
	n.links[m][b][a] = struct{}{}
}

func (n *SimNet) Disconnect(m Medium, a, b identity_manager.NodeID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.links[m] == nil {
		return
	}
	if n.links[m][a] != nil {
		delete(n.links[m][a], b)
	}
	if n.links[m][b] != nil {
		delete(n.links[m][b], a)
	}
}

type Endpoint struct {
	net    *SimNet
	medium Medium
	node   identity_manager.NodeID
	inbox  chan packet

	available atomic.Bool
	txCount   atomic.Int64
}

func (e *Endpoint) Name() string { return string(e.medium) }

func (e *Endpoint) Available() bool { return e.available.Load() }

func (e *Endpoint) SetAvailable(v bool) { e.available.Store(v) }

func (e *Endpoint) Broadcast(data []byte) error {
	if !e.available.Load() {
		return errors.New("transport unavailable")
	}
	cp := append([]byte(nil), data...)
	e.txCount.Add(1)

	e.net.mu.RLock()
	neighbors := e.net.links[e.medium][e.node]
	inboxes := e.net.inboxes[e.medium]
	e.net.mu.RUnlock()

	for peer := range neighbors {
		ch := inboxes[peer]
		if ch == nil {
			continue
		}
		select {
		case ch <- packet{from: e.node, raw: cp}:
		case <-time.After(2 * time.Millisecond):
		}
	}
	return nil
}

func (e *Endpoint) ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error) {
	select {
	case <-ctx.Done():
		return nil, identity_manager.NodeID{}, ctx.Err()
	case p := <-e.inbox:
		return p.raw, p.from, nil
	}
}

func (e *Endpoint) LinkScore(_ identity_manager.NodeID, _ time.Time) float64 {
	if !e.available.Load() {
		return 0
	}
	if e.medium == MediumWiFi {
		return 0.9
	}
	return 0.5
}

func (e *Endpoint) TxCount() int64 { return e.txCount.Load() }
