package transport_manager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
)

type Transport interface {
	transport_selector.Candidate

	Broadcast(data []byte) error
	ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error)
	LinkScore(peer identity_manager.NodeID, now time.Time) float64
}

type FusedTransport struct {
	selector  transport_selector.Selector
	multipath atomic.Bool

	mu         sync.RWMutex
	transports []Transport
	preference transport_selector.Preference

	started atomic.Bool
	in      chan rx
	errCh   chan error
}

type Options struct {
	Profile     transport_selector.Profile
	Multipath   bool
	Transports  []Transport
	BufferBytes int
}

func NewFused(opts Options) (*FusedTransport, error) {
	if len(opts.Transports) == 0 {
		return nil, errors.New("at least one transport required")
	}
	if opts.BufferBytes <= 0 {
		opts.BufferBytes = 64
	}
	ft := &FusedTransport{
		selector: transport_selector.Selector{Profile: opts.Profile},
		in:       make(chan rx, opts.BufferBytes),
		errCh:    make(chan error, 1),
	}
	ft.transports = append([]Transport(nil), opts.Transports...)
	ft.multipath.Store(opts.Multipath)
	ft.preference = transport_selector.PreferenceAny
	return ft, nil
}

type rx struct {
	raw  []byte
	from identity_manager.NodeID
}

func (f *FusedTransport) Name() string { return "fused" }

func (f *FusedTransport) Available() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, t := range f.transports {
		if t != nil && t.Available() {
			return true
		}
	}
	return false
}

func (f *FusedTransport) WithPreference(pref transport_selector.Preference, fn func()) {
	prev := f.preference
	f.mu.Lock()
	f.preference = pref
	f.mu.Unlock()
	fn()
	f.mu.Lock()
	f.preference = prev
	f.mu.Unlock()
}

func (f *FusedTransport) SetProfile(profile transport_selector.Profile) {
	f.mu.Lock()
	f.selector.Profile = profile
	f.mu.Unlock()
}

func (f *FusedTransport) SetMultipath(enabled bool) {
	f.multipath.Store(enabled)
}

func (f *FusedTransport) Broadcast(data []byte) error {
	f.mu.RLock()
	transports := append([]Transport(nil), f.transports...)
	pref := f.preference
	sel := f.selector
	f.mu.RUnlock()

	decision := sel.Decide(toCandidates(transports), pref, len(data), f.multipath.Load())
	if len(decision.Names) == 0 {
		return nil
	}

	var firstErr error
	for _, name := range decision.Names {
		for _, t := range transports {
			if t == nil || !t.Available() || t.Name() != name {
				continue
			}
			if err := t.Broadcast(data); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (f *FusedTransport) Receive(ctx context.Context) ([]byte, error) {
	b, _, err := f.ReceiveFrom(ctx)
	return b, err
}

func (f *FusedTransport) ReceiveFrom(ctx context.Context) ([]byte, identity_manager.NodeID, error) {
	f.startFanIn(ctx)
	select {
	case <-ctx.Done():
		return nil, identity_manager.NodeID{}, ctx.Err()
	case err := <-f.errCh:
		return nil, identity_manager.NodeID{}, err
	case p := <-f.in:
		return p.raw, p.from, nil
	}
}

func (f *FusedTransport) startFanIn(ctx context.Context) {
	if !f.started.CompareAndSwap(false, true) {
		return
	}
	f.mu.RLock()
	transports := append([]Transport(nil), f.transports...)
	f.mu.RUnlock()

	for _, t := range transports {
		if t == nil {
			continue
		}
		go func(tt Transport) {
			for {
				raw, from, err := tt.ReceiveFrom(ctx)
				if err != nil {
					select {
					case f.errCh <- err:
					default:
					}
					return
				}
				select {
				case f.in <- rx{raw: raw, from: from}:
				case <-ctx.Done():
					return
				}
			}
		}(t)
	}
}

func (f *FusedTransport) LinkScore(peer identity_manager.NodeID, now time.Time) float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	best := 0.0
	for _, t := range f.transports {
		if t == nil || !t.Available() {
			continue
		}
		if s := t.LinkScore(peer, now); s > best {
			best = s
		}
	}
	return best
}

func toCandidates(ts []Transport) []transport_selector.Candidate {
	out := make([]transport_selector.Candidate, 0, len(ts))
	for i := range ts {
		out = append(out, ts[i])
	}
	return out
}
