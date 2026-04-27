package message_router

import (
	"errors"
	"fmt"
)

func ChunkMessage(msg Message, maxFrameBytes int) ([]ChunkFrame, error) {
	if maxFrameBytes <= 0 {
		return nil, errors.New("max_frame_bytes must be > 0")
	}
	overhead := 2 + 1 + 1 + 16 + 8 + (4 + 2 + 1 + 2 + 2)
	if maxFrameBytes <= overhead {
		return nil, fmt.Errorf("max_frame_bytes too small: need > %d", overhead)
	}
	maxData := maxFrameBytes - overhead
	payload := msg.Payload
	if len(payload) == 0 {
		f := ChunkFrame{
			MsgID:      msg.ID,
			Sender:     msg.Sender,
			Timestamp:  msg.Timestamp,
			TTLSec:     msg.TTLSec,
			Hop:        msg.Hop,
			ChunkIndex: 0,
			ChunkCount: 1,
			Data:       nil,
		}
		return []ChunkFrame{f}, nil
	}
	chunks := (len(payload) + maxData - 1) / maxData
	if chunks > 65535 {
		return nil, errors.New("too many chunks")
	}
	out := make([]ChunkFrame, 0, chunks)
	for i := 0; i < chunks; i++ {
		start := i * maxData
		end := start + maxData
		if end > len(payload) {
			end = len(payload)
		}
		f := ChunkFrame{
			MsgID:      msg.ID,
			Sender:     msg.Sender,
			Timestamp:  msg.Timestamp,
			TTLSec:     msg.TTLSec,
			Hop:        msg.Hop,
			ChunkIndex: uint16(i),
			ChunkCount: uint16(chunks),
			Data:       append([]byte(nil), payload[start:end]...),
		}
		out = append(out, f)
	}
	return out, nil
}
