package multipath_router

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/ble_transport"
	"github.com/kaveh/sunlionet-agent/core/transport/bridge_node_logic"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
	"github.com/kaveh/sunlionet-agent/core/transport/wifi_direct_transport"
)

func baseRouterOpts() message_router.Options {
	return message_router.Options{
		MaxFrameBytes:        120,
		MaxHop:               12,
		MinRebroadcastEvery:  1 * time.Millisecond,
		ReassemblyStaleAfter: 2 * time.Second,
		CacheMaxItems:        4096,
		ClockSkewAllowance:   10 * time.Second,
		GlobalRateCapacity:   10000,
		GlobalRate:           10000,
		PerSenderCapacity:    10000,
		PerSenderRate:        10000,
		GossipSeed:           1,
		RoutingBaseProb:      1.0,
		RoutingMaxJitter:     0,
		InventoryEvery:       40 * time.Millisecond,
		InventoryMax:         32,
		WantMax:              16,
	}
}

func TestHybrid_ScenarioA_BLEUnavailable_WiFiFailover(t *testing.T) {
	net := bridge_node_logic.NewSimNet()
	m0, _ := identity_manager.New(200 * time.Millisecond)
	m1, _ := identity_manager.New(200 * time.Millisecond)
	id0 := m0.Current(time.Now()).NodeID
	id1 := m1.Current(time.Now()).NodeID

	ble0 := ble_transport.NewSim(net, id0)
	ble1 := ble_transport.NewSim(net, id1)
	w0 := wifi_direct_transport.NewSim(net, id0)
	w1 := wifi_direct_transport.NewSim(net, id1)

	net.Connect(bridge_node_logic.MediumWiFi, id0, id1)
	net.Connect(bridge_node_logic.MediumBLE, id0, id1)

	ble0.SetAvailable(false)
	ble1.SetAvailable(false)

	n0, err := NewNode(m0, []transport_manager.Transport{ble0, w0}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})
	if err != nil {
		t.Fatal(err)
	}
	n1, err := NewNode(m1, []transport_manager.Transport{ble1, w1}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})
	if err != nil {
		t.Fatal(err)
	}

	var got atomic.Int64
	n1.Router.SetOnMessage(func(m message_router.Message) { got.Add(1) })

	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	go func() { _ = n0.Run(ctx) }()
	go func() { _ = n1.Run(ctx) }()

	_, _ = n0.Send(time.Now(), []byte("hi"), 2, transport_selector.PreferenceAny)
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected delivery with BLE unavailable (wifi failover)")
	}
	if w0.TxCount() == 0 && w1.TxCount() == 0 {
		t.Fatalf("expected wifi transport activity")
	}
}

func TestHybrid_ScenarioB_BLEWiFiMixedHops(t *testing.T) {
	net := bridge_node_logic.NewSimNet()
	ms := make([]*identity_manager.Manager, 5)
	ids := make([]identity_manager.NodeID, 5)
	for i := range ms {
		m, _ := identity_manager.New(200 * time.Millisecond)
		ms[i] = m
		ids[i] = m.Current(time.Now()).NodeID
	}

	ble0 := ble_transport.NewSim(net, ids[0])
	ble1 := ble_transport.NewSim(net, ids[1])
	w1 := wifi_direct_transport.NewSim(net, ids[1])
	w2 := wifi_direct_transport.NewSim(net, ids[2])
	ble3 := ble_transport.NewSim(net, ids[3])
	w3 := wifi_direct_transport.NewSim(net, ids[3])
	ble4 := ble_transport.NewSim(net, ids[4])

	net.Connect(bridge_node_logic.MediumBLE, ids[0], ids[1])
	net.Connect(bridge_node_logic.MediumWiFi, ids[1], ids[2])
	net.Connect(bridge_node_logic.MediumWiFi, ids[2], ids[3])
	net.Connect(bridge_node_logic.MediumBLE, ids[3], ids[4])

	n0, _ := NewNode(ms[0], []transport_manager.Transport{ble0}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})
	n1, _ := NewNode(ms[1], []transport_manager.Transport{ble1, w1}, Options{Profile: transport_selector.ProfileBalanced, Multipath: true, Router: baseRouterOpts()})
	n2, _ := NewNode(ms[2], []transport_manager.Transport{w2}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})
	n3, _ := NewNode(ms[3], []transport_manager.Transport{w3, ble3}, Options{Profile: transport_selector.ProfileBalanced, Multipath: true, Router: baseRouterOpts()})
	n4, _ := NewNode(ms[4], []transport_manager.Transport{ble4}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})

	var got atomic.Int64
	n4.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "mixed" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()
	go func() { _ = n0.Run(ctx) }()
	go func() { _ = n1.Run(ctx) }()
	go func() { _ = n2.Run(ctx) }()
	go func() { _ = n3.Run(ctx) }()
	go func() { _ = n4.Run(ctx) }()

	_, err := n0.Send(time.Now(), []byte("mixed"), 3, transport_selector.PreferenceAny)
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected message to cross BLE and Wi-Fi hops")
	}
}

