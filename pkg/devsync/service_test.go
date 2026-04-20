package devsync

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordBuildApplyBatch(t *testing.T) {
	dir := t.TempDir()
	mk := []byte("0123456789abcdef0123456789abcdef")

	saStore, err := NewStore(filepath.Join(dir, "a.enc"), mk)
	if err != nil {
		t.Fatalf("NewStore(a): %v", err)
	}
	sbStore, err := NewStore(filepath.Join(dir, "b.enc"), mk)
	if err != nil {
		t.Fatalf("NewStore(b): %v", err)
	}
	sa, err := NewService(saStore)
	if err != nil {
		t.Fatalf("NewService(a): %v", err)
	}
	sb, err := NewService(sbStore)
	if err != nil {
		t.Fatalf("NewService(b): %v", err)
	}
	if err := sa.SetLocalDeviceID("dev-a"); err != nil {
		t.Fatalf("SetLocalDeviceID(a): %v", err)
	}
	if err := sb.SetLocalDeviceID("dev-b"); err != nil {
		t.Fatalf("SetLocalDeviceID(b): %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"chat_id": "c1", "text": "hello"})
	if _, err := sa.Record("msg.append", payload, time.Hour); err != nil {
		t.Fatalf("Record(a): %v", err)
	}

	batch := sa.BuildBatch(sb.Snapshot().Cursors, 100)
	applied, err := sb.ApplyBatch(batch)
	if err != nil {
		t.Fatalf("ApplyBatch(b): %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected 1 applied event, got %d", applied)
	}
	appliedAgain, err := sb.ApplyBatch(batch)
	if err != nil {
		t.Fatalf("ApplyBatch dedupe(b): %v", err)
	}
	if appliedAgain != 0 {
		t.Fatalf("expected 0 applied on duplicate batch, got %d", appliedAgain)
	}
	if len(sb.Snapshot().Events) != 1 {
		t.Fatalf("expected 1 event in receiver snapshot")
	}
}

func TestOutboxRetryAndAck(t *testing.T) {
	dir := t.TempDir()
	mk := []byte("0123456789abcdef0123456789abcdef")
	ds, err := NewStore(filepath.Join(dir, "sync.enc"), mk)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	svc, err := NewService(ds)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.SetLocalDeviceID("dev-1"); err != nil {
		t.Fatalf("SetLocalDeviceID: %v", err)
	}
	payload, _ := json.Marshal(map[string]string{"kind": "ping"})
	ev, err := svc.Record("heartbeat", payload, time.Minute)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, ok := svc.NextOutbox(time.Now())
	if !ok || got.ID != ev.ID {
		t.Fatalf("expected pending outbox event")
	}
	if err := svc.MarkEventRetry(ev.ID, time.Now()); err != nil {
		t.Fatalf("MarkEventRetry: %v", err)
	}
	got2, ok := svc.NextOutbox(time.Now())
	if ok && got2.ID == ev.ID {
		t.Fatalf("event should be delayed after retry")
	}
	if err := svc.AckEvent(ev.ID); err != nil {
		t.Fatalf("AckEvent: %v", err)
	}
	if _, ok := svc.NextOutbox(time.Now().Add(10 * time.Minute)); ok {
		t.Fatalf("expected empty outbox after ack")
	}
}
