package devsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SchemaV1 = 1

	MaxEvents = 50000
	MaxSeen   = 200000
)

type Event struct {
	ID        string          `json:"id"`
	DeviceID  string          `json:"device_id"`
	Seq       uint64          `json:"seq"`
	Lamport   uint64          `json:"lamport"`
	CreatedAt int64           `json:"created_at"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
}

type OutboxItem struct {
	EventID       string `json:"event_id"`
	RetryCount    int    `json:"retry_count"`
	NextAttemptAt int64  `json:"next_attempt_at"`
	ExpiresAt     int64  `json:"expires_at"`
}

type Batch struct {
	SchemaVersion int               `json:"schema_version"`
	DeviceID      string            `json:"device_id"`
	CreatedAt     int64             `json:"created_at"`
	Cursors       map[string]uint64 `json:"cursors,omitempty"`
	Events        []Event           `json:"events"`
}

type State struct {
	SchemaVersion int               `json:"schema_version"`
	UpdatedAt     int64             `json:"updated_at"`
	LocalDeviceID string            `json:"local_device_id,omitempty"`
	NextSeq       uint64            `json:"next_seq"`
	Lamport       uint64            `json:"lamport"`
	Cursors       map[string]uint64 `json:"cursors,omitempty"`
	Seen          map[string]int64  `json:"seen,omitempty"`
	Events        []Event           `json:"events,omitempty"`
	Outbox        []OutboxItem      `json:"outbox,omitempty"`
}

func NewState() *State {
	return &State{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Cursors:       map[string]uint64{},
		Seen:          map[string]int64{},
		Events:        []Event{},
		Outbox:        []OutboxItem{},
	}
}

func (s *State) Validate() error {
	if s == nil {
		return errors.New("devsync: state is nil")
	}
	if s.SchemaVersion != SchemaV1 {
		return fmt.Errorf("devsync: unsupported schema_version: %d", s.SchemaVersion)
	}
	if len(s.Events) > MaxEvents*2 {
		return fmt.Errorf("devsync: too many events: %d", len(s.Events))
	}
	if len(s.Seen) > MaxSeen*2 {
		return fmt.Errorf("devsync: too many seen ids: %d", len(s.Seen))
	}
	for i := range s.Events {
		e := s.Events[i]
		if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.DeviceID) == "" || e.Seq == 0 || e.CreatedAt <= 0 || strings.TrimSpace(e.Kind) == "" {
			return fmt.Errorf("devsync: invalid event at index %d", i)
		}
	}
	for i := range s.Outbox {
		if strings.TrimSpace(s.Outbox[i].EventID) == "" {
			return fmt.Errorf("devsync: outbox event_id required at index %d", i)
		}
	}
	return nil
}

func (s *State) Prune(now time.Time) {
	if s == nil {
		return
	}
	s.UpdatedAt = now.Unix()

	if len(s.Events) > 1 {
		sort.Slice(s.Events, func(i, j int) bool {
			if s.Events[i].Lamport == s.Events[j].Lamport {
				if s.Events[i].DeviceID == s.Events[j].DeviceID {
					return s.Events[i].Seq < s.Events[j].Seq
				}
				return s.Events[i].DeviceID < s.Events[j].DeviceID
			}
			return s.Events[i].Lamport < s.Events[j].Lamport
		})
	}
	if len(s.Events) > MaxEvents {
		s.Events = s.Events[len(s.Events)-MaxEvents:]
	}
	if len(s.Seen) > MaxSeen {
		type pair struct {
			id string
			ts int64
		}
		items := make([]pair, 0, len(s.Seen))
		for id, ts := range s.Seen {
			items = append(items, pair{id: id, ts: ts})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ts < items[j].ts })
		drop := len(items) - MaxSeen
		for i := 0; i < drop; i++ {
			delete(s.Seen, items[i].id)
		}
	}
}
