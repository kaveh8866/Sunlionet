package ble_scanner

import (
	"context"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type Observation struct {
	NodeID identity_manager.NodeID
	RSSI   int
}

type Scanner interface {
	Start(ctx context.Context) (<-chan Observation, error)
}
