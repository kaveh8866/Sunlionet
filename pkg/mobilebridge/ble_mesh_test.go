package mobilebridge

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBLEAdvertPayloadFitsLegacyAdvertisingBudget(t *testing.T) {
	raw, err := BuildBLEAdvertPayload([]byte("secret"), []byte("node-1234"), []byte("config-v1"), 1_700_000_000)
	if err != nil {
		t.Fatalf("BuildBLEAdvertPayload: %v", err)
	}
	if len(raw) != BLEAdvertPayloadSize || len(raw) > 31 {
		t.Fatalf("advert payload size=%d", len(raw))
	}
	epoch, peer, hash, ok := ParseBLEAdvertPayload(raw)
	if !ok {
		t.Fatalf("parse failed")
	}
	if epoch == 0 || len(peer) != 8 || len(hash) != 6 {
		t.Fatalf("bad fields epoch=%d peer=%d hash=%d", epoch, len(peer), len(hash))
	}
}

func TestBLECheckpointResume(t *testing.T) {
	payload := make([]byte, 401)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	cp, err := NewBLETransferCheckpoint(payload, 100)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if cp.TotalChunks != 5 {
		t.Fatalf("chunks=%d", cp.TotalChunks)
	}
	if err := cp.MarkChunk(0, payload[:100]); err != nil {
		t.Fatalf("mark 0: %v", err)
	}
	if err := cp.MarkChunk(2, payload[200:300]); err != nil {
		t.Fatalf("mark 2: %v", err)
	}
	if got := cp.NextMissing(); got != 1 {
		t.Fatalf("next=%d", got)
	}
	path := filepath.Join(t.TempDir(), "ble_checkpoint.json")
	if err := SaveBLECheckpoint(path, cp); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadBLECheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.NextMissing() != 1 {
		t.Fatalf("loaded next=%d", loaded.NextMissing())
	}
}

func TestBLEMeshStateCycle(t *testing.T) {
	engine := NewBLEMeshEngine([]byte("cfg"), BLEMeshConfig{})
	now := time.Unix(1_700_000_000, 0)
	if s := engine.Next(now, false); s.Mode != BLEMeshStateAdvertisingVersion {
		t.Fatalf("state=%s", s.Mode)
	}
	if s := engine.Next(now, false); s.Mode != BLEMeshStateDiscovering {
		t.Fatalf("state=%s", s.Mode)
	}
	if s := engine.Next(now, true); s.Mode != BLEMeshStateConnectingGATT {
		t.Fatalf("state=%s", s.Mode)
	}
}
