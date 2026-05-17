package proxycore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/e2e"
)

type fakeCore struct {
	name        string
	validateErr error
	reloadErr   error
	health      HealthSample
	started     int
	stopped     int
}

func (f *fakeCore) Name() string { return f.name }
func (f *fakeCore) PID() int     { return 7 }
func (f *fakeCore) Validate(ctx context.Context, cfg CoreConfig) error {
	return f.validateErr
}
func (f *fakeCore) Start(ctx context.Context, cfg CoreConfig) error {
	return f.HotReload(ctx, cfg)
}
func (f *fakeCore) HotReload(ctx context.Context, cfg CoreConfig) error {
	if f.reloadErr != nil {
		return f.reloadErr
	}
	f.started++
	return nil
}
func (f *fakeCore) Stop(ctx context.Context) error {
	f.stopped++
	return nil
}
func (f *fakeCore) CheckHealth(ctx context.Context, cfg CoreConfig) (HealthSample, error) {
	if f.health.Status == "" {
		f.health = HealthSample{Status: "ok", Reason: e2e.ReasonOK, ObservedAt: time.Now(), Passive: true}
	}
	if f.health.Status != "ok" {
		return f.health, errors.New("unhealthy")
	}
	return f.health, nil
}

func TestEngineFailoverSkipsBlockedCandidate(t *testing.T) {
	bad := &fakeCore{name: "bad", reloadErr: errors.New("blocked")}
	good := &fakeCore{name: "good"}
	ks := &NoopKillSwitch{}
	engine := NewEngine(DefaultFailoverPolicy(), ks, nil)

	res := engine.Switch(context.Background(), []Candidate{
		{Config: CoreConfig{ID: "a", Protocol: "reality"}, Core: bad},
		{Config: CoreConfig{ID: "b", Protocol: "hysteria2"}, Core: good},
	})
	if res.Err != nil {
		t.Fatalf("Switch: %v", res.Err)
	}
	if res.Selected == nil || res.Selected.Config.ID != "b" {
		t.Fatalf("expected candidate b, got %#v", res.Selected)
	}
	if !ks.IsEngaged() && engine.State() != StateRunning {
		t.Fatalf("expected running state")
	}
	if bad.stopped == 0 {
		t.Fatalf("expected failed core to be stopped")
	}
}

func TestEngineBeaconModeKeepsKillSwitchEngaged(t *testing.T) {
	core := &fakeCore{name: "bad", health: HealthSample{Status: "failed", Reason: e2e.ReasonNetworkBlocked}}
	ks := &NoopKillSwitch{}
	engine := NewEngine(DefaultFailoverPolicy(), ks, nil)

	res := engine.Switch(context.Background(), []Candidate{{Config: CoreConfig{ID: "a", Protocol: "reality"}, Core: core}})
	if !errors.Is(res.Err, ErrAllCandidatesFailed) {
		t.Fatalf("expected ErrAllCandidatesFailed, got %v", res.Err)
	}
	if res.State != StateBeacon {
		t.Fatalf("expected beacon mode, got %s", res.State)
	}
	if !ks.IsEngaged() {
		t.Fatalf("kill switch must remain engaged in beacon mode")
	}
}

func TestNetworkHealthMonitorEmitsPassiveDegradation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon := NewNetworkHealthMonitor(DefaultFailoverPolicy(), 2)
	go mon.Run(ctx)

	mon.Record(HealthSample{Status: "ok", Reason: e2e.ReasonOK, Passive: true})
	mon.Record(HealthSample{Status: "ok", Reason: e2e.ReasonOK, TCPResetCount: 4, Passive: true})

	select {
	case sample := <-mon.Degraded():
		if sample.TCPResetCount != 4 {
			t.Fatalf("unexpected sample: %#v", sample)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected degraded sample")
	}
}
