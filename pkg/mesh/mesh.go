package mesh

import (
	"context"

	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type Mesh interface {
	Broadcast(data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	RuntimeMode() runtimecfg.RuntimeMode
}
