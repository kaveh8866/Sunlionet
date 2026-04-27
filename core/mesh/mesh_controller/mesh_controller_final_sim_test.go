package mesh_controller

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/core/mesh/security"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/ble_transport"
	"github.com/kaveh/sunlionet-agent/core/transport/bridge_node_logic"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
	"github.com/kaveh/sunlionet-agent/core/transport/wifi_direct_transport"
	"github.com/kaveh/sunlionet-agent/experimental/lora/lora_transport"
)

func routerDefaults() message_router.Options {
	return message_router.Options{
		MaxFrameBytes:       220,
		MaxHop:              8,
		MinRebroadcastEvery: 30 * time.Millisecond,
		ClockSkewAllowance:  2 * time.Minute,
		MaxPayloadBytes:     8 * 1024,

		ReplayBucketDuration: 2 * time.Second,
		ReplayWindowDuration: 20 * time.Second,

		GlobalRateCapacity: 90,
		GlobalRate:         45,
		PerSenderCapacity:  18,
		PerSenderRate:      7,

		GossipSeed:           1,
		RoutingBaseProb:      0.98,
		RoutingMaxJitter:     0,
		RoutingMinTTLRemain:  0,
		InventoryEvery:       60 * time.Millisecond,
		InventoryMax:         18,
		WantMax:              18,
		CoverTrafficEvery:    0,
		CoverTrafficSize:     24,
		ReassemblyStaleAfter: 3 * time.Second,
		CacheMaxItems:        2048,
	}
}

type node struct {
	idm  *identity_manager.Manager
	id   identity_manager.NodeID
	ctrl *Controller

	ble  *bridge_node_logic.Endpoint
	wifi *bridge_node_logic.Endpoint
	lora *lora_transport.Endpoint

	got chan []byte
}

func newNode(t *testing.T, now time.Time, bleNet *bridge_node_logic.SimNet, wifiNet *bridge_node_logic.SimNet, loraNet *lora_transport.SimNet, enableBLE bool, enableWiFi bool, enableLoRa bool, pol security.Policy) node {
	t.Helper()

	idm, err := identity_manager.New(24 * time.Hour)
	if err != nil {
		t.Fatalf("identity_manager.New: %v", err)
	}
	id := idm.Current(now).NodeID

	var transports []transport_manager.Transport
	var ble *bridge_node_logic.Endpoint
	var wifi *bridge_node_logic.Endpoint
	var lora *lora_transport.Endpoint

	if enableBLE {
		ble = ble_transport.NewSim(bleNet, id)
		transports = append(transports, ble)
	}
	if enableWiFi {
		wifi = wifi_direct_transport.NewSim(wifiNet, id)
		transports = append(transports, wifi)
	}
	if enableLoRa {
		lora = loraNet.Register(id, 1024)
		lora.Configure(96, 64)
		lora.SetAvailable(true)
		transports = append(transports, lora)
	}

	ctrl, err := New(idm, transports, Options{
		Policy: pol,
		TTL:    8,
		Router: routerDefaults(),
	})
	if err != nil {
		t.Fatalf("controller.New: %v", err)
	}
	got := make(chan []byte, 64)
	ctrl.SetOnMessage(func(m message_router.Message) {
		cp := append([]byte(nil), m.Payload...)
		select {
		case got <- cp:
		default:
		}
	})
	return node{idm: idm, id: id, ctrl: ctrl, ble: ble, wifi: wifi, lora: lora, got: got}
}

func runNodes(ctx context.Context, ns []node) {
	for i := range ns {
		n := ns[i]
		go func() { _ = n.ctrl.Run(ctx) }()
	}
}

func waitFor(t *testing.T, ctx context.Context, ch <-chan []byte, want []byte) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %q", string(want))
		case b := <-ch:
			if string(b) == string(want) {
				return
			}
		}
	}
}

