package ble_advertiser

import (
	"context"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type Advertiser interface {
	Start(ctx context.Context, id identity_manager.Identity) error
}
