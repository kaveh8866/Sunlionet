package chaos_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	mrand "math/rand/v2"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/bundle"
	"github.com/kaveh/sunlionet-agent/pkg/e2e"
	"github.com/kaveh/sunlionet-agent/pkg/proxycore"
	"github.com/kaveh/sunlionet-agent/pkg/telemetry"
)

const chaosSeed uint64 = 0x51A7E20260517

func TestChaosExtremeLossJitterChunkReassembly(t *testing.T) {
	t.Parallel()
	payload := deterministicPayload(32 * 1024)
	chunks, err := bundle.EncodeErasureChunks(payload, bundle.ChunkOptions{
		DataShards:   10,
		ParityShards: 60,
		ShardSize:    4096,
	})
	if err != nil {
		t.Fatal(err)
	}

	rng := randv2(chaosSeed)
	order := rng.Perm(len(chunks))
	transport := newLossyTransport(chaosSeed, 85, 500*time.Microsecond, 15*time.Millisecond)
	reassembler := bundle.NewChunkReassembler(bundle.DefaultMaxCacheByte, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var reconstructed []byte
	for delivered, idx := range order {
		raw, err := chunks[idx].MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		frame, ok, err := transport.Deliver(ctx, raw)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			continue
		}
		out, done, err := reassembler.Add(frame, time.Now())
		if err != nil {
			t.Fatalf("seed=%x delivered=%d idx=%d: %v", chaosSeed, delivered, idx, err)
		}
		if done {
			reconstructed = out
			break
		}
	}

	if !bytes.Equal(reconstructed, payload) {
		t.Fatalf("failed to reconstruct under deterministic loss seed=%x delivered=%d", chaosSeed, transport.delivered())
	}
}

