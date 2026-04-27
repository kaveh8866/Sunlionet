package simulation

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/policy"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/tests/simulation/dpisim"
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

func TestAdversarial_DNSFiltering_TriggersEmergencyDNS(t *testing.T) {
	dnsMgr := &dpisim.DNSManager{Poisoned: true}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	poisoned, err := detector.CheckDNSPoisoningWith(ctx, dnsMgr, "twitter.com")
	if err != nil {
		t.Fatalf("dns check: %v", err)
	}
	if !poisoned {
		t.Fatalf("expected poisoned")
	}

	engine := &policy.Engine{}
	events := []detector.Event{
		{
			Type:      detector.EventDNSPoisonSuspected,
			Severity:  detector.SeverityCritical,
			Timestamp: time.Now(),
		},
	}

	action := engine.Evaluate(events, profile.Profile{ID: "current"})

	if action.Type != policy.ActionEnterEmergencyDNS {
		t.Errorf("Expected ActionEnterEmergencyDNS, got %s", action.Type)
	}
}

func TestAdversarial_UDPBlocking_TriggersFamilySwitch(t *testing.T) {
	pc, err := dpisim.StartUDPSink()
	if err != nil {
		t.Fatalf("udp sink: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	blocked, err := detector.CheckUDPBlockedWith(ctx, nil, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("udp check: %v", err)
	}
	if !blocked {
		t.Fatalf("expected UDP to be blocked")
	}

	engine := &policy.Engine{}
	activeProfile := profile.Profile{
		ID: "hysteria2-udp",
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
	}

	events := []detector.Event{
		{
			Type:      detector.EventUDPBlockSuspected,
			Severity:  detector.SeverityHigh,
			Timestamp: time.Now(),
		},
	}

	action := engine.Evaluate(events, activeProfile)

	if action.Type != policy.ActionSwitchFamily {
		t.Errorf("Expected ActionSwitchFamily, got %s", action.Type)
	}
}

func TestAdversarial_SNIReset_TriggersAnomalyEvent(t *testing.T) {
	requireLoopback(t)
	ln, err := dpisim.StartTLSDropper()
	if err != nil {
		t.Fatalf("tls dropper: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := detector.CheckSNIResetWith(ctx, &net.Dialer{}, "blocked.com", ln.Addr().String())
	if err != nil {
		t.Fatalf("sni check: %v", err)
	}
	if !blocked {
		t.Fatalf("expected SNI reset")
	}

	engine := &policy.Engine{}
	events := []detector.Event{
		{
			Type:      detector.EventSNIBlockSuspected,
			Severity:  detector.SeverityHigh,
			Timestamp: time.Now(),
		},
	}

	action := engine.Evaluate(events, profile.Profile{ID: "current"})
	if action.Type != policy.ActionInvokeLLM {
		t.Errorf("Expected ActionInvokeLLM for SNI block (ambiguous), got %s", action.Type)
	}
}
