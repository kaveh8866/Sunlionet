package message_router

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/cache_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"golang.org/x/crypto/blake2s"
)

type MessageID = cache_manager.MessageID

type Message struct {
	ID        MessageID
	Sender    identity_manager.NodeID
	Timestamp time.Time
	TTLSec    uint16
	Hop       uint8
	Payload   []byte
}

func NewMessage(sender identity_manager.NodeID, now time.Time, ttlSec uint16, payload []byte) (Message, error) {
	if ttlSec == 0 {
		return Message{}, errors.New("ttl_sec must be > 0")
	}
	cp := append([]byte(nil), payload...)
	msg := Message{
		Sender:    sender,
		Timestamp: now.UTC(),
		TTLSec:    ttlSec,
		Hop:       0,
		Payload:   cp,
	}
	msg.ID = ComputeMessageID(msg.Sender, msg.Timestamp, msg.TTLSec, msg.Payload)
	return msg, nil
}

func ComputeMessageID(sender identity_manager.NodeID, ts time.Time, ttlSec uint16, payload []byte) MessageID {
	var tmp [8 + 8 + 2]byte
	copy(tmp[:8], sender[:])
	binary.LittleEndian.PutUint64(tmp[8:16], uint64(ts.Unix()))
	binary.LittleEndian.PutUint16(tmp[16:18], ttlSec)
	h, _ := blake2s.New256(nil)
	_, _ = h.Write(tmp[:])
	_, _ = h.Write(payload)
	sum := h.Sum(nil)
	var id MessageID
	copy(id[:], sum[:16])
	return id
}

func (m Message) ExpiresAt() time.Time {
	return m.Timestamp.Add(time.Duration(m.TTLSec) * time.Second)
}
