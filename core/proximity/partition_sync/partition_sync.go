package partition_sync

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/cache_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type MessageID = cache_manager.MessageID

const (
	KindInventory byte = 2
	KindWant      byte = 3
	KindCover     byte = 4
)

type InventoryItem struct {
	ID        MessageID
	ExpiresAt time.Time
}

type InventoryFrame struct {
	Sender    identity_manager.NodeID
	Timestamp time.Time
	Items     []InventoryItem
}

type WantFrame struct {
	Sender identity_manager.NodeID
	Wants  []MessageID
}

func EncodeInventory(sender identity_manager.NodeID, now time.Time, items []InventoryItem, maxItems int) ([]byte, error) {
	if maxItems <= 0 {
		maxItems = 12
	}
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	ts := uint32(now.Unix())
	out := make([]byte, 0, 4+8+4+1+len(items)*(16+4))
	out = append(out, byte('S'), byte('L'), byte(1), KindInventory)
	out = append(out, sender[:]...)
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], ts)
	out = append(out, b[:]...)
	out = append(out, byte(len(items)))
	for i := range items {
		out = append(out, items[i].ID[:]...)
		exp := uint32(items[i].ExpiresAt.Unix())
		binary.LittleEndian.PutUint32(b[:], exp)
		out = append(out, b[:]...)
	}
	return out, nil
}

func DecodeInventory(b []byte) (InventoryFrame, error) {
	min := 4 + 8 + 4 + 1
	if len(b) < min {
		return InventoryFrame{}, errors.New("inv too small")
	}
	if b[0] != 'S' || b[1] != 'L' || b[2] != 1 || b[3] != KindInventory {
		return InventoryFrame{}, errors.New("bad inv header")
	}
	off := 4
	var f InventoryFrame
	copy(f.Sender[:], b[off:off+8])
	off += 8
	ts := binary.LittleEndian.Uint32(b[off : off+4])
	off += 4
	f.Timestamp = time.Unix(int64(ts), 0).UTC()
	n := int(b[off])
	off++
	need := off + n*(16+4)
	if len(b) < need {
		return InventoryFrame{}, errors.New("inv truncated")
	}
	f.Items = make([]InventoryItem, 0, n)
	for i := 0; i < n; i++ {
		var id MessageID
		copy(id[:], b[off:off+16])
		off += 16
		exp := binary.LittleEndian.Uint32(b[off : off+4])
		off += 4
		f.Items = append(f.Items, InventoryItem{
			ID:        id,
			ExpiresAt: time.Unix(int64(exp), 0).UTC(),
		})
	}
	return f, nil
}

func EncodeWant(sender identity_manager.NodeID, ids []MessageID, max int) ([]byte, error) {
	if max <= 0 {
		max = 10
	}
	if len(ids) > max {
		ids = ids[:max]
	}
	out := make([]byte, 0, 4+8+1+len(ids)*16)
	out = append(out, byte('S'), byte('L'), byte(1), KindWant)
	out = append(out, sender[:]...)
	out = append(out, byte(len(ids)))
	for i := range ids {
		out = append(out, ids[i][:]...)
	}
	return out, nil
}

func DecodeWant(b []byte) (WantFrame, error) {
	min := 4 + 8 + 1
	if len(b) < min {
		return WantFrame{}, errors.New("want too small")
	}
	if b[0] != 'S' || b[1] != 'L' || b[2] != 1 || b[3] != KindWant {
		return WantFrame{}, errors.New("bad want header")
	}
	off := 4
	var f WantFrame
	copy(f.Sender[:], b[off:off+8])
	off += 8
	n := int(b[off])
	off++
	need := off + n*16
	if len(b) < need {
		return WantFrame{}, errors.New("want truncated")
	}
	f.Wants = make([]MessageID, 0, n)
	for i := 0; i < n; i++ {
		var id MessageID
		copy(id[:], b[off:off+16])
		off += 16
		f.Wants = append(f.Wants, id)
	}
	return f, nil
}

func EncodeCover(now time.Time, size int) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("size must be >= 0")
	}
	if size > 255 {
		size = 255
	}
	ts := uint32(now.Unix())
	out := make([]byte, 0, 4+4+1+size)
	out = append(out, byte('S'), byte('L'), byte(1), KindCover)
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], ts)
	out = append(out, b[:]...)
	out = append(out, byte(size))
	payload := make([]byte, size)
	_, _ = rand.Read(payload)
	out = append(out, payload...)
	return out, nil
}
