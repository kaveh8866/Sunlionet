package message_router

import (
	"errors"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type partial struct {
	sender    [8]byte
	timestamp time.Time
	ttlSec    uint16
	hop       uint8
	count     uint16
	received  uint16
	chunks    [][]byte
	lastAt    time.Time
	total     int
}

type Reassembler struct {
	mu         sync.Mutex
	parts      map[MessageID]*partial
	staleAfter time.Duration
	maxBytes   int
}

func NewReassembler(staleAfter time.Duration) *Reassembler {
	return NewReassemblerWithLimit(staleAfter, 0)
}

func NewReassemblerWithLimit(staleAfter time.Duration, maxBytes int) *Reassembler {
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	if maxBytes <= 0 {
		maxBytes = 32 * 1024
	}
	return &Reassembler{
		parts:      make(map[MessageID]*partial),
		staleAfter: staleAfter,
		maxBytes:   maxBytes,
	}
}

func (r *Reassembler) Add(frame ChunkFrame, now time.Time) (Message, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.parts[frame.MsgID]
	if !ok {
		p = &partial{
			timestamp: frame.Timestamp,
			ttlSec:    frame.TTLSec,
			hop:       frame.Hop,
			count:     frame.ChunkCount,
			chunks:    make([][]byte, int(frame.ChunkCount)),
			lastAt:    now,
		}
		copy(p.sender[:], frame.Sender[:])
		r.parts[frame.MsgID] = p
	}

	if p.count != frame.ChunkCount {
		return Message{}, false, errors.New("chunk count mismatch")
	}
	if p.timestamp.Unix() != frame.Timestamp.Unix() || p.ttlSec != frame.TTLSec {
		return Message{}, false, errors.New("header mismatch")
	}
	if p.chunks[int(frame.ChunkIndex)] == nil {
		if p.total+len(frame.Data) > r.maxBytes {
			delete(r.parts, frame.MsgID)
			return Message{}, false, errors.New("payload too large")
		}
		p.chunks[int(frame.ChunkIndex)] = append([]byte(nil), frame.Data...)
		p.received++
		p.total += len(frame.Data)
	}
	p.lastAt = now

	if p.received < p.count {
		return Message{}, false, nil
	}

	payload := make([]byte, 0, 512)
	for i := 0; i < int(p.count); i++ {
		payload = append(payload, p.chunks[i]...)
	}

	var sender identity_manager.NodeID
	copy(sender[:], p.sender[:])
	msg := Message{
		Sender:    sender,
		Timestamp: p.timestamp,
		TTLSec:    p.ttlSec,
		Hop:       p.hop,
		Payload:   payload,
	}
	msg.ID = ComputeMessageID(msg.Sender, msg.Timestamp, msg.TTLSec, msg.Payload)
	delete(r.parts, frame.MsgID)
	return msg, true, nil
}

func (r *Reassembler) Sweep(now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, p := range r.parts {
		if now.Sub(p.lastAt) >= r.staleAfter {
			delete(r.parts, id)
			removed++
		}
	}
	return removed
}