func TestFinalScenarioA_ShutdownBluetoothOnlyMesh(t *testing.T) {
	now := time.Now()
	bleNet := bridge_node_logic.NewSimNet()
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	pol := security.Policy{Mode: security.ModeStealth}
	ns := make([]node, 0, 5)
	for i := 0; i < 5; i++ {
		ns = append(ns, newNode(t, now, bleNet, wifiNet, loraNet, true, false, false, pol))
	}
	for i := 0; i < len(ns)-1; i++ {
		bleNet.Connect(bridge_node_logic.MediumBLE, ns[i].id, ns[i+1].id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runNodes(ctx, ns)

	payload := []byte("A:bluetooth-only")
	if _, err := ns[0].ctrl.Send(time.Now(), payload, 10, 0); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, ctx, ns[len(ns)-1].got, payload)
}

func TestFinalScenarioB_PartitionHealsViaBridgeNodes(t *testing.T) {
	now := time.Now()
	bleNet := bridge_node_logic.NewSimNet()
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	pol := security.Policy{Mode: security.ModeResilience}
	ns := make([]node, 0, 6)
	for i := 0; i < 6; i++ {
		enableLoRa := i == 2 || i == 3
		ns = append(ns, newNode(t, now, bleNet, wifiNet, loraNet, true, false, enableLoRa, pol))
	}

	bleNet.Connect(bridge_node_logic.MediumBLE, ns[0].id, ns[1].id)
	bleNet.Connect(bridge_node_logic.MediumBLE, ns[1].id, ns[2].id)
	bleNet.Connect(bridge_node_logic.MediumBLE, ns[3].id, ns[4].id)
	bleNet.Connect(bridge_node_logic.MediumBLE, ns[4].id, ns[5].id)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	runNodes(ctx, ns)

	payload := []byte("B:partition-heal")
	if _, err := ns[0].ctrl.Send(time.Now(), payload, 12, 0); err != nil {
		t.Fatalf("send: %v", err)
	}

	time.Sleep(250 * time.Millisecond)
	loraNet.Connect(ns[2].id, ns[3].id)

	waitFor(t, ctx, ns[5].got, payload)
}

func TestFinalScenarioC_MaliciousFloodingDegradesGracefully(t *testing.T) {
	now := time.Now()
	bleNet := bridge_node_logic.NewSimNet()
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	pol := security.Policy{Mode: security.ModeBalanced}
	ns := make([]node, 0, 10)
	for i := 0; i < 10; i++ {
		ns = append(ns, newNode(t, now, bleNet, wifiNet, loraNet, true, false, false, pol))
	}
	for i := 0; i < len(ns); i++ {
		for j := i + 1; j < len(ns); j++ {
			bleNet.Connect(bridge_node_logic.MediumBLE, ns[i].id, ns[j].id)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	runNodes(ctx, ns)

	for i := 1; i <= 4; i++ {
		spammer := ns[i]
		go func() {
			raw := []byte{0x00, 0x01, 0x02, 0x03}
			for k := 0; k < 300; k++ {
				_ = spammer.ble.Broadcast(raw)
			}
		}()
		go func() {
			for k := 0; k < 120; k++ {
				_, _ = spammer.ctrl.Send(time.Now(), []byte("spam"), 6, 0)
			}
		}()
	}

	payload := []byte("C:legit")
	if _, err := ns[0].ctrl.Send(time.Now(), payload, 10, 0); err != nil {
		t.Fatalf("send legit: %v", err)
	}
	waitFor(t, ctx, ns[9].got, payload)
}

func TestFinalScenarioD_TransportFailuresTriggerAutomaticFailover(t *testing.T) {
	now := time.Now()
	bleNet := bridge_node_logic.NewSimNet()
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	pol := security.Policy{Mode: security.ModeBalanced}
	a := newNode(t, now, bleNet, wifiNet, loraNet, true, true, false, pol)
	b := newNode(t, now, bleNet, wifiNet, loraNet, true, true, false, pol)

	bleNet.Connect(bridge_node_logic.MediumBLE, a.id, b.id)
	wifiNet.Connect(bridge_node_logic.MediumWiFi, a.id, b.id)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runNodes(ctx, []node{a, b})

	p1 := append([]byte("D:wifi-first:"), bytes.Repeat([]byte{0xAB}, 1024)...)
	if _, err := a.ctrl.Send(time.Now(), p1, 10, transport_selector.PreferencePreferWiFi); err != nil {
		t.Fatalf("send p1: %v", err)
	}
	waitFor(t, ctx, b.got, p1)
	if a.wifi.TxCount() == 0 {
		t.Fatalf("expected wifi traffic for first send")
	}

	a.wifi.SetAvailable(false)
	b.wifi.SetAvailable(false)

	p2 := append([]byte("D:ble-fallback:"), bytes.Repeat([]byte{0xCD}, 1024)...)
	if _, err := a.ctrl.Send(time.Now(), p2, 10, transport_selector.PreferencePreferWiFi); err != nil {
		t.Fatalf("send p2: %v", err)
	}
	waitFor(t, ctx, b.got, p2)
	if a.ble.TxCount() == 0 {
		t.Fatalf("expected ble traffic after wifi disabled")
	}
}

func TestFinalScenarioE_MixedBLEWiFiLoRaFallbackSurvival(t *testing.T) {
	now := time.Now()
	bleNet := bridge_node_logic.NewSimNet()
	wifiNet := bridge_node_logic.NewSimNet()
	loraNet := lora_transport.NewSimNet()

	pol := security.Policy{Mode: security.ModeResilience, CoverTraffic: true}
	n0 := newNode(t, now, bleNet, wifiNet, loraNet, true, true, true, pol)
	n1 := newNode(t, now, bleNet, wifiNet, loraNet, true, false, true, pol)
	n2 := newNode(t, now, bleNet, wifiNet, loraNet, true, false, true, pol)
	n3 := newNode(t, now, bleNet, wifiNet, loraNet, true, true, true, pol)

	bleNet.Connect(bridge_node_logic.MediumBLE, n0.id, n1.id)
	bleNet.Connect(bridge_node_logic.MediumBLE, n2.id, n3.id)
	loraNet.Connect(n1.id, n2.id)

	n0.wifi.SetAvailable(false)
	n3.wifi.SetAvailable(false)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	runNodes(ctx, []node{n0, n1, n2, n3})

	payload := []byte("E:mixed-fallback")
	if _, err := n0.ctrl.Send(time.Now(), payload, 14, 0); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, ctx, n3.got, payload)
	if n1.lora.TxCount() == 0 && n2.lora.TxCount() == 0 {
		t.Fatalf("expected some lora traffic in mixed fallback")
	}
}
