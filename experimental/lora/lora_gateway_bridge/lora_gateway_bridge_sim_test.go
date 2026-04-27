package lora_gateway_bridge

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/bridge_node_logic"
	"github.com/kaveh/sunlionet-agent/core/transport/multipath_router"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
	"github.com/kaveh/sunlionet-agent/experimental/lora/lora_transport"
)

func loraRouterOpts() message_router.Options {
	return message_router.Options{
		MaxFrameBytes:        60,
		MaxHop:               18,
		MinRebroadcastEvery:  2 * time.Millisecond,
		ReassemblyStaleAfter: 5 * time.Second,
		CacheMaxItems:        8192,
		ClockSkewAllowance:   20 * time.Second,
		GlobalRateCapacity:   4000,
		GlobalRate:           4000,
		PerSenderCapacity:    4000,
		PerSenderRate:        4000,
		GossipSeed:           3,
		RoutingBaseProb:      1.0,
		RoutingMaxJitter:     0,
		InventoryEvery:       40 * time.Millisecond,
		InventoryMax:         64,
		WantMax:              32,
	}
}

func TestLoRa_ScenarioA_TwoBLEClustersBridgedByLoRaRelay(t *testing.T) {
	bleNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	ma, _ := identity_manager.New(200 * time.Millisecond)
	mb, _ := identity_manager.New(200 * time.Millisecond)
	mc, _ := identity_manager.New(200 * time.Millisecond)
	md, _ := identity_manager.New(200 * time.Millisecond)

	now := time.Now()
	idA := ma.Current(now).NodeID
	idB := mb.Current(now).NodeID
	idC := mc.Current(now).NodeID
	idD := md.Current(now).NodeID

	bleA := bleNet.Register(bridge_node_logic.MediumBLE, idA, 0)
	bleB := bleNet.Register(bridge_node_logic.MediumBLE, idB, 0)
	bleC := bleNet.Register(bridge_node_logic.MediumBLE, idC, 0)
	bleD := bleNet.Register(bridge_node_logic.MediumBLE, idD, 0)
	bleA.SetAvailable(true)
	bleB.SetAvailable(true)
	bleC.SetAvailable(true)
	bleD.SetAvailable(true)

	loraB := loraNet.Register(idB, 0)
	loraC := loraNet.Register(idC, 0)
	loraB.Configure(96, 40)
	loraC.Configure(96, 40)
	loraB.SetAvailable(true)
	loraC.SetAvailable(true)

	bleNet.Connect(bridge_node_logic.MediumBLE, idA, idB)
	bleNet.Connect(bridge_node_logic.MediumBLE, idC, idD)
	loraNet.Connect(idB, idC)

	nA, _ := multipath_router.NewNode(ma, []transport_manager.Transport{bleA}, multipath_router.Options{
		Profile: transport_selector.ProfileStealth,
		Router:  loraRouterOpts(),
	})
	nB, _ := multipath_router.NewNode(mb, []transport_manager.Transport{bleB, loraB}, multipath_router.Options{
		Profile:   transport_selector.ProfileBalanced,
		Multipath: true,
		Router:    loraRouterOpts(),
	})
	nC, _ := multipath_router.NewNode(mc, []transport_manager.Transport{bleC, loraC}, multipath_router.Options{
		Profile:   transport_selector.ProfileBalanced,
		Multipath: true,
		Router:    loraRouterOpts(),
	})
	nD, _ := multipath_router.NewNode(md, []transport_manager.Transport{bleD}, multipath_router.Options{
		Profile: transport_selector.ProfileStealth,
		Router:  loraRouterOpts(),
	})

	var got atomic.Int64
	nD.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "E2EE:short" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = nA.Run(ctx) }()
	go func() { _ = nB.Run(ctx) }()
	go func() { _ = nC.Run(ctx) }()
	go func() { _ = nD.Run(ctx) }()

	_, err := nA.Send(time.Now(), []byte("E2EE:short"), 6, transport_selector.PreferenceAny)
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected delivery from BLE cluster A to BLE cluster B via LoRa bridge")
	}
}

