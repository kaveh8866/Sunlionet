package message_router

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

const (
	wireMagic0  = byte('S')
	wireMagic1  = byte('L')
	wireVersion = byte(1)

	wireKindChunk = byte(1)
)

func DecodeHeader(b []byte) (version byte, kind byte, ok bool) {
	if len(b) < 4 {
		return 0, 0, false
	}
	if b[0] != wireMagic0 || b[1] != wireMagic1 {
		return 0, 0, false
	}
	return b[2], b[3], true
}

type ChunkFrame struct {
	MsgID      MessageID
	Sender     identity_manager.NodeID
	Timestamp  time.Time
	TTLSec     uint16
	Hop        uint8
	ChunkIndex uint16
	ChunkCount uint16
	Data       []byte
}

func (f ChunkFrame) ExpiresAt() time.Time {
	return f.Timestamp.Add(time.Duration(f.TTLSec) * time.Second)
}

func EncodeChunkFrame(f ChunkFrame) ([]byte, error) {
	if f.TTLSec == 0 {
		return nil, errors.New("ttl_sec must be > 0")
	}
	if f.ChunkCount == 0 || f.ChunkIndex >= f.ChunkCount {
		return nil, fmt.Errorf("invalid chunk index/count: %d/%d", f.ChunkIndex, f.ChunkCount)
	}
	if f.Timestamp.IsZero() {
		return nil, errors.New("timestamp required")
	}
	ts := uint32(f.Timestamp.Unix())

	out := make([]byte, 0, 64+len(f.Data))
	out = append(out, wireMagic0, wireMagic1, wireVersion, wireKindChunk)
	out = append(out, f.MsgID[:]...)
	out = append(out, f.Sender[:]...)

	var b [4 + 2 + 1 + 2 + 2]byte
	binary.LittleEndian.PutUint32(b[0:4], ts)
	binary.LittleEndian.PutUint16(b[4:6], f.TTLSec)
	b[6] = f.Hop
	binary.LittleEndian.PutUint16(b[7:9], f.ChunkIndex)
	binary.LittleEndian.PutUint16(b[9:11], f.ChunkCount)
	out = append(out, b[:]...)
	out = append(out, f.Data...)
	return out, nil
}

func DecodeChunkFrame(b []byte) (ChunkFrame, error) {
	min := 2 + 1 + 1 + 16 + 8 + (4 + 2 + 1 + 2 + 2)
	if len(b) < min {
		return ChunkFrame{}, errors.New("frame too small")
	}
	if b[0] != wireMagic0 || b[1] != wireMagic1 {
		return ChunkFrame{}, errors.New("bad magic")
	}
	if b[2] != wireVersion {
		return ChunkFrame{}, errors.New("unsupported version")
	}
	if b[3] != wireKindChunk {
		return ChunkFrame{}, errors.New("unsupported kind")
	}

	var f ChunkFrame
	off := 4
	copy(f.MsgID[:], b[off:off+16])
	off += 16
	copy(f.Sender[:], b[off:off+8])
	off += 8

	ts := binary.LittleEndian.Uint32(b[off : off+4])
	off += 4
	f.TTLSec = binary.LittleEndian.Uint16(b[off : off+2])
	off += 2
	f.Hop = b[off]
	off++
	f.ChunkIndex = binary.LittleEndian.Uint16(b[off : off+2])
	off += 2
	f.ChunkCount = binary.LittleEndian.Uint16(b[off : off+2])
	off += 2

	if f.TTLSec == 0 {
		return ChunkFrame{}, errors.New("ttl_sec must be > 0")
	}
	if f.ChunkCount == 0 || f.ChunkIndex >= f.ChunkCount {
		return ChunkFrame{}, fmt.Errorf("invalid chunk index/count: %d/%d", f.ChunkIndex, f.ChunkCount)
	}
	f.Timestamp = time.Unix(int64(ts), 0).UTC()
	f.Data = append([]byte(nil), b[off:]...)
	return f, nil
}
