package devsync

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultOutboxTTL = 7 * 24 * time.Hour
	maxBackoff       = 5 * time.Minute
)

type Service struct {
	store *Store
	mu    sync.Mutex
	state *State
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("devsync: store is nil")
	}
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, state: st}, nil
}

func (s *Service) SetLocalDeviceID(deviceID string) error {
	if s == nil {
		return errors.New("devsync: service is nil")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("devsync: device_id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LocalDeviceID = deviceID
	if s.state.Cursors == nil {
		s.state.Cursors = map[string]uint64{}
	}
	if _, ok := s.state.Cursors[deviceID]; !ok {
		s.state.Cursors[deviceID] = 0
	}
	s.state.UpdatedAt = time.Now().Unix()
	return s.store.Save(s.state)
}

func (s *Service) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *s.state
	cp.Events = append([]Event(nil), s.state.Events...)
	cp.Outbox = append([]OutboxItem(nil), s.state.Outbox...)
	cp.Cursors = make(map[string]uint64, len(s.state.Cursors))
	for k, v := range s.state.Cursors {
		cp.Cursors[k] = v
	}
	cp.Seen = make(map[string]int64, len(s.state.Seen))
	for k, v := range s.state.Seen {
		cp.Seen[k] = v
	}
	return cp
}

func (s *Service) Record(kind string, payload json.RawMessage, expiry time.Duration) (Event, error) {
	if s == nil {
		return Event{}, errors.New("devsync: service is nil")
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return Event{}, errors.New("devsync: event kind required")
	}
	if len(payload) == 0 {
		return Event{}, errors.New("devsync: event payload required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.state.LocalDeviceID) == "" {
		return Event{}, errors.New("devsync: local device id not set")
	}
	now := time.Now()
	s.state.NextSeq++
	if s.state.Lamport < s.state.NextSeq {
		s.state.Lamport = s.state.NextSeq
	}
	s.state.Lamport++
	ev := Event{
		DeviceID:  s.state.LocalDeviceID,
		Seq:       s.state.NextSeq,
		Lamport:   s.state.Lamport,
		CreatedAt: now.Unix(),
		Kind:      kind,
		Payload:   append(json.RawMessage(nil), payload...),
	}
	ev.ID = eventID(ev)
	if s.state.Cursors == nil {
		s.state.Cursors = map[string]uint64{}
	}
	if s.state.Seen == nil {
		s.state.Seen = map[string]int64{}
	}
	s.state.Events = append(s.state.Events, ev)
	s.state.Cursors[ev.DeviceID] = ev.Seq
	s.state.Seen[ev.ID] = now.Unix()
	exp := now.Add(defaultOutboxTTL).Unix()
	if expiry > 0 {
		exp = now.Add(expiry).Unix()
	}
	s.state.Outbox = append(s.state.Outbox, OutboxItem{
		EventID:       ev.ID,
		RetryCount:    0,
		NextAttemptAt: now.Unix(),
		ExpiresAt:     exp,
	})
	s.state.UpdatedAt = now.Unix()
	return ev, s.store.Save(s.state)
}

func (s *Service) BuildBatch(peerCursors map[string]uint64, limit int) Batch {
	if s == nil {
		return Batch{SchemaVersion: SchemaV1}
	}
	if limit <= 0 {
		limit = 256
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if peerCursors == nil {
		peerCursors = map[string]uint64{}
	}
	out := make([]Event, 0, min(limit, 64))
	for i := range s.state.Events {
		ev := s.state.Events[i]
		if ev.Seq <= peerCursors[ev.DeviceID] {
			continue
		}
		out = append(out, ev)
		if len(out) >= limit {
			break
		}
	}
	cursorCopy := make(map[string]uint64, len(s.state.Cursors))
	for k, v := range s.state.Cursors {
		cursorCopy[k] = v
	}
	return Batch{
		SchemaVersion: SchemaV1,
		DeviceID:      s.state.LocalDeviceID,
		CreatedAt:     time.Now().Unix(),
		Cursors:       cursorCopy,
		Events:        out,
	}
}

func (s *Service) ApplyBatch(b Batch) (int, error) {
	if s == nil {
		return 0, errors.New("devsync: service is nil")
	}
	if b.SchemaVersion != SchemaV1 {
		return 0, fmt.Errorf("devsync: unsupported batch schema: %d", b.SchemaVersion)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Cursors == nil {
		s.state.Cursors = map[string]uint64{}
	}
	if s.state.Seen == nil {
		s.state.Seen = map[string]int64{}
	}
	now := time.Now().Unix()
	applied := 0
	for i := range b.Events {
		ev := b.Events[i]
		if strings.TrimSpace(ev.ID) == "" {
			ev.ID = eventID(ev)
		}
		if _, seen := s.state.Seen[ev.ID]; seen {
			continue
		}
		if ev.Seq <= s.state.Cursors[ev.DeviceID] {
			s.state.Seen[ev.ID] = now
			continue
		}
		if s.state.Lamport < ev.Lamport {
			s.state.Lamport = ev.Lamport
		}
		s.state.Lamport++
		s.state.Events = append(s.state.Events, ev)
		s.state.Cursors[ev.DeviceID] = ev.Seq
		s.state.Seen[ev.ID] = now
		applied++
	}
	s.state.UpdatedAt = now
	return applied, s.store.Save(s.state)
}

func (s *Service) NextOutbox(now time.Time) (Event, bool) {
	if s == nil {
		return Event{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unixNow := now.Unix()
	for i := range s.state.Outbox {
		it := s.state.Outbox[i]
		if it.ExpiresAt > 0 && unixNow >= it.ExpiresAt {
			continue
		}
		if unixNow < it.NextAttemptAt {
			continue
		}
		for j := range s.state.Events {
			if s.state.Events[j].ID == it.EventID {
				return s.state.Events[j], true
			}
		}
	}
	return Event{}, false
}

func (s *Service) AckEvent(eventID string) error {
	if s == nil {
		return errors.New("devsync: service is nil")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return errors.New("devsync: event id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.state.Outbox[:0]
	removed := false
	for i := range s.state.Outbox {
		if s.state.Outbox[i].EventID == eventID {
			removed = true
			continue
		}
		kept = append(kept, s.state.Outbox[i])
	}
	s.state.Outbox = kept
	if !removed {
		return nil
	}
	s.state.UpdatedAt = time.Now().Unix()
	return s.store.Save(s.state)
}

func (s *Service) MarkEventRetry(eventID string, now time.Time) error {
	if s == nil {
		return errors.New("devsync: service is nil")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return errors.New("devsync: event id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Outbox {
		if s.state.Outbox[i].EventID != eventID {
			continue
		}
		s.state.Outbox[i].RetryCount++
		backoff := time.Second * time.Duration(1<<min(s.state.Outbox[i].RetryCount, 8))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		s.state.Outbox[i].NextAttemptAt = now.Add(backoff).Unix()
		s.state.UpdatedAt = now.Unix()
		return s.store.Save(s.state)
	}
	return errors.New("devsync: outbox event not found")
}

func eventID(ev Event) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s|%s", ev.DeviceID, ev.Seq, ev.CreatedAt, ev.Kind, string(ev.Payload))))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
