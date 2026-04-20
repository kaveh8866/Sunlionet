package ledger

import (
	"encoding/base64"
	"errors"
	"strings"
)

var ErrDuplicateEvent = errors.New("ledger: duplicate event")

// AddEvent is the minimal append-only API expected by Phase 6 Prompt 1.
func (l *Ledger) AddEvent(e *Event) error {
	if l == nil {
		return errors.New("ledger: ledger is nil")
	}
	if e == nil {
		return errors.New("ledger: event is nil")
	}
	if e.Seq > 1 {
		if strings.TrimSpace(e.Prev) == "" {
			return errors.New("ledger: prev required for seq>1")
		}
		if _, ok := l.Get(e.Prev); !ok {
			return errors.New("ledger: prev not found")
		}
	}
	if l.Have(e.ID) {
		return ErrDuplicateEvent
	}
	return l.Add(*e)
}

// GetEvent resolves event IDs from either raw hash bytes or base64url ID bytes.
func (l *Ledger) GetEvent(id []byte) (*Event, bool) {
	if l == nil {
		return nil, false
	}
	evID, ok := normalizeEventIDInput(id)
	if !ok {
		return nil, false
	}
	ev, found := l.Get(evID)
	if !found {
		return nil, false
	}
	out := ev
	return &out, true
}

// GetHeads returns current DAG heads as raw hash bytes.
func (l *Ledger) GetHeads() [][]byte {
	if l == nil {
		return nil
	}
	ids := l.Heads()
	if len(ids) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(ids))
	for _, id := range ids {
		raw, err := base64.RawURLEncoding.DecodeString(id)
		if err != nil {
			// Keep API resilient even if an external snapshot has malformed IDs.
			raw = []byte(id)
		}
		out = append(out, append([]byte(nil), raw...))
	}
	return out
}

func normalizeEventIDInput(id []byte) (string, bool) {
	if len(id) == 0 {
		return "", false
	}
	asString := strings.TrimSpace(string(id))
	if asString != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(asString); err == nil {
			if base64.RawURLEncoding.EncodeToString(raw) == asString {
				return asString, true
			}
		}
	}
	return base64.RawURLEncoding.EncodeToString(id), true
}
