package simulation

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/tests/simulation/dpisim"
)

func TestSimulation_UDPLatencySpike_NotTreatedAsBlocked(t *testing.T) {
	pc, err := dpisim.StartUDPEchoProfile(1500*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("udp echo: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := detector.CheckUDPBlockedWith(ctx, &net.Dialer{}, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("udp check: %v", err)
	}
	if blocked {
		t.Fatalf("expected not blocked under latency spike")
	}
}

func TestSimulation_UDPPacketLoss_TreatedAsBlocked(t *testing.T) {
	pc, err := dpisim.StartUDPEchoProfile(0, 1)
	if err != nil {
		t.Fatalf("udp echo: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	blocked, err := detector.CheckUDPBlockedWith(ctx, &net.Dialer{}, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("udp check: %v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked under packet loss")
	}
}
