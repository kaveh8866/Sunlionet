package ledgersync

import (
	"encoding/base64"
	"errors"
	"sync"

	"github.com/kaveh/sunlionet-agent/pkg/ledger"
)

type HeadsMessage struct {
	Heads [][]byte `json:"heads"`
}

type InventoryMessage struct {
	Heads [][]byte `json:"heads"`
	Have  [][]byte `json:"have,omitempty"`
}

type WantMessage struct {
	Want [][]byte `json:"want"`
}

type EventsMessage struct {
	Events []*ledger.Event `json:"events"`
}

type Sync struct {
	mu          sync.Mutex
	ledger      *ledger.Ledger
	pendingWant map[string]struct{}
}

func NewSync(l *ledger.Ledger) *Sync {
	return &Sync{
		ledger:      l,
		pendingWant: map[string]struct{}{},
	}
}

func (s *Sync) BuildHeadsMessage() HeadsMessage {
	if s == nil || s.ledger == nil {
		return HeadsMessage{}
	}
	h := s.ledger.BuildHeadsMessage()
	return HeadsMessage{Heads: decodeIDs(h.Heads)}
}

func (s *Sync) BuildInventoryMessage(maxHave int) InventoryMessage {
	if s == nil || s.ledger == nil {
		return InventoryMessage{}
	}
	inv := s.ledger.BuildInventoryMessage(maxHave)
	return InventoryMessage{
		Heads: decodeIDs(inv.Heads),
		Have:  decodeIDs(inv.Have),
	}
}

func (s *Sync) PlanSyncFromPeer(peerInv InventoryMessage, maxWant int) WantMessage {
	if s == nil || s.ledger == nil {
		return WantMessage{}
	}
	if maxWant <= 0 {
		maxWant = 256
	}

	peerHeads := encodeIDs(peerInv.Heads)
	peerHave := encodeIDs(peerInv.Have)

	peerHas := make(map[string]struct{}, len(peerHeads)+len(peerHave))
	for _, id := range peerHeads {
		if id == "" {
			continue
		}
		peerHas[id] = struct{}{}
	}
	for _, id := range peerHave {
		if id == "" {
			continue
		}
		peerHas[id] = struct{}{}
	}

	want := make([][]byte, 0, maxWant)
	wantSet := make(map[string]struct{})

	addWant := func(id string) bool {
		if id == "" {
			return false
		}
		if len(want) >= maxWant {
			return false
		}
		if s.ledger.Have(id) {
			return false
		}
		if _, ok := peerHas[id]; !ok {
			return false
		}
		if _, ok := wantSet[id]; ok {
			return false
		}
		wantSet[id] = struct{}{}
		raw, err := base64.RawURLEncoding.DecodeString(id)
		if err != nil {
			return false
		}
		want = append(want, raw)
		return true
	}

	for _, id := range peerHeads {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}

	missing := s.ledger.MissingRefs()
	for id := range missing {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}

	s.mu.Lock()
	pending := make([]string, 0, len(s.pendingWant))
	for id := range s.pendingWant {
		pending = append(pending, id)
	}
	s.mu.Unlock()
	for _, id := range pending {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}

	for _, id := range peerHave {
		if len(want) >= maxWant {
			break
		}
		addWant(id)
	}
	return WantMessage{Want: want}
}

func (s *Sync) BuildEventsMessage(w WantMessage, maxEvents int) EventsMessage {
	if s == nil || s.ledger == nil {
		return EventsMessage{}
	}
	if maxEvents <= 0 {
		maxEvents = 256
	}
	ids := encodeIDs(w.Want)
	msg := s.ledger.BuildEventsMessage(ids, maxEvents)
	out := make([]*ledger.Event, 0, len(msg.Events))
	for i := range msg.Events {
		ev := msg.Events[i]
		out = append(out, &ev)
	}
	return EventsMessage{Events: out}
}

func (s *Sync) ApplyHeadsMessage(msg HeadsMessage) error {
	if s == nil || s.ledger == nil {
		return errors.New("ledgersync: sync is nil")
	}
	heads := encodeIDs(msg.Heads)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range heads {
		if id == "" {
			continue
		}
		if s.ledger.Have(id) {
			continue
		}
		s.pendingWant[id] = struct{}{}
	}
	return nil
}

func (s *Sync) ApplyEventsMessage(msg EventsMessage) (ledger.ApplyReport, error) {
	if s == nil || s.ledger == nil {
		return ledger.ApplyReport{}, errors.New("ledgersync: sync is nil")
	}
	rep := ledger.ApplyReport{}
	for _, evp := range msg.Events {
		if evp == nil {
			rep.Rejected++
			continue
		}
		ev := *evp
		if s.ledger.Have(ev.ID) {
			rep.Dupe++
			s.mu.Lock()
			delete(s.pendingWant, ev.ID)
			s.mu.Unlock()
			continue
		}
		if _, err := s.ledger.ApplyWithContext(ev, "", nil, nil); err != nil {
			rep.Rejected++
			continue
		}
		rep.Applied++
		s.mu.Lock()
		delete(s.pendingWant, ev.ID)
		s.mu.Unlock()
	}
	return rep, nil
}

func decodeIDs(ids []string) [][]byte {
	if len(ids) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(ids))
	for _, id := range ids {
		raw, err := base64.RawURLEncoding.DecodeString(id)
		if err != nil {
			continue
		}
		out = append(out, raw)
	}
	return out
}

func encodeIDs(ids [][]byte) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if len(id) == 0 {
			continue
		}
		out = append(out, base64.RawURLEncoding.EncodeToString(id))
	}
	return out
}
