package simulation

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/tests/simulation/dpisim"
)

type fakeResolver struct {
	ips []net.IPAddr
	err error
}

func (r *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return r.ips, r.err
}

func TestCensorshipSimulationScenarioMatrix_64Scenarios(t *testing.T) {
	http403 := dpisim.StartHTTP403Injector()
	defer http403.Close()

	httpOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer httpOK.Close()

	tlsDrop, err := dpisim.StartTLSDropper()
	if err != nil {
		t.Fatalf("tls dropper: %v", err)
	}
	defer tlsDrop.Close()

	tlsOK, err := dpisim.StartTLSServer()
	if err != nil {
		t.Fatalf("tls server: %v", err)
	}
	defer tlsOK.Close()

	udpEcho, err := dpisim.StartUDPEcho()
	if err != nil {
		t.Fatalf("udp echo: %v", err)
	}
	defer udpEcho.Close()

	udpSink, err := dpisim.StartUDPSink()
	if err != nil {
		t.Fatalf("udp sink: %v", err)
	}
	defer udpSink.Close()

	baselineLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("baseline listen: %v", err)
	}
	defer baselineLn.Close()
	go func() {
		for {
			c, err := baselineLn.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	makeUnusedTCPAddr := func() string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "127.0.0.1:1"
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		return addr
	}

	type scenario struct {
		name         string
		dnsPoison    bool
		sniReset     bool
		httpInject   bool
		udpDrop      bool
		baselineFail bool
		transport    string
	}

	scenarios := make([]scenario, 0, 64)
	transports := []string{"tcp", "udp"}
	for _, transport := range transports {
		for mask := 0; mask < 32; mask++ {
			scenarios = append(scenarios, scenario{
				name:         fmt.Sprintf("%s-mask-%02d", transport, mask),
				dnsPoison:    mask&(1<<0) != 0,
				sniReset:     mask&(1<<1) != 0,
				httpInject:   mask&(1<<2) != 0,
				udpDrop:      mask&(1<<3) != 0,
				baselineFail: mask&(1<<4) != 0,
				transport:    transport,
			})
		}
	}

	e := &policy.Engine{}
	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()

			resolverIP := "8.8.8.8"
			if tc.dnsPoison {
				resolverIP = "10.10.34.34"
			}

			tlsAddr := tlsOK.Addr().String()
			if tc.sniReset {
				tlsAddr = tlsDrop.Addr().String()
			}

			httpURL := httpOK.URL
			httpClient := httpOK.Client()
			if tc.httpInject {
				httpURL = http403.URL
				httpClient = http403.Client()
			}

			udpAddr := udpEcho.LocalAddr().String()
			if tc.udpDrop {
				udpAddr = udpSink.LocalAddr().String()
			}

			baselineAddr := baselineLn.Addr().String()
			if tc.baselineFail {
				baselineAddr = makeUnusedTCPAddr()
			}

			poisoned, _ := detector.CheckDNSPoisoningWith(ctx, &fakeResolver{ips: []net.IPAddr{{IP: net.ParseIP(resolverIP)}}}, "twitter.com")
			sniBlocked, _ := detector.CheckSNIResetWith(ctx, &net.Dialer{}, "localhost", tlsAddr)
			httpFiltered, _ := detector.CheckHTTPFilteringWith(ctx, httpClient, httpURL)
			udpBlocked, _ := detector.CheckUDPBlockedWith(ctx, &net.Dialer{}, udpAddr)
			baselineOK, _ := detector.CheckConnectivityBaselineWith(ctx, &net.Dialer{}, baselineAddr)

			var events []detector.Event
			if !baselineOK {
				events = append(events, detector.Event{Type: detector.EventThroughputCollapse, Severity: detector.SeverityHigh, Timestamp: time.Now()})
			}
			if httpFiltered {
				events = append(events, detector.Event{Type: detector.EventActiveResetSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()})
			}
			if sniBlocked {
				events = append(events, detector.Event{Type: detector.EventSNIBlockSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()})
			}
			if udpBlocked {
				events = append(events, detector.Event{Type: detector.EventUDPBlockSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()})
			}
			if poisoned {
				events = append(events, detector.Event{Type: detector.EventDNSPoisonSuspected, Severity: detector.SeverityCritical, Timestamp: time.Now()})
			}

			act := e.Evaluate(events, profile.Profile{Capabilities: profile.Capabilities{Transport: tc.transport}})

			var want policy.ActionType
			switch {
			case poisoned:
				want = policy.ActionEnterEmergencyDNS
			case udpBlocked && tc.transport == "udp":
				want = policy.ActionSwitchFamily
			case len(events) == 0:
				want = policy.ActionKeep
			default:
				want = policy.ActionInvokeLLM
			}

			if act.Type != want {
				t.Fatalf("want %s got %s (poison=%v sni=%v http=%v udp=%v baselineOK=%v)", want, act.Type, poisoned, sniBlocked, httpFiltered, udpBlocked, baselineOK)
			}
		})
	}
}