func TestHybrid_ScenarioC_BridgeNodeRepairsFragmentation(t *testing.T) {
	net := bridge_node_logic.NewSimNet()
	mA, _ := identity_manager.New(200 * time.Millisecond)
	mB, _ := identity_manager.New(200 * time.Millisecond)
	mBridge, _ := identity_manager.New(200 * time.Millisecond)
	mC, _ := identity_manager.New(200 * time.Millisecond)
	idA := mA.Current(time.Now()).NodeID
	idB := mB.Current(time.Now()).NodeID
	idX := mBridge.Current(time.Now()).NodeID
	idC := mC.Current(time.Now()).NodeID

	bleA := ble_transport.NewSim(net, idA)
	bleB := ble_transport.NewSim(net, idB)
	bleX := ble_transport.NewSim(net, idX)
	wX := wifi_direct_transport.NewSim(net, idX)
	wC := wifi_direct_transport.NewSim(net, idC)

	net.Connect(bridge_node_logic.MediumBLE, idA, idB)
	net.Connect(bridge_node_logic.MediumBLE, idB, idX)
	net.Connect(bridge_node_logic.MediumWiFi, idX, idC)

	nA, _ := NewNode(mA, []transport_manager.Transport{bleA}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})
	nB, _ := NewNode(mB, []transport_manager.Transport{bleB}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})
	nX, _ := NewNode(mBridge, []transport_manager.Transport{bleX, wX}, Options{Profile: transport_selector.ProfileBalanced, Multipath: true, Router: baseRouterOpts()})
	nC, _ := NewNode(mC, []transport_manager.Transport{wC}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})

	var got atomic.Int64
	nC.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "bridge" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()
	go func() { _ = nA.Run(ctx) }()
	go func() { _ = nB.Run(ctx) }()
	go func() { _ = nX.Run(ctx) }()
	go func() { _ = nC.Run(ctx) }()

	_, _ = nA.Send(time.Now(), []byte("bridge"), 3, transport_selector.PreferenceAny)
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected message to reach wifi-only cluster via bridge node")
	}
}

func TestHybrid_ScenarioD_StealthVsPerformanceBehavior(t *testing.T) {
	m0, _ := identity_manager.New(200 * time.Millisecond)
	m1, _ := identity_manager.New(200 * time.Millisecond)
	id0 := m0.Current(time.Now()).NodeID
	id1 := m1.Current(time.Now()).NodeID

	netS := bridge_node_logic.NewSimNet()

	ble0 := ble_transport.NewSim(netS, id0)
	ble1 := ble_transport.NewSim(netS, id1)
	w0 := wifi_direct_transport.NewSim(netS, id0)
	w1 := wifi_direct_transport.NewSim(netS, id1)

	netS.Connect(bridge_node_logic.MediumBLE, id0, id1)
	netS.Connect(bridge_node_logic.MediumWiFi, id0, id1)

	n0s, _ := NewNode(m0, []transport_manager.Transport{ble0, w0}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})
	n1s, _ := NewNode(m1, []transport_manager.Transport{ble1, w1}, Options{Profile: transport_selector.ProfileStealth, Router: baseRouterOpts()})

	ctxS, cancelS := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancelS()
	go func() { _ = n0s.Run(ctxS) }()
	go func() { _ = n1s.Run(ctxS) }()
	_, _ = n0s.Send(time.Now(), []byte("stealth"), 2, transport_selector.PreferenceAny)
	<-ctxS.Done()

	stealthBle := ble0.TxCount() + ble1.TxCount()
	stealthWiFi := w0.TxCount() + w1.TxCount()
	if stealthBle == 0 || stealthWiFi != 0 {
		t.Fatalf("expected stealth mode to use BLE only (ble=%d wifi=%d)", stealthBle, stealthWiFi)
	}

	netP := bridge_node_logic.NewSimNet()
	ble0p := ble_transport.NewSim(netP, id0)
	ble1p := ble_transport.NewSim(netP, id1)
	w0p := wifi_direct_transport.NewSim(netP, id0)
	w1p := wifi_direct_transport.NewSim(netP, id1)

	netP.Connect(bridge_node_logic.MediumBLE, id0, id1)
	netP.Connect(bridge_node_logic.MediumWiFi, id0, id1)

	n0p, _ := NewNode(m0, []transport_manager.Transport{ble0p, w0p}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})
	n1p, _ := NewNode(m1, []transport_manager.Transport{ble1p, w1p}, Options{Profile: transport_selector.ProfilePerformance, Router: baseRouterOpts()})

	ctxP, cancelP := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancelP()
	go func() { _ = n0p.Run(ctxP) }()
	go func() { _ = n1p.Run(ctxP) }()
	_, _ = n0p.Send(time.Now(), []byte("perf"), 2, transport_selector.PreferenceAny)
	<-ctxP.Done()

	perfBle := ble0p.TxCount() + ble1p.TxCount()
	perfWiFi := w0p.TxCount() + w1p.TxCount()
	if perfWiFi == 0 || perfBle != 0 {
		t.Fatalf("expected performance mode to prefer Wi-Fi only (ble=%d wifi=%d)", perfBle, perfWiFi)
	}
}
