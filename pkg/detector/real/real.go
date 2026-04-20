package real

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/orchestrator"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type Config struct {
	BaselineAddr string
	DNSDomain    string
	UDPEchoAddr  string
}

type RealDetector struct {
	cfg      Config
	resolver detector.Resolver
	dialer   detector.Dialer
}

func New(cfg Config) *RealDetector {
	if cfg.BaselineAddr == "" {
		cfg.BaselineAddr = "wikipedia.org:443"
	}
	if cfg.DNSDomain == "" {
		cfg.DNSDomain = "twitter.com"
	}
	if cfg.UDPEchoAddr == "" {
		cfg.UDPEchoAddr = "1.1.1.1:53"
	}
	return &RealDetector{
		cfg:      cfg,
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{},
	}
}

func (d *RealDetector) Analyze(ctx context.Context) (orchestrator.NetworkState, error) {
	if d.cfg.BaselineAddr == "" || d.cfg.DNSDomain == "" || d.cfg.UDPEchoAddr == "" {
		return orchestrator.NetworkState{}, fmt.Errorf("detector: no real signal available")
	}

	start := time.Now()
	conn, err := d.dialer.DialContext(ctx, "tcp", d.cfg.BaselineAddr)
	if err != nil {
		return orchestrator.NetworkState{}, fmt.Errorf("detector: baseline connectivity failed: %w", err)
	}
	_ = conn.Close()
	latencyMS := int(time.Since(start) / time.Millisecond)

	dnsPoisoned, err := detector.CheckDNSPoisoningWith(ctx, d.resolver, d.cfg.DNSDomain)
	if err != nil {
		return orchestrator.NetworkState{}, fmt.Errorf("detector: dns check failed: %w", err)
	}

	udpBlocked, err := detector.CheckUDPBlockedWith(ctx, d.dialer, d.cfg.UDPEchoAddr)
	if err != nil {
		return orchestrator.NetworkState{}, fmt.Errorf("detector: udp check failed: %w", err)
	}

	return orchestrator.NetworkState{
		UDPBlocked:   udpBlocked,
		DNSPoisoning: dnsPoisoned,
		LatencyMS:    latencyMS,
	}, nil
}

func (d *RealDetector) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeReal
}