func TestChaosRapidInterfaceFlappingFailClosedNoDeadlock(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ks := newChaosKillSwitch()
	vif := &mockVirtualInterface{}
	policy := proxycore.DefaultFailoverPolicy()
	policy.ValidationTimeout = 40 * time.Millisecond
	policy.HealthTimeout = 40 * time.Millisecond
	policy.BackoffBase = time.Millisecond
	policy.BackoffMax = 5 * time.Millisecond
	engine := proxycore.NewEngine(policy, ks, nil)
	candidates := []proxycore.Candidate{
		{
			Config: proxycore.CoreConfig{ID: "wifi", Protocol: "reality"},
			Core: &chaosCore{
				name: "wifi-core",
				ks:   ks,
				vif:  vif,
				health: proxycore.HealthSample{
					Status:              "failed",
					Reason:              e2e.ReasonNetworkBlocked,
					PacketDropPercent:   85,
					ConsecutiveFailures: 8,
				},
			},
		},
		{
			Config: proxycore.CoreConfig{ID: "cell", Protocol: "hysteria2"},
			Core: &chaosCore{
				name:      "cell-core",
				ks:        ks,
				vif:       vif,
				reloadErr: errors.New("interface flapped during handshake"),
				health:    proxycore.HealthSample{Status: "ok", Reason: e2e.ReasonOK},
			},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.Switch(ctx, candidates)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("deadlock or stalled failover loop seed=%x", chaosSeed)
	}
	if engine.State() != proxycore.StateBeacon && engine.State() != proxycore.StateIsolated {
		t.Fatalf("expected fail-closed state, got %s", engine.State())
	}
	if !ks.IsEngaged() {
		t.Fatalf("kill switch not engaged after total failure")
	}
	if vif.PlaintextBytes() != 0 {
		t.Fatalf("plaintext leaked while isolated: %d bytes", vif.PlaintextBytes())
	}
}

func TestChaosMaliciousChunksRejectedAndCacheBounded(t *testing.T) {
	t.Parallel()
	payload := deterministicPayload(4096)
	chunks, err := bundle.EncodeErasureChunks(payload, bundle.ChunkOptions{
		DataShards:   4,
		ParityShards: 3,
		ShardSize:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	reassembler := bundle.NewChunkReassembler(bundle.ChunkHeaderSize+512, time.Minute)

	raw, err := chunks[0].MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xaa
	if _, _, err := reassembler.Add(raw, time.Now()); !errors.Is(err, bundle.ErrChunkChecksum) {
		t.Fatalf("expected checksum rejection, got %v", err)
	}

	oversized := chunks[1]
	oversized.Data = append(oversized.Data, make([]byte, bundle.MaxChunkDataSize)...)
	oversized.ChunkSHA256 = [32]byte{}
	if _, _, err := reassembler.AddChunk(oversized, time.Now()); err == nil {
		t.Fatalf("expected oversized chunk rejection")
	}

	if _, _, err := reassembler.AddChunk(chunks[2], time.Now()); !errors.Is(err, bundle.ErrChunkCacheLimit) {
		t.Fatalf("expected cache limit rejection, got %v", err)
	}
}

func TestChaosMemoryStableAcrossReconnectStorm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping reconnect storm in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	policy := proxycore.DefaultFailoverPolicy()
	policy.ValidationTimeout = 10 * time.Millisecond
	policy.HealthTimeout = 10 * time.Millisecond
	policy.BackoffBase = 0
	ks := newChaosKillSwitch()
	engine := proxycore.NewEngine(policy, ks, nil)
	core := &chaosCore{
		name: "storm",
		ks:   ks,
		vif:  &mockVirtualInterface{},
		health: proxycore.HealthSample{
			Status:              "failed",
			Reason:              e2e.ReasonNetworkBlocked,
			TCPResetCount:       99,
			TLSTimeoutCount:     99,
			PacketDropPercent:   85,
			ConsecutiveFailures: 99,
		},
	}
	candidates := []proxycore.Candidate{{Config: proxycore.CoreConfig{ID: "storm", Protocol: "tuic"}, Core: core}}

	runtime.GC()
	before := heapAlloc()
	for i := 0; i < 500; i++ {
		_ = engine.Switch(ctx, candidates)
	}
	runtime.GC()
	after := heapAlloc()
	if after > before+8*1024*1024 {
		t.Fatalf("heap growth too high before=%d after=%d delta=%d", before, after, after-before)
	}
	if !ks.IsEngaged() {
		t.Fatalf("kill switch must remain engaged after reconnect storm")
	}
}

func TestChaosTelemetryEnumsOnlyUnderFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/telemetry.queue"
	engine, err := telemetry.NewEngine(telemetry.Config{
		Enabled:                  true,
		QueuePath:                path,
		CollectorPublicKeyB64URL: collectorPubB64(t),
		Transport:                telemetry.TransportOnion,
		Now:                      func() time.Time { return time.Unix(1_700_000_000, 0) },
		Rand:                     bytes.NewReader(bytes.Repeat([]byte{0x7f}, 512)),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range []telemetry.DiagnosticEvent{
		{Code: telemetry.EventTCPReset, CoreVersion: telemetry.CoreVersionSunLionetV1, Carrier: telemetry.CarrierUnknown},
		{Code: telemetry.EventImportSignatureInvalid, CoreVersion: telemetry.CoreVersionSunLionetV1, Carrier: telemetry.CarrierUnknown},
		{Code: telemetry.EventBLETransferInterrupted, CoreVersion: telemetry.CoreVersionSunLionetV1, Carrier: telemetry.CarrierUnknown},
	} {
		if err := engine.Record(ev); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"profile", "identity", "ssid", "imei", "TCP_RESET", "signature", "user", "route"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(forbidden)) {
			t.Fatalf("telemetry queue leaked forbidden token %q: %s", forbidden, raw)
		}
	}
}

type lossyTransport struct {
	mu        sync.Mutex
	rng       *mrand.Rand
	dropPct   int
	minJitter time.Duration
	maxJitter time.Duration
	ok        int
}

func newLossyTransport(seed uint64, dropPct int, minJitter, maxJitter time.Duration) *lossyTransport {
	return &lossyTransport{
		rng:       randv2(seed),
		dropPct:   dropPct,
		minJitter: minJitter,
		maxJitter: maxJitter,
	}
}

func (t *lossyTransport) Deliver(ctx context.Context, packet []byte) ([]byte, bool, error) {
	t.mu.Lock()
	drop := t.rng.IntN(100) < t.dropPct
	jitter := t.minJitter
	if t.maxJitter > t.minJitter {
		jitter += time.Duration(t.rng.Int64N(int64(t.maxJitter - t.minJitter)))
	}
	t.mu.Unlock()
	timer := time.NewTimer(jitter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-timer.C:
	}
	if drop {
		return nil, false, nil
	}
	t.mu.Lock()
	t.ok++
	t.mu.Unlock()
	return append([]byte(nil), packet...), true, nil
}

func (t *lossyTransport) delivered() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ok
}

type chaosCore struct {
	mu          sync.Mutex
	name        string
	ks          *chaosKillSwitch
	vif         *mockVirtualInterface
	validateErr error
	reloadErr   error
	health      proxycore.HealthSample
}

func (c *chaosCore) Name() string { return c.name }
func (c *chaosCore) PID() int     { return 42 }
func (c *chaosCore) Validate(ctx context.Context, cfg proxycore.CoreConfig) error {
	return c.waitOrErr(ctx, c.validateErr)
}
func (c *chaosCore) Start(ctx context.Context, cfg proxycore.CoreConfig) error {
	return c.HotReload(ctx, cfg)
}
func (c *chaosCore) HotReload(ctx context.Context, cfg proxycore.CoreConfig) error {
	_ = c.vif.WritePlaintext(c.ks, []byte("dns-leak-probe"))
	return c.waitOrErr(ctx, c.reloadErr)
}
func (c *chaosCore) Stop(ctx context.Context) error { return ctx.Err() }
func (c *chaosCore) CheckHealth(ctx context.Context, cfg proxycore.CoreConfig) (proxycore.HealthSample, error) {
	if err := ctx.Err(); err != nil {
		return proxycore.HealthSample{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.health.Status == "" {
		c.health = proxycore.HealthSample{Status: "ok", Reason: e2e.ReasonOK, ObservedAt: time.Now(), Passive: true}
	}
	if c.health.ObservedAt.IsZero() {
		c.health.ObservedAt = time.Now()
	}
	if c.health.Status != "ok" {
		return c.health, errors.New("chaos degraded")
	}
	return c.health, nil
}
func (c *chaosCore) waitOrErr(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Millisecond):
		return err
	}
}

type chaosKillSwitch struct {
	mu      sync.Mutex
	engaged bool
}

func newChaosKillSwitch() *chaosKillSwitch { return &chaosKillSwitch{} }
func (k *chaosKillSwitch) Engage(ctx context.Context, reason string) error {
	k.mu.Lock()
	k.engaged = true
	k.mu.Unlock()
	return ctx.Err()
}
func (k *chaosKillSwitch) Release(ctx context.Context) error {
	k.mu.Lock()
	k.engaged = false
	k.mu.Unlock()
	return ctx.Err()
}
func (k *chaosKillSwitch) IsEngaged() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.engaged
}

type mockVirtualInterface struct {
	mu        sync.Mutex
	plaintext int
}

func (v *mockVirtualInterface) WritePlaintext(ks *chaosKillSwitch, p []byte) error {
	if ks != nil && ks.IsEngaged() {
		return proxycore.ErrKillSwitchEngaged
	}
	v.mu.Lock()
	v.plaintext += len(p)
	v.mu.Unlock()
	return nil
}

func (v *mockVirtualInterface) PlaintextBytes() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.plaintext
}

func deterministicPayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((i*17 + 31) % 251)
	}
	return out
}

func heapAlloc() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

func randv2(seed uint64) *mrand.Rand {
	return mrand.New(mrand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

func collectorPubB64(t *testing.T) string {
	t.Helper()
	priv, err := ecdh.X25519().GenerateKey(crand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes())
}
