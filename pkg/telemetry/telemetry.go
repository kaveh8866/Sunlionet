package telemetry

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	SchemaVersion       = 1
	MaxQueueBytes       = 50 * 1024
	MaxEventsPerQueue   = 256
	MinDispatchInterval = 24 * time.Hour
	MaxDispatchInterval = 72 * time.Hour
	MaxReportCounters   = 32
)

type EventCode uint16

const (
	EventUnknown EventCode = iota
	EventProxyHandshakeTimeout
	EventDNSBlocked
	EventTCPReset
	EventTLSBlocked
	EventNoRoute
	EventConfigInvalid
	EventRuntimeCrash
	EventVPNRestart
	EventKernelTunFailure
	EventBLETransferInterrupted
	EventImportSignatureInvalid
	EventImportReplayDetected
	EventCoreStartFailure
)

type CoreVersion uint16

const (
	CoreVersionUnknown     CoreVersion = 0
	CoreVersionSunLionetV1 CoreVersion = 1
)

type CarrierClass uint8

const (
	CarrierUnknown CarrierClass = iota
	CarrierSimulated
	CarrierMobile
	CarrierWifi
	CarrierMixed
)

type TransportKind uint8

const (
	TransportUnknown TransportKind = iota
	TransportOnion
	TransportI2P
	TransportMixnet
	TransportDomainFronted
)

type DiagnosticEvent struct {
	Code        EventCode
	CoreVersion CoreVersion
	Carrier     CarrierClass
}

type Config struct {
	Enabled                   bool
	QueuePath                 string
	CollectorPublicKeyB64URL  string
	Transport                 TransportKind
	EndpointURL               string
	AllowUnsafeDirectForTests bool
	Now                       func() time.Time
	Rand                      io.Reader
}

type Sender interface {
	Send(ctx context.Context, envelope []byte) error
}

type Engine struct {
	mu     sync.Mutex
	cfg    Config
	sender Sender
}

type queueFile struct {
	Version   int          `json:"v"`
	NextAfter int64        `json:"next_after"`
	Events    []queueEvent `json:"events"`
}

type queueEvent struct {
	Code        uint16 `json:"c"`
	CoreVersion uint16 `json:"v"`
	Carrier     uint8  `json:"k"`
}

type encryptedEnvelope struct {
	Schema       int    `json:"s"`
	Transport    uint8  `json:"t"`
	EphemeralPub string `json:"e"`
	Nonce        string `json:"n"`
	Ciphertext   string `json:"c"`
}

func NewEngine(cfg Config, sender Sender) (*Engine, error) {
	if !cfg.Enabled {
		return &Engine{cfg: cfg, sender: sender}, nil
	}
	if cfg.QueuePath == "" {
		return nil, errors.New("telemetry: missing queue path")
	}
	if cfg.Transport == TransportUnknown {
		return nil, errors.New("telemetry: missing privacy transport")
	}
	if cfg.Transport != TransportOnion && cfg.Transport != TransportI2P && cfg.Transport != TransportMixnet && cfg.Transport != TransportDomainFronted {
		return nil, errors.New("telemetry: unsupported privacy transport")
	}
	if cfg.EndpointURL != "" && !cfg.AllowUnsafeDirectForTests {
		u, err := url.Parse(cfg.EndpointURL)
		if err != nil {
			return nil, fmt.Errorf("telemetry: invalid endpoint url: %w", err)
		}
		host := u.Hostname()
		if cfg.Transport == TransportOnion && !hasSuffix(host, ".onion") {
			return nil, errors.New("telemetry: onion transport requires .onion endpoint")
		}
		if cfg.Transport == TransportI2P && !hasSuffix(host, ".i2p") {
			return nil, errors.New("telemetry: i2p transport requires .i2p endpoint")
		}
	}
	if _, err := decodeCollectorKey(cfg.CollectorPublicKeyB64URL); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Rand == nil {
		cfg.Rand = rand.Reader
	}
	return &Engine{cfg: cfg, sender: sender}, nil
}

