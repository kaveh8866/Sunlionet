package outsidectl

import (
	"strings"
	"testing"
)

func TestChunkStringRoundTrip(t *testing.T) {
	uri := "snb://v2:" + strings.Repeat("a", 5000)
	chunks, err := ChunkString(uri, 900)
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	lines, err := RenderChunkLines(chunks)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out, err := JoinChunkLines(lines)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if out != uri {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestJoinChunksRejectsMissing(t *testing.T) {
	uri := "snb://v2:" + strings.Repeat("b", 2500)
	chunks, err := ChunkString(uri, 500)
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	chunks = chunks[:len(chunks)-1]
	if _, err := JoinChunks(chunks); err == nil {
		t.Fatalf("expected missing chunk error")
	}
}
