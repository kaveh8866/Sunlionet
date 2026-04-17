package mesh

import (
	"context"

	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

type Mesh interface {
	Broadcast(data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	RuntimeMode() runtimecfg.RuntimeMode
}