func (e *Engine) Enabled() bool {
	return e != nil && e.cfg.Enabled
}

func (e *Engine) Record(ev DiagnosticEvent) error {
	if e == nil || !e.cfg.Enabled {
		return nil
	}
	if !validEvent(ev) {
		ev = DiagnosticEvent{Code: EventUnknown, CoreVersion: CoreVersionUnknown, Carrier: CarrierUnknown}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	q, err := e.loadQueueLocked()
	if err != nil {
		q = queueFile{Version: SchemaVersion}
	}
	if q.NextAfter == 0 {
		q.NextAfter = e.nextAfterLocked()
	}
	q.Events = append(q.Events, queueEvent{
		Code:        uint16(ev.Code),
		CoreVersion: uint16(ev.CoreVersion),
		Carrier:     uint8(ev.Carrier),
	})
	for len(q.Events) > MaxEventsPerQueue {
		q.Events = q.Events[1:]
	}
	return e.saveQueueLocked(q)
}

func (e *Engine) FlushDue(ctx context.Context) (bool, error) {
	if e == nil || !e.cfg.Enabled || e.sender == nil {
		return false, nil
	}
	e.mu.Lock()
	q, err := e.loadQueueLocked()
	if err != nil {
		e.mu.Unlock()
		return false, nil
	}
	now := e.cfg.Now().Unix()
	if len(q.Events) == 0 || now < q.NextAfter {
		e.mu.Unlock()
		return false, nil
	}
	env, err := e.buildEnvelopeLocked(q)
	if err != nil {
		_ = e.destroyQueueLocked()
		e.mu.Unlock()
		return false, err
	}
	e.mu.Unlock()

	if err := e.sender.Send(ctx, env); err != nil {
		e.mu.Lock()
		defer e.mu.Unlock()
		if info, statErr := os.Stat(e.cfg.QueuePath); statErr == nil && info.Size() >= MaxQueueBytes {
			_ = e.destroyQueueLocked()
		}
		return false, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	return true, e.destroyQueueLocked()
}

func (e *Engine) buildEnvelopeLocked(q queueFile) ([]byte, error) {
	collector, err := decodeCollectorKey(e.cfg.CollectorPublicKeyB64URL)
	if err != nil {
		return nil, err
	}
	report, err := e.aggregateReportLocked(q.Events)
	if err != nil {
		return nil, err
	}
	priv, err := ecdh.X25519().GenerateKey(e.cfg.Rand)
	if err != nil {
		return nil, err
	}
	shared, err := priv.ECDH(collector)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 32)
	if _, err := io.ReadFull(e.cfg.Rand, salt); err != nil {
		return nil, err
	}
	key, err := hkdf.Key(sha256.New, shared, salt, "SUNLIONET-TELEMETRY-V1", chacha20poly1305.KeySize)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(e.cfg.Rand, nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, report, []byte{byte(e.cfg.Transport), byte(SchemaVersion)})
	nonceWithSalt := append(salt, nonce...)
	env := encryptedEnvelope{
		Schema:       SchemaVersion,
		Transport:    uint8(e.cfg.Transport),
		EphemeralPub: base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
		Nonce:        base64.RawURLEncoding.EncodeToString(nonceWithSalt),
		Ciphertext:   base64.RawURLEncoding.EncodeToString(ct),
	}
	return json.Marshal(env)
}

func (e *Engine) aggregateReportLocked(events []queueEvent) ([]byte, error) {
	counts := map[queueEvent]int{}
	for _, ev := range events {
		counts[ev]++
	}
	keys := make([]queueEvent, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Code != keys[j].Code {
			return keys[i].Code < keys[j].Code
		}
		if keys[i].CoreVersion != keys[j].CoreVersion {
			return keys[i].CoreVersion < keys[j].CoreVersion
		}
		return keys[i].Carrier < keys[j].Carrier
	})
	if len(keys) > MaxReportCounters {
		keys = keys[:MaxReportCounters]
	}
	buf := bytes.NewBuffer(make([]byte, 0, 8+len(keys)*8))
	buf.WriteByte(byte(SchemaVersion))
	buf.WriteByte(byte(e.cfg.Transport))
	putU16(buf, uint16(len(keys)))
	for _, k := range keys {
		c := counts[k] + discreteNoise(e.cfg.Rand)
		if c < 0 {
			c = 0
		}
		buf.WriteByte(byte(k.Code >> 8))
		buf.WriteByte(byte(k.Code))
		buf.WriteByte(byte(k.CoreVersion >> 8))
		buf.WriteByte(byte(k.CoreVersion))
		buf.WriteByte(k.Carrier)
		putU16(buf, uint16(c))
	}
	return buf.Bytes(), nil
}

