package telemetry

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type captureSender struct {
	payloads [][]byte
	err      error
}

func (s *captureSender) Send(_ context.Context, envelope []byte) error {
	if s.err != nil {
		return s.err
	}
	s.payloads = append(s.payloads, append([]byte(nil), envelope...))
	return nil
}

func TestEngineDormantByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.queue")
	e, err := NewEngine(Config{Enabled: false, QueuePath: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Record(DiagnosticEvent{Code: EventDNSBlocked}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no queue file, err=%v", err)
	}
}

func TestEngineRejectsDirectEndpointWithoutPrivacyTransport(t *testing.T) {
	pub := collectorPubB64(t)
	_, err := NewEngine(Config{
		Enabled:                  true,
		QueuePath:                filepath.Join(t.TempDir(), "q"),
		CollectorPublicKeyB64URL: pub,
		Transport:                TransportOnion,
		EndpointURL:              "https://example.com/metrics",
	}, nil)
	if err == nil {
		t.Fatal("expected direct endpoint rejection")
	}
}

func TestRecordBuffersEnumsOnlyAndBlursDispatch(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	e, err := NewEngine(Config{
		Enabled:                  true,
		QueuePath:                filepath.Join(t.TempDir(), "q"),
		CollectorPublicKeyB64URL: collectorPubB64(t),
		Transport:                TransportOnion,
		Now:                      func() time.Time { return now },
		Rand:                     zeroReader{},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Record(DiagnosticEvent{Code: EventTCPReset, CoreVersion: CoreVersionSunLionetV1, Carrier: CarrierMobile}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(e.cfg.QueuePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || contains(raw, []byte("TCP_RESET")) || contains(raw, []byte("mobile")) {
		t.Fatalf("queue leaked string labels: %s", raw)
	}
	var q queueFile
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if q.NextAfter < now.Add(MinDispatchInterval).Unix() || q.NextAfter > now.Add(MaxDispatchInterval).Unix() {
		t.Fatalf("next_after outside blur window: %d", q.NextAfter)
	}
}

func TestFlushEncryptsWithFreshEphemeralKeyAndDestroysQueue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	sender := &captureSender{}
	e, err := NewEngine(Config{
		Enabled:                   true,
		QueuePath:                 filepath.Join(t.TempDir(), "q"),
		CollectorPublicKeyB64URL:  collectorPubB64(t),
		Transport:                 TransportOnion,
		EndpointURL:               "http://collector.onion/metrics",
		AllowUnsafeDirectForTests: true,
		Now:                       func() time.Time { return now },
		Rand:                      rand.Reader,
	}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Record(DiagnosticEvent{Code: EventTLSBlocked, CoreVersion: CoreVersionSunLionetV1}); err != nil {
		t.Fatal(err)
	}
	q, err := e.loadQueueLocked()
	if err != nil {
		t.Fatal(err)
	}
	q.NextAfter = now.Add(-time.Second).Unix()
	if err := e.saveQueueLocked(q); err != nil {
		t.Fatal(err)
	}
	ok, err := e.FlushDue(context.Background())
	if err != nil || !ok {
		t.Fatalf("flush ok=%v err=%v", ok, err)
	}
	if len(sender.payloads) != 1 {
		t.Fatalf("payload count=%d", len(sender.payloads))
	}
	var env encryptedEnvelope
	if err := json.Unmarshal(sender.payloads[0], &env); err != nil {
		t.Fatal(err)
	}
	if env.EphemeralPub == "" || env.Ciphertext == "" || env.Nonce == "" {
		t.Fatalf("missing envelope fields: %+v", env)
	}
	if _, err := os.Stat(e.cfg.QueuePath); !os.IsNotExist(err) {
		t.Fatalf("queue not destroyed, err=%v", err)
	}
}

func TestFailedFlushDestroysOversizeQueue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	sender := &captureSender{err: errors.New("offline")}
	e, err := NewEngine(Config{
		Enabled:                   true,
		QueuePath:                 filepath.Join(t.TempDir(), "q"),
		CollectorPublicKeyB64URL:  collectorPubB64(t),
		Transport:                 TransportOnion,
		EndpointURL:               "http://collector.onion/metrics",
		AllowUnsafeDirectForTests: true,
		Now:                       func() time.Time { return now },
		Rand:                      zeroReader{},
	}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.cfg.QueuePath, make([]byte, MaxQueueBytes), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = e.FlushDue(context.Background())
	if err != nil {
		t.Fatalf("expected fail-silent flush, got %v", err)
	}
	if _, statErr := os.Stat(e.cfg.QueuePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected queue self-destruction, stat=%v", statErr)
	}
}

func collectorPubB64(t *testing.T) string {
	t.Helper()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes())
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func contains(haystack []byte, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
