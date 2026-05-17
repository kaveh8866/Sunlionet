package proxycore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/e2e"
)

type EventSink func(event string, fields map[string]any)

type FailoverPolicy struct {
	ValidationTimeout      time.Duration
	HealthTimeout          time.Duration
	BackoffBase            time.Duration
	BackoffMax             time.Duration
	MaxConsecutiveFailures int
	MaxTCPResets           int
	MaxTLSTimeouts         int
	MaxPacketDropPercent   int
}

func DefaultFailoverPolicy() FailoverPolicy {
	return FailoverPolicy{
		ValidationTimeout:      5 * time.Second,
		HealthTimeout:          10 * time.Second,
		BackoffBase:            500 * time.Millisecond,
		BackoffMax:             8 * time.Second,
		MaxConsecutiveFailures: 2,
		MaxTCPResets:           3,
		MaxTLSTimeouts:         2,
		MaxPacketDropPercent:   40,
	}
}

type Candidate struct {
	Config     CoreConfig
	Core       ProxyCore
	Priority   int
	BeaconMode bool
}

type Attempt struct {
	ID       string
	Protocol string
	Core     string
	State    RuntimeState
	Health   HealthSample
	Error    string
}

type SwitchResult struct {
	Selected *Candidate
	State    RuntimeState
	Attempts []Attempt
	Err      error
}

type Engine struct {
	policy     FailoverPolicy
	killSwitch KillSwitch
	events     EventSink

	mu       sync.Mutex
	state    RuntimeState
	activeID string
	failures map[string]int
}

func NewEngine(policy FailoverPolicy, killSwitch KillSwitch, events EventSink) *Engine {
	if policy.ValidationTimeout == 0 {
		policy = DefaultFailoverPolicy()
	}
	if killSwitch == nil {
		killSwitch = &NoopKillSwitch{}
	}
	return &Engine{
		policy:     policy,
		killSwitch: killSwitch,
		events:     events,
		state:      StateIdle,
		failures:   map[string]int{},
	}
}

func (e *Engine) State() RuntimeState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

func (e *Engine) Switch(ctx context.Context, candidates []Candidate) SwitchResult {
	if len(candidates) == 0 {
		return SwitchResult{State: StateFailed, Err: ErrNoCandidate}
	}

	var attempts []Attempt
	var lastErr error
	for idx := range candidates {
		c := candidates[idx]
		if c.Core == nil {
			attempts = append(attempts, Attempt{ID: c.Config.ID, Protocol: c.Config.Protocol, State: StateFailed, Error: "missing proxy core"})
			continue
		}
		if delay := e.backoffFor(c.Config.ID); delay > 0 {
			if err := sleepContext(ctx, delay); err != nil {
				return SwitchResult{State: e.State(), Attempts: attempts, Err: err}
			}
		}

		e.setState(StateValidating)
		e.emit("CORE_VALIDATE_START", c)
		if err := e.validateCandidate(ctx, c); err != nil {
			lastErr = err
			e.recordFailure(c.Config.ID)
			attempts = append(attempts, Attempt{ID: c.Config.ID, Protocol: c.Config.Protocol, Core: c.Core.Name(), State: StateFailed, Error: err.Error()})
			e.emit("CORE_VALIDATE_FAIL", c)
			continue
		}
		e.emit("CORE_VALIDATE_OK", c)

		e.setState(StateIsolated)
		if err := e.killSwitch.Engage(ctx, "core transition"); err != nil {
			return SwitchResult{State: StateIsolated, Attempts: attempts, Err: err}
		}

		e.setState(StateStarting)
		e.emit("CORE_SWITCH_START", c)
		err := c.Core.HotReload(ctx, c.Config)
		if err != nil {
			lastErr = err
			_ = c.Core.Stop(ctx)
			e.recordFailure(c.Config.ID)
			attempts = append(attempts, Attempt{ID: c.Config.ID, Protocol: c.Config.Protocol, Core: c.Core.Name(), State: StateFailed, Error: err.Error()})
			e.emit("CORE_SWITCH_FAIL", c)
			continue
		}

		health, err := e.checkHealth(ctx, c)
		if err != nil || IsDegraded(health, e.policy) {
			_ = c.Core.Stop(ctx)
			e.recordFailure(c.Config.ID)
			if err == nil {
				err = errors.New("degraded link")
			}
			lastErr = err
			attempts = append(attempts, Attempt{ID: c.Config.ID, Protocol: c.Config.Protocol, Core: c.Core.Name(), State: StateDegraded, Health: health, Error: err.Error()})
			e.emit("CORE_HEALTH_DEGRADED", c)
			continue
		}

		if err := e.killSwitch.Release(ctx); err != nil {
			_ = c.Core.Stop(ctx)
			return SwitchResult{State: StateIsolated, Attempts: attempts, Err: err}
		}
		e.clearFailure(c.Config.ID)
		e.setActive(c.Config.ID, StateRunning)
		attempts = append(attempts, Attempt{ID: c.Config.ID, Protocol: c.Config.Protocol, Core: c.Core.Name(), State: StateRunning, Health: health})
		e.emit("CORE_SWITCH_OK", c)
		return SwitchResult{Selected: &c, State: StateRunning, Attempts: attempts}
	}

	_ = e.killSwitch.Engage(ctx, "all proxy cores failed")
	e.setState(StateBeacon)
	if lastErr != nil {
		return SwitchResult{State: StateBeacon, Attempts: attempts, Err: errors.Join(ErrAllCandidatesFailed, lastErr)}
	}
	return SwitchResult{State: StateBeacon, Attempts: attempts, Err: ErrAllCandidatesFailed}
}

