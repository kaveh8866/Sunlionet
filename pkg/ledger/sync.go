package ledger

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
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
	sort.Strings(missing)
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
	sort.Strings(heads)
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

	out := make([]Event, 0, min(maxEvents, len(want)))
	bytesUsed := 0
	for _, id := range want {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
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
	rep := ApplyReport{}
	for i := range msg.Events {
		ev := msg.Events[i]
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

	rep := ApplyReport{}
	bytesUsed := 0
	for i := range msg.Events {
		ev := msg.Events[i]
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
