package dpisim

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
)

func TestSimulation_HTTP403_InjectionDetected(t *testing.T) {
	srv := StartHTTP403Injector()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := detector.CheckHTTPFilteringWith(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked")
	}
}

func TestSimulation_TLSResetLikeDropDetected(t *testing.T) {
	ln, err := StartTLSDropper()
	if err != nil {
		t.Fatalf("start dropper: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := detector.CheckSNIResetWith(ctx, &net.Dialer{}, "blocked.example", ln.Addr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked")
	}
}

func TestSimulation_UDPEcho_NotBlocked(t *testing.T) {
	pc, err := StartUDPEcho()
	if err != nil {
		t.Fatalf("udp echo: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := detector.CheckUDPBlockedWith(ctx, &net.Dialer{}, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatalf("expected not blocked")
	}
}
