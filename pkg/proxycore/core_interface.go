package proxycore

import (
	"context"
	"errors"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/e2e"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
)

var (
	ErrNoCandidate         = errors.New("proxycore: no candidate")
	ErrAllCandidatesFailed = errors.New("proxycore: all candidates failed")
	ErrKillSwitchEngaged   = errors.New("proxycore: kill switch engaged")
)

type RuntimeState string

const (
	StateIdle       RuntimeState = "idle"
	StateIsolated   RuntimeState = "isolated"
	StateValidating RuntimeState = "validating"
	StateStarting   RuntimeState = "starting"
	StateRunning    RuntimeState = "running"
	StateDegraded   RuntimeState = "degraded"
	StateBeacon     RuntimeState = "beacon"
	StateFailed     RuntimeState = "failed"
)

type CoreConfig struct {
	ID             string
	Protocol       string
	ConfigPath     string
	ConfigBytes    []byte
	ProbeProxyURL  string
	ProbeTargetURL string
}

type HealthSample struct {
	Status              string
	Reason              e2e.FailureReason
	LatencyMS           int64
	ConsecutiveFailures int
	TCPResetCount       int
	TLSTimeoutCount     int
	PacketDropPercent   int
	ObservedAt          time.Time
	Passive             bool
	Error               string
}

type ProxyCore interface {
	Name() string
	PID() int
	Validate(ctx context.Context, cfg CoreConfig) error
	Start(ctx context.Context, cfg CoreConfig) error
	HotReload(ctx context.Context, cfg CoreConfig) error
	Stop(ctx context.Context) error
	CheckHealth(ctx context.Context, cfg CoreConfig) (HealthSample, error)
}

type KillSwitch interface {
	Engage(ctx context.Context, reason string) error
	Release(ctx context.Context) error
	IsEngaged() bool
}

type NoopKillSwitch struct {
	engaged bool
}

func (k *NoopKillSwitch) Engage(ctx context.Context, reason string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		k.engaged = true
		return nil
	}
}

func (k *NoopKillSwitch) Release(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		k.engaged = false
		return nil
	}
}

func (k *NoopKillSwitch) IsEngaged() bool {
	if k == nil {
		return false
	}
	return k.engaged
}

type SingBoxCore struct {
	Controller *sbctl.Controller
}

func NewSingBoxCore(ctrl *sbctl.Controller) *SingBoxCore {
	return &SingBoxCore{Controller: ctrl}
}

func (c *SingBoxCore) Name() string {
	return "sing-box"
}

func (c *SingBoxCore) PID() int {
	if c == nil || c.Controller == nil {
		return 0
	}
	return c.Controller.PID()
}

func (c *SingBoxCore) Validate(ctx context.Context, cfg CoreConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.Controller == nil {
		return errors.New("proxycore: missing sing-box controller")
	}
	return c.Controller.ValidateConfig(cfg.ConfigPath)
}

func (c *SingBoxCore) Start(ctx context.Context, cfg CoreConfig) error {
	return c.HotReload(ctx, cfg)
}

func (c *SingBoxCore) HotReload(ctx context.Context, cfg CoreConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.Controller == nil {
		return errors.New("proxycore: missing sing-box controller")
	}
	return c.Controller.ApplyAndReload(string(cfg.ConfigBytes))
}

func (c *SingBoxCore) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.Controller == nil {
		return nil
	}
	return c.Controller.Stop()
}

func (c *SingBoxCore) CheckHealth(ctx context.Context, cfg CoreConfig) (HealthSample, error) {
	if cfg.ProbeTargetURL == "" {
		return HealthSample{
			Status:     "ok",
			Reason:     e2e.ReasonOK,
			ObservedAt: time.Now(),
			Passive:    true,
		}, nil
	}
	res := e2e.HTTPProxyProbe(ctx, cfg.ProbeProxyURL, cfg.ProbeTargetURL)
	sample := HealthSample{
		Status:     res.Status,
		Reason:     res.Reason,
		LatencyMS:  res.DurationMS,
		ObservedAt: time.Unix(res.ObservedAt, 0),
		Passive:    false,
		Error:      res.Error,
	}
	if res.Status != "ok" {
		return sample, errors.New(res.Error)
	}
	return sample, nil
}