func TestLoRa_ScenarioB_StoreCarryForwardViaMobileNodes(t *testing.T) {
	bleNet := bridge_node_logic.NewSimNet()

	mA, _ := identity_manager.New(200 * time.Millisecond)
	mCourier, _ := identity_manager.New(200 * time.Millisecond)
	mB, _ := identity_manager.New(200 * time.Millisecond)

	now := time.Now()
	idA := mA.Current(now).NodeID
	idX := mCourier.Current(now).NodeID
	idB := mB.Current(now).NodeID

	bleA := bleNet.Register(bridge_node_logic.MediumBLE, idA, 0)
	bleX := bleNet.Register(bridge_node_logic.MediumBLE, idX, 0)
	bleB := bleNet.Register(bridge_node_logic.MediumBLE, idB, 0)
	bleA.SetAvailable(true)
	bleX.SetAvailable(true)
	bleB.SetAvailable(true)

	nA, _ := multipath_router.NewNode(mA, []transport_manager.Transport{bleA}, multipath_router.Options{
		Profile: transport_selector.ProfileStealth,
		Router:  loraRouterOpts(),
	})
	nX, _ := multipath_router.NewNode(mCourier, []transport_manager.Transport{bleX}, multipath_router.Options{
		Profile: transport_selector.ProfileStealth,
		Router:  loraRouterOpts(),
	})
	nB, _ := multipath_router.NewNode(mB, []transport_manager.Transport{bleB}, multipath_router.Options{
		Profile: transport_selector.ProfileStealth,
		Router:  loraRouterOpts(),
	})

	var got atomic.Int64
	nB.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "E2EE:courier" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = nA.Run(ctx) }()
	go func() { _ = nX.Run(ctx) }()
	go func() { _ = nB.Run(ctx) }()

	bleNet.Connect(bridge_node_logic.MediumBLE, idA, idX)
	time.AfterFunc(450*time.Millisecond, func() {
		bleNet.Disconnect(bridge_node_logic.MediumBLE, idA, idX)
		bleNet.Connect(bridge_node_logic.MediumBLE, idX, idB)
	})

	time.Sleep(120 * time.Millisecond)
	_, err := nA.Send(time.Now(), []byte("E2EE:courier"), 6, transport_selector.PreferenceAny)
	if err != nil {
		t.Fatal(err)
	}

	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected store-carry-forward delivery via mobile courier node")
	}
}

func TestLoRa_ScenarioC_BridgeNodeExtendsAcrossBlackoutZone(t *testing.T) {
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	mLocal, _ := identity_manager.New(200 * time.Millisecond)
	mBridge, _ := identity_manager.New(200 * time.Millisecond)
	mRemote, _ := identity_manager.New(200 * time.Millisecond)

	now := time.Now()
	idL := mLocal.Current(now).NodeID
	idX := mBridge.Current(now).NodeID
	idR := mRemote.Current(now).NodeID

	wifiL := wifiNet.Register(bridge_node_logic.MediumWiFi, idL, 0)
	wifiX := wifiNet.Register(bridge_node_logic.MediumWiFi, idX, 0)
	wifiL.SetAvailable(true)
	wifiX.SetAvailable(true)
	wifiNet.Connect(bridge_node_logic.MediumWiFi, idL, idX)

	loraX := loraNet.Register(idX, 0)
	loraR := loraNet.Register(idR, 0)
	loraX.Configure(96, 35)
	loraR.Configure(96, 35)
	loraX.SetAvailable(true)
	loraR.SetAvailable(true)
	loraNet.Connect(idX, idR)

	nL, _ := multipath_router.NewNode(mLocal, []transport_manager.Transport{wifiL}, multipath_router.Options{
		Profile: transport_selector.ProfilePerformance,
		Router:  loraRouterOpts(),
	})
	nX, _ := multipath_router.NewNode(mBridge, []transport_manager.Transport{wifiX, loraX}, multipath_router.Options{
		Profile:   transport_selector.ProfileBalanced,
		Multipath: true,
		Router:    loraRouterOpts(),
	})
	nR, _ := multipath_router.NewNode(mRemote, []transport_manager.Transport{loraR}, multipath_router.Options{
		Profile: transport_selector.ProfileBalanced,
		Router:  loraRouterOpts(),
	})

	var got atomic.Int64
	nR.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "E2EE:blackout" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = nL.Run(ctx) }()
	go func() { _ = nX.Run(ctx) }()
	go func() { _ = nR.Run(ctx) }()

	_, _ = nL.Send(time.Now(), []byte("E2EE:blackout"), 6, transport_selector.PreferenceAny)
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected bridge node to relay between Wi-Fi and LoRa")
	}
}

func TestLoRa_ScenarioD_LowBandwidthEncryptedTextOverLoRa(t *testing.T) {
	loraNet := lora_transport.NewSimNet()

	m0, _ := identity_manager.New(200 * time.Millisecond)
	m1, _ := identity_manager.New(200 * time.Millisecond)
	now := time.Now()
	id0 := m0.Current(now).NodeID
	id1 := m1.Current(now).NodeID

	l0 := loraNet.Register(id0, 0)
	l1 := loraNet.Register(id1, 0)
	l0.Configure(96, 18)
	l1.Configure(96, 18)
	l0.SetAvailable(true)
	l1.SetAvailable(true)
	loraNet.Connect(id0, id1)

	n0, _ := multipath_router.NewNode(m0, []transport_manager.Transport{l0}, multipath_router.Options{
		Profile: transport_selector.ProfileBalanced,
		Router:  loraRouterOpts(),
	})
	n1, _ := multipath_router.NewNode(m1, []transport_manager.Transport{l1}, multipath_router.Options{
		Profile: transport_selector.ProfileBalanced,
		Router:  loraRouterOpts(),
	})

	var got atomic.Int64
	n1.Router.SetOnMessage(func(m message_router.Message) {
		if string(m.Payload) == "E2EE:hello" {
			got.Add(1)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { _ = n0.Run(ctx) }()
	go func() { _ = n1.Run(ctx) }()

	_, _ = n0.Send(time.Now(), []byte("E2EE:hello"), 8, transport_selector.PreferencePreferLoRa)
	<-ctx.Done()
	if got.Load() == 0 {
		t.Fatalf("expected delivery over low-bandwidth LoRa")
	}
}