func (e *Engine) loadQueueLocked() (queueFile, error) {
	raw, err := os.ReadFile(e.cfg.QueuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return queueFile{Version: SchemaVersion, NextAfter: e.nextAfterLocked()}, nil
		}
		return queueFile{}, err
	}
	if len(raw) >= MaxQueueBytes {
		_ = e.destroyQueueLocked()
		return queueFile{}, errors.New("telemetry: queue too large")
	}
	var q queueFile
	if err := json.Unmarshal(raw, &q); err != nil {
		_ = e.destroyQueueLocked()
		return queueFile{}, err
	}
	if q.Version != SchemaVersion {
		return queueFile{}, errors.New("telemetry: unsupported queue version")
	}
	return q, nil
}

func (e *Engine) saveQueueLocked(q queueFile) error {
	if len(q.Events) == 0 {
		return e.destroyQueueLocked()
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	if len(raw) > MaxQueueBytes {
		return e.destroyQueueLocked()
	}
	if err := os.MkdirAll(filepath.Dir(e.cfg.QueuePath), 0700); err != nil {
		return err
	}
	tmp := e.cfg.QueuePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, e.cfg.QueuePath)
}

func (e *Engine) destroyQueueLocked() error {
	if e == nil || e.cfg.QueuePath == "" {
		return nil
	}
	if err := os.Remove(e.cfg.QueuePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(e.cfg.QueuePath + ".tmp")
	return nil
}

func (e *Engine) nextAfterLocked() int64 {
	jitter := randomInt(e.cfg.Rand, int64(MaxDispatchInterval-MinDispatchInterval))
	return e.cfg.Now().Add(MinDispatchInterval + time.Duration(jitter)).Unix()
}

type HTTPSender struct {
	Endpoint string
	Client   *http.Client
}

func (s HTTPSender) Send(ctx context.Context, envelope []byte) error {
	if s.Endpoint == "" {
		return errors.New("telemetry: missing endpoint")
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(envelope))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("telemetry: delivery failed status=%d", res.StatusCode)
	}
	return nil
}

func MapFailureReason(reason string) EventCode {
	switch reason {
	case "TIMEOUT":
		return EventProxyHandshakeTimeout
	case "DNS_FAILURE", "DNS_BLOCKED":
		return EventDNSBlocked
	case "TCP_RESET":
		return EventTCPReset
	case "TLS_BLOCKED":
		return EventTLSBlocked
	case "NO_ROUTE":
		return EventNoRoute
	case "CONFIG_ERROR":
		return EventConfigInvalid
	default:
		return EventUnknown
	}
}

func validEvent(ev DiagnosticEvent) bool {
	if ev.Code > EventCoreStartFailure {
		return false
	}
	if ev.CoreVersion > CoreVersionSunLionetV1 {
		return false
	}
	if ev.Carrier > CarrierMixed {
		return false
	}
	return true
}

func decodeCollectorKey(s string) (*ecdh.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("telemetry: invalid collector key: %w", err)
	}
	pub, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("telemetry: invalid collector key: %w", err)
	}
	return pub, nil
}

func discreteNoise(r io.Reader) int {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0
	}
	switch b[0] & 0x0f {
	case 0:
		return -1
	case 1:
		return 1
	default:
		return 0
	}
}

func randomInt(r io.Reader, max int64) int64 {
	if max <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b[:]) % uint64(max))
}

func putU16(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v))
}

func hasSuffix(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}
