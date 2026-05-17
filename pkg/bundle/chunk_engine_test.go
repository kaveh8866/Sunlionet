package bundle

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"
	"time"
)

func TestErasureChunks_ReconstructWithLostDataChunks(t *testing.T) {
	payload := deterministicPayload(4097)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 6, ParityShards: 3, ShardSize: 700})
	if err != nil {
		t.Fatal(err)
	}
	kept := []EncodedChunk{chunks[1], chunks[3], chunks[5], chunks[6], chunks[7], chunks[8]}
	out, err := ReconstructErasurePayload(kept)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatal("reconstructed payload mismatch")
	}
}

func TestChunkReassembler_OutOfOrderCompletesAndFlushes(t *testing.T) {
	payload := deterministicPayload(2048)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 4, ParityShards: 2, ShardSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	order := []int{5, 1, 4, 2}
	r := NewChunkReassembler(DefaultMaxCacheByte, time.Minute)
	var got []byte
	for i, idx := range order {
		raw, err := chunks[idx].MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		out, done, err := r.Add(raw, time.Unix(int64(i), 0))
		if err != nil {
			t.Fatal(err)
		}
		if done {
			got = out
		}
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}
	if len(r.partials) != 0 {
		t.Fatal("reassembler did not flush completed cache")
	}
}

func TestChunkParserRejectsCorruption(t *testing.T) {
	payload := deterministicPayload(1024)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 3, ParityShards: 2, ShardSize: 400})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := chunks[0].MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0x55
	if _, err := ParseEncodedChunk(raw); !errors.Is(err, ErrChunkChecksum) {
		t.Fatalf("expected checksum error, got %v", err)
	}
}

func TestChunkReassemblerMemoryBound(t *testing.T) {
	payload := deterministicPayload(2048)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 4, ParityShards: 2, ShardSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	r := NewChunkReassembler(ChunkHeaderSize+100, time.Minute)
	_, done, err := r.AddChunk(chunks[0], time.Now())
	if !errors.Is(err, ErrChunkCacheLimit) {
		t.Fatalf("expected cache limit, done=%v err=%v", done, err)
	}
}

func TestChunkTextEnvelopeRoundTrip(t *testing.T) {
	payload := deterministicPayload(777)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 3, ParityShards: 1, ShardSize: 259})
	if err != nil {
		t.Fatal(err)
	}
	line, err := chunks[2].EncodeText()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseEncodedChunkText(line)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Index != chunks[2].Index || !bytes.Equal(parsed.Data, chunks[2].Data) {
		t.Fatal("text round-trip mismatch")
	}
}

func TestErasureChunks_RandomLossPatterns(t *testing.T) {
	payload := deterministicPayload(3001)
	chunks, err := EncodeErasureChunks(payload, ChunkOptions{DataShards: 7, ParityShards: 4, ShardSize: 429})
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 50; trial++ {
		perm := rng.Perm(len(chunks))
		var kept []EncodedChunk
		for _, idx := range perm[:7] {
			kept = append(kept, chunks[idx])
		}
		out, err := ReconstructErasurePayload(kept)
		if err != nil {
			t.Fatalf("trial %d: %v indices=%v", trial, err, perm[:7])
		}
		if !bytes.Equal(out, payload) {
			t.Fatalf("trial %d: payload mismatch", trial)
		}
	}
}

func deterministicPayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((i*31 + 17) % 251)
	}
	return out
}
