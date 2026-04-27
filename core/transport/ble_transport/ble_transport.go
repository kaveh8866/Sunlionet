package ble_transport

import (
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/bridge_node_logic"
)

type Transport = bridge_node_logic.Endpoint

func NewSim(net *bridge_node_logic.SimNet, node identity_manager.NodeID) *Transport {
	ep := net.Register(bridge_node_logic.MediumBLE, node, 0)
	ep.SetAvailable(true)
	return ep
}

func LinkScore(_ identity_manager.NodeID, _ time.Time) float64 { return 0.5 }