func (e *Engine) validateCandidate(ctx context.Context, c Candidate) error {
	timeout := e.policy.ValidationTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	vctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- c.Core.Validate(vctx, c.Config)
	}()
	select {
	case <-vctx.Done():
		return vctx.Err()
	case err := <-done:
		return err
	}
}

func (e *Engine) checkHealth(ctx context.Context, c Candidate) (HealthSample, error) {
	timeout := e.policy.HealthTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Core.CheckHealth(hctx, c.Config)
}

func IsDegraded(sample HealthSample, policy FailoverPolicy) bool {
	if sample.Status != "" && sample.Status != "ok" {
		return true
	}
	if sample.Reason != "" && sample.Reason != e2e.ReasonOK {
		return true
	}
	if policy.MaxConsecutiveFailures > 0 && sample.ConsecutiveFailures >= policy.MaxConsecutiveFailures {
		return true
	}
	if policy.MaxTCPResets > 0 && sample.TCPResetCount >= policy.MaxTCPResets {
		return true
	}
	if policy.MaxTLSTimeouts > 0 && sample.TLSTimeoutCount >= policy.MaxTLSTimeouts {
		return true
	}
	if policy.MaxPacketDropPercent > 0 && sample.PacketDropPercent >= policy.MaxPacketDropPercent {
		return true
	}
	return false
}

func (e *Engine) backoffFor(id string) time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := e.failures[id]
	if n <= 0 || e.policy.BackoffBase <= 0 {
		return 0
	}
	delay := e.policy.BackoffBase << min(n-1, 6)
	if e.policy.BackoffMax > 0 && delay > e.policy.BackoffMax {
		return e.policy.BackoffMax
	}
	return delay
}

func (e *Engine) recordFailure(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failures[id]++
}

func (e *Engine) clearFailure(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.failures, id)
}

func (e *Engine) setState(state RuntimeState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = state
}

func (e *Engine) setActive(id string, state RuntimeState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.activeID = id
	e.state = state
}

func (e *Engine) emit(event string, c Candidate) {
	if e.events == nil {
		return
	}
	e.events(event, map[string]any{
		"profile":  c.Config.ID,
		"protocol": c.Config.Protocol,
		"core":     c.Core.Name(),
	})
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type NetworkHealthMonitor struct {
	policy FailoverPolicy
	in     chan HealthSample
	out    chan HealthSample
	done   chan struct{}
}

func NewNetworkHealthMonitor(policy FailoverPolicy, buffer int) *NetworkHealthMonitor {
	if policy.ValidationTimeout == 0 {
		policy = DefaultFailoverPolicy()
	}
	if buffer <= 0 {
		buffer = 16
	}
	return &NetworkHealthMonitor{
		policy: policy,
		in:     make(chan HealthSample, buffer),
		out:    make(chan HealthSample, buffer),
		done:   make(chan struct{}),
	}
}

func (m *NetworkHealthMonitor) Record(sample HealthSample) {
	if m == nil {
		return
	}
	if sample.ObservedAt.IsZero() {
		sample.ObservedAt = time.Now()
	}
	select {
	case m.in <- sample:
	default:
	}
}

func (m *NetworkHealthMonitor) Degraded() <-chan HealthSample {
	if m == nil {
		return nil
	}
	return m.out
}

func (m *NetworkHealthMonitor) Run(ctx context.Context) {
	if m == nil {
		return
	}
	defer close(m.done)
	for {
		select {
		case <-ctx.Done():
			return
		case sample := <-m.in:
			if IsDegraded(sample, m.policy) {
				select {
				case m.out <- sample:
				default:
				}
			}
		}
	}
}
