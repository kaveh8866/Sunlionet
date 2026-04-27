package dpisim

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
)

func requireLoopback(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback not available: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	errCh := make(chan error, 1)
	go func() {
		c, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = c.Close()
		}
		errCh <- err
	}()
	go func() {
		c, err := ln.Accept()
		if err == nil && c != nil {
			_ = c.Close()
		}
	}()
	if err := <-errCh; err != nil {
		t.Skipf("loopback dial blocked: %v", err)
	}
}

func TestSimulation_HTTP403_InjectionDetected(t *testing.T) {
	requireLoopback(t)
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
	requireLoopback(t)
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
